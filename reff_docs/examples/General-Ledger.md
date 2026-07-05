# Forma Example: General Ledger Module

**Status:** Draft — contoh modul vertikal (Frappe-style) dengan
`characteristics: [reference]`, `[summary]`, double-entry validation,
dan cross-module `kind: Subscription`.
**Spec target:** Forma Core Basic v0.2.0.

---

## 1. Kebutuhan Bisnis: General Ledger

**Alur:** setiap transaksi keuangan (pembayaran order, pengeluaran, dll.)
otomatis menghasilkan journal entry double-entry → diposting ke
general ledger balance.

| # | Requirement | Kenapa sulit tanpa konvensi |
|---|---|---|
| FR1 | Chart of accounts (AKUN-1-10001, dll.) seeded, tidak bisa diedit user | Reference data — milik App Owner |
| FR2 | Journal entry selalu double-entry: debit == credit | Validasi cross-row, bukan per-field |
| FR3 | Nomor jurnal urut: `JRN-{year}-{seq}` | Sequence dengan lock |
| FR4 | Status: draft → posted → reversed | State machine dengan guard |
| FR5 | GL Balance otomatis ter-update saat posting | Summary entity via reliable event |
| FR6 | Saat order dibayar → auto-create journal entry | Subscription ke `billing.order.paid` |
| FR7 | Multi-currency via config | Config per-workspace |

---

## 2. Struktur project

```
general-ledger/
  module.yaml
  entities/
    account.yaml            # kind: Entity, characteristics: [reference]
    journal-entry.yaml      # kind: Entity, characteristics: [transaction]
    gl-balance.yaml         # kind: Entity, characteristics: [summary]
  subscriptions/
    order-to-journal.yaml   # kind: Subscription → billing.order.paid
  scripts/
    journal_post.star
    gl_balance_update.star
  config/
    gl.yaml                 # kind: Config (default_currency, fiscal_year_start)
```

---

## 3. Module manifest

```yaml
apiVersion: forma.dev/v1alpha1
kind: Module
metadata:
  name: general-ledger
  description: Double-entry accounting core
spec:
  version: 1.0.0
  vendor: forma-dev
  depends:
    - module: forma/core
  config:
    default_currency: IDR
    fiscal_year_start: "01-01"
```

Permission namespace: `general-ledger.*` (misal `general-ledger.journal-entries.post`).

---

## 4. Entity `account` — Chart of Accounts (reference)

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: account
  module: general-ledger
  description: Chart of accounts — satu bagan akun standar per workspace
spec:
  version: v1
  characteristics: [reference]    # seeded, read-only bagi Data Owner

  fields:
    - name: code
      type: string
      natural_key: true
      immutable: true
      unique: true
      index: true
      description: Kode akun — "1-10001" (Kas), "4-40001" (Pendapatan)
    - name: name
      type: string
      rules: [required]
      description: Nama akun — "Kas", "Pendapatan Penjualan"
    - name: type
      type: enum
      enum_values: [asset, liability, equity, revenue, expense]
      index: true
    - name: normal_balance
      type: enum
      enum_values: [debit, credit]
      description: Saldo normal — asset=debit, revenue=credit, dll.
    - name: is_active
      type: boolean
      default: true
```

Chart of accounts di-seed per-tenant oleh App Owner via rilis.
Data Owner hanya bisa membaca; tidak bisa create/update/delete.

**Contoh seed data (via `forma/seed`):**
| code | name | type | normal_balance |
|---|---|---|---|
| 1-10001 | Kas | asset | debit |
| 1-11001 | Piutang Usaha | asset | debit |
| 4-40001 | Pendapatan Penjualan | revenue | credit |
| 5-50001 | Harga Pokok Penjualan | expense | debit |

---

## 5. Entity `journal-entry` — Transaction

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: journal-entry
  module: general-ledger
  description: Satu journal entry — selalu double-entry (min 2 baris)
spec:
  version: v1
  characteristics: [transaction]

  fields:
    - name: number
      type: string
      natural_key: true
      immutable: true
      unique: true
      index: true
      natural_key_rule:
        strategy: sequence
        format: "JRN-{year}-{seq:06d}"
        prefix: { value: "JRN" }
        reset: yearly
    - name: entry_date
      type: date
      rules: [required]
    - name: reference
      type: string
      description: Referensi dokumen sumber — "ORD-2026-000123"
    - name: source
      type: string
      immutable: true
      index: true
      description: Resource asal — "billing.order"
    - name: source_id
      type: uuid
      immutable: true
      index: true
    - name: description
      type: string
      rules: [required]
    - name: currency
      type: string
      default: "IDR"
    - name: status
      type: enum
      enum_values: [draft, posted, reversed]
      index: true
    - name: lines
      type: child
      child:
        storage: table               # table — perlu query langsung
        sequence_field: line_number
        fields:
          - { name: line_number, type: integer, immutable: true }
          - { name: account_id, type: uuid, rules: [required, {exists: account}] }
          - { name: description, type: string }
          - { name: debit, type: decimal, rules: [min: 0] }
          - { name: credit, type: decimal, rules: [min: 0] }

  state_machine:
    field: status
    initial: draft
    transitions:
      - { from: draft,    to: posted,   via: post,
          guard: "sum_line('debit') == sum_line('credit') and sum_line('debit') > 0" }
      - { from: posted,   to: reversed, via: reverse }
      # reversed = final; tidak bisa kembali ke posted

  actions:
    - name: post
      description: Posting journal entry — update GL balance
      required_permission: journal-entries.post
      idempotent: true
      idempotency_key: { from: server }
      audit: true
      emits: journal-posted
      uses:
        primitives: [lock]
      impl: { type: script_ref, ref: general-ledger/journal_post }

    - name: reverse
      description: Reverse (batalkan) journal entry yang sudah posted
      required_permission: journal-entries.reverse
      audit: true
      emits: journal-reversed
      conditions:
        - script: "resource.status == 'posted'"
          message: "Hanya journal posted yang bisa di-reverse"
      impl: { type: script_ref, ref: general-ledger/journal_reverse }

    - name: create
      # Override standard create untuk validasi double-entry
      idempotent: true
      idempotency_key: { from: server }
      conditions:
        - script: "len(resource.lines) >= 2"
          message: "Journal entry harus punya minimal 2 baris (debit + credit)"

  events:
    - name: journal-posted
      description: Journal entry berhasil diposting
      publish:
        durable: true
      payload:
        fields: [id, number, entry_date, source, source_id, currency]
      deliver:
        - channel: reliable_event
          target: { resource: general-ledger.gl-balance, action: update }
          retry: { max: 5, backoff: exponential }
          idempotency_key: "balance.{id}"

    - name: journal-reversed
      description: Journal entry di-reverse
      publish:
        durable: true
      payload:
        fields: [id, number, entry_date]
      deliver:
        - channel: reliable_event
          target: { resource: general-ledger.gl-balance, action: reverse }
          retry: { max: 5, backoff: exponential }
          idempotency_key: "balance-reverse.{id}"
```

