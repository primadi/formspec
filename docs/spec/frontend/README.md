# Spec Frontend — Kontrak Visual

Kontrak yang wajib dipenuhi **Shell** dan renderer visual manapun. Kind visual
tidak flat — ada hirarki empat tingkat:

```
Shell  →  App renderer  →  Page renderer  →  Component renderer
```

Shell adalah stack teknologi + kontrak bootstrap penuh (satu App = satu Shell).
Tiga tingkat di dalamnya dideklarasikan lewat meta-kind **VisualSpecKind**
(dengan `tier: app | page | component`) dan diimplementasikan oleh kind
**Renderer**. Shell membaca spec saat **runtime** lewat **Spec Resolution API**
— bukan code generation.

| Dokumen | Cakupan |
|---|---|
| [01-visual-hierarchy.md](01-visual-hierarchy.md) | Empat tingkat hirarki, aturan `stack_family`, kebijakan shell baru |
| [02-visual-spec-kind.md](02-visual-spec-kind.md) | Meta-kind VisualSpecKind: tier, schema, renderer_contract, slot system |
| [03-renderer-kind.md](03-renderer-kind.md) | Kind Renderer: implements, stack_family, trust_tier, registry, konformansi |
| [04-spec-resolution-api.md](04-spec-resolution-api.md) | Seam runtime shell ↔ engine; wajib backend-agnostic |
| [05-app-kinds.md](05-app-kinds.md) | Katalog tier app: sidebar-nav, topnav, landing-page |
| [06-page-kinds.md](06-page-kinds.md) | Katalog tier page: data-entry, table, wizard, kanban, dashboard, report, … |
| [07-component-kinds.md](07-component-kinds.md) | Katalog tier component: input, widget, slot filling, asset |
| [08-formaexpr.md](08-formaexpr.md) | Grammar ekspresi client-side, berlaku untuk semua shell |

Renderer/shell author: baca 01 → 02 → 03 → 04, lalu katalog tier yang relevan.
