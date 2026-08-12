# Cafe — Contoh Aplikasi FormSpec

Aplikasi referensi **kafe** yang dibangun lewat alur **agent-assisted
app development** (tanpa MCP) — lihat
[`docs/guides/agent-assisted-app-development.md`](../../docs/guides/agent-assisted-app-development.md).

Proyek ini di-scaffold dengan `formspec init` (memuat `.agents/skills/`,
`schemas/`, `.github/copilot-instructions.md`) lalu diisi spec + docs
mengikuti workflow 4 fase: **Discovery → Proposal → Draft → Iterate**.

## Struktur

```
cafe/
  formspec-app.yaml               # Config CLI (bukan kind: Config)
  .agents/skills/              # 4 AI skills untuk Copilot
  .github/copilot-instructions.md
  schemas/ + .vscode/settings.json
  spec/
    apps/cafe.yaml             # kind: App
    modules/
      cafe-master/             # master: menu-item, table, member, employee
      cafe-order/              # transaction: order (child items), payment
      cafe-report/             # dashboard + widgets + report
  docs/
    overview.md                # Discovery output
    architecture.md            # Proposal output
    domain-model.md            # Proposal output (ER + state machines)
```

## Modul & Entity

| Modul | Entity | Karakteristik |
|---|---|---|
| `cafe-master` | menu-item, table, member, employee | `master` |
| `cafe-order` | order, payment | `transaction` |
| `cafe-report` | dashboard, 4 widget, report | UI kinds |

## Validasi

```bash
formspec validate --spec spec
# → 16 manifest(s) validated, 0 problem(s) found
```
