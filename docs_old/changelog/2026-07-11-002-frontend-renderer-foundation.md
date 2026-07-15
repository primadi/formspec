# Frontend Renderer — Fase 4.F1 Foundation

**Date:** 2026-07-11  
**Plan:** `docs/plan/todo.md` Fase 4.F1  
**Design:** `docs/implementation/frontend-renderer.md`  

## What

Membangun fondasi manifest-driven renderer SPA Forma: TypeScript types, API client, zustand stores, FormaExpr interpreter, dan permission gate.

## Files Created (16 files)

### Phase A — TypeScript Types
- `web/src/types/manifest.ts` — Mirror lengkap Go structs dari `pkg/spec/frontend.go`, `pkg/spec/entity.go`, `pkg/spec/spec.go`, `internal/ui/meta.go`. Mencakup 12 UI kind specs, entity/document spec, Meta API payloads (EntitySchema, MetaBundle, MeResponse), API response envelopes, ListParams, constants.
- `web/src/types/index.ts` — Barrel re-export

### Phase B — API Client
- `web/src/lib/api/client.ts` — ky instance factory dengan auth header, response interceptor (ErrorResponse → typed FormaApiError), CAS version header, list parameter builder (`buildListParams`). Helper: `apiGet`, `apiList`, `apiPost`, `apiPut`, `apiPatch`, `apiDelete`.
- `web/src/lib/api/meta.ts` — `fetchMetaBundle()`, `fetchEntitySchema()`, `fetchMe()` untuk Meta API endpoints.
- `web/src/lib/api/index.ts` — Barrel re-export

### Phase C — Zustand Stores
- `web/src/stores/session.ts` — Session store (workspace, token, me, `can()` permission check, `boot()` lifecycle)
- `web/src/stores/meta.ts` — Meta bundle store dengan lazy-computed lookup maps (`getEntity`, `getForm`, `getTable`, dll.)
- `web/src/stores/prefs.ts` — Preferences store (sidebarCollapsed, theme) persisted ke localStorage
- `web/src/stores/index.ts` — Barrel re-export

### Phase D — FormaExpr Interpreter
- `web/src/lib/formaexpr/lexer.ts` — Tokenizer: identifiers, numbers, strings (escaped), keywords (and/or/not/in/len/sum), operators, delimiters
- `web/src/lib/formaexpr/parser.ts` — Pratt parser: precedence climbing, 7 precedence levels, AST nodes (Binary/Unary/Member/Call/List), error reporting
- `web/src/lib/formaexpr/eval.ts` — Tree-walking evaluator: arithmetic, comparison, logic, `len()`, `sum()`, member access, list literals, graceful null/undefined handling
- `web/src/lib/formaexpr/index.ts` — Unified API: `evalFormaExpr()`, `validateFormaExpr()`, convenience wrappers (`evalVisibleWhen`, `evalReadonlyWhen`, `evalRequiredWhen`, `evalCompute`)
- `web/src/lib/formaexpr/formaexpr.test.ts` — 84 table-driven test cases (literals, arithmetic, comparisons, logic, field access, `len`/`sum`, lists, error handling, edge cases, convenience wrappers)

### Phase E — Permission Gate
- `web/src/engine/permissions.ts` — Port Go `Identity.HasPermission`: exact match, wildcard (`module.entity.*`), super-wildcard (`*`), `public`, `isValidPermissionFormat()`, `qualifyPerm()`.
- `web/src/engine/index.ts` — Barrel re-export

## Verification

- `npx tsc -b --noEmit` — ✅ Zero TypeScript errors
- `npx vitest run` — ✅ 84/84 tests passing

## Key Decisions

- **Types mirror, not codegen**: Ditulis manual. Codegen resmi (Fase 6.4) akan mengganti nanti.
- **FormaExpr tanpa dependency**: Lexer + Pratt parser + tree-walking evaluator murni TS, tanpa `eval()`, ~450 LOC.
- **Session di memory**: Reload page → re-fetch dari Meta API. Hanya `prefs` persist ke localStorage.
- **Graceful undefined**: FormaExpr mengembalikan `null` untuk field/identifier yang tidak dikenal (tidak throw/crash).
