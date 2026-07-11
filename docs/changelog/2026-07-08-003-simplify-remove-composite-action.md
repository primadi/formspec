# 2026-07-08 — Simplifikasi: Hapus Composite Action Konsep Terpisah

**Diskusi:** Use case composite action ternyata sedikit. Cukup inline Starlark `ctx.call_action()`.
**File utama:** `docs/spec/02-core-basic.md`, `docs/spec/05-frontend.md`

## Perubahan

### 1. Hapus `composite` dan `composite_calls` dari Action Spec

Sebelumnya ada dua YAML field khusus (`composite: true`, `composite_calls: []`) ditambah section §4.1c dan §11.6. Semua dihapus. Untuk multi-step action kustom, cukup inline Starlark:

```python
def handle(params, ctx):
    order = ctx.call_action("order", "create", params)
    ctx.call_action("order", "submit", {"id": order.id})
    return order
```

### 2. §4.1c → "Multi-Step Actions via Inline Script"

Bukan mekanisme baru — penjelasan bahwa `ctx.call_action()` berurutan dalam handler Starlark sudah cukup. Framework handle transaksi secara implisit via `ctx.db`.

### 3. §11.6 (Composite actions) → Dihapus

Tidak ada lagi sub-section khusus untuk composite.

### 4. `create-submit` tetap sebagai 7th reserved action

Auto-derived, tanpa perlu deklarasi. Framework handle atomically.

### 5. §14d (Saga) → Detach dari konsep composite

Sekarang framed sebagai: "Ketika `ctx.call_action()` cross-boundary." Tidak terkait dengan composite action.

### 6. Frontend §1.7 → Update references

Hapus `composite: true` + `composite_calls` dari contoh YAML. Ganti `create-and-submit` dengan `create-submit` (built-in).

## File yang Diubah

| File | Perubahan |
|---|---|
| `docs/spec/02-core-basic.md` | Hapus `composite`/`composite_calls` dari YAML example. Hapus §11.6. Simplifikasi §4.1c. Update §14d. Update scope + header. |
| `docs/spec/05-frontend.md` | Hapus `composite: true`/`composite_calls` dari YAML. Update referensi. |
