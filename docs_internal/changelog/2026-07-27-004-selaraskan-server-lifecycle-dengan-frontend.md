# Fix: Selaraskan Server `lifecycle()` dengan Frontend

## Perubahan

### `internal/ui/meta.go` — `lifecycle()`
**Sebelum:**
```go
if a.Name == "submit" && a.Disabled {
    return "plain_crud"
}
return "two_step_autosave"  // ← default selalu autosave
```

**Sesudah:**
```go
if a.Name == "submit" {
    if a.Disabled { return "plain_crud" }
    return "two_step_autosave"  // ← hanya jika submit aktif
}
return "plain_crud"  // ← default
```

Server sekarang cuma return `two_step_autosave` kalau ada action `submit` yang tidak di-`disabled`. Entity tanpa `submit` action (seperti Visit) dapet `plain_crud` → 2 tombol (Save + Cancel), tanpa auto-save.

### `internal/ui/ui_test.go`
Update test `TestEntitySchemaDerivation` — ekspektasi lifecycle berubah dari `two_step_autosave` ke `plain_crud` karena entity test (`orderEntity`) tidak punya action `submit`.

## File Terkena Dampak
- `internal/ui/meta.go`
- `internal/ui/ui_test.go`