---

## 6. Entity `gl-balance` — Summary

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: gl-balance
  module: general-ledger
  description: Saldo per akun per periode — di-update otomatis via reliable event
spec:
  version: v1
  characteristics: [summary]    # no create/update/delete via API

  fields:
    - name: account_id
      type: relation
      relation: { type: belongs_to, resource: account }
      immutable: true
    - name: period
      type: string           # "2026-07"
      immutable: true
      index: true
    - name: currency
      type: string
      immutable: true
    - name: opening_balance
      type: decimal
    - name: debit_movement
      type: decimal
      default: 0
    - name: credit_movement
      type: decimal
      default: 0
    - name: closing_balance
      type: decimal            # computed: opening + debit - credit (untuk asset)
                               # atau: opening + credit - debit (untuk liability/revenue)

  actions:
    - name: update
      description: Dipanggil outbox worker saat journal-posted/reversed
      idempotent: true
      impl: { type: script_ref, ref: general-ledger/gl_balance_update }
```

---

## 7. Script Handler

### 7.1 `journal_post.star`

```python
# modules/general-ledger/scripts/journal_post.star

def execute(resource, params, ctx):
    # Guard state machine sudah memvalidasi debit == credit sebelum jalan.
    # Transition dari draft → posted + tulis outbox journal-posted
    # dalam satu transaksi DB.
    resource.set("status", "posted")
    resource.save()
    ctx.log.info("journal.posted", {
        "journal_id": resource.id,
        "number": resource.field.number,
    })
    return ok()
```

### 7.2 `gl_balance_update.star`

```python
# modules/general-ledger/scripts/gl_balance_update.star

def execute(resource, params, ctx):
    # Dipanggil oleh outbox worker via reliable_event.
    # Idempotent: kalau sudah ada balance untuk (account, period, currency),
    # update; kalau belum, create.

    journal = journal_entry.load(params.id)
    period = str(journal.field.entry_date)[:7]  # "2026-07"

    for line in journal.field.lines:
        account = line.account_id
        existing = gl_balance.query() \
            .where("account_id", account) \
            .where("period", period) \
            .where("currency", journal.field.currency) \
            .first()

        if existing:
            existing.set("debit_movement",
                existing.field.debit_movement + line.debit)
            existing.set("credit_movement",
                existing.field.credit_movement + line.credit)
            existing.save()
        else:
            gl_balance.new() \
                .set("account_id", account) \
                .set("period", period) \
                .set("currency", journal.field.currency) \
                .set("debit_movement", line.debit) \
                .set("credit_movement", line.credit) \
                .save()

    ctx.log.info("gl_balance.updated", {
        "journal_id": journal.id,
        "period": period,
    })
    return ok()
```

---

## 8. Subscription — reaksi ke order.paid

```yaml
# modules/general-ledger/subscriptions/order-to-journal.yaml
apiVersion: forma.dev/v1alpha1
kind: Subscription
metadata:
  name: order-to-journal
  module: general-ledger
  description: Auto-create journal entry setiap order dibayar
spec:
  on: { resource: billing.order, event: paid }
  deliver:
    - channel: queue
      job: create-sales-journal
      # Job handler (native Go) menerima PaidEvent payload,
      # membuat journal-entry baru dengan baris:
      #   Debit:  Piutang Usaha (1-11001)  = order.total
      #   Credit: Pendapatan Penjualan (4-40001) = order.total
```

---

## 9. Pemetaan ke Primitif

| Primitif | Dipakai di | FR |
|---|---|---|
| `ctx.lock` | `ctx.next_key` untuk nomor jurnal | FR3 |
| `ctx.db` | GL balance update (write [general-ledger]) | FR5 |
| `ctx.config` | default_currency | FR7 |

---

## 10. Yang di-cover oleh contoh ini

| Konsep | Dimana |
|---|---|
| `characteristics: [reference]` — data seeded | account entity |
| `characteristics: [summary]` — system-managed | gl-balance entity |
| `characteristics: [transaction]` — append-heavy | journal-entry entity |
| Double-entry validation via guard | journal-entry.post transition |
| `child: { storage: table }` | journal-entry.lines |
| `natural_key_rule: sequence` | journal-entry.number |
| State machine: draft → posted → reversed | journal-entry state_machine |
| Reliable event ke summary entity | journal-posted → gl-balance.update |
| `kind: Subscription` cross-module | order-to-journal subscription |
| Module config defaults | general-ledger module.yaml |
| Idempotency di create + post | journal-entry actions |
