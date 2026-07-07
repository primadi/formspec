# Entity Extension

**Status:** Draft
**Audience:** Module Vendors & App Developers
**Prerequisites:** [Core Basic Spec](./02-core-basic.md) · [Core Extended Spec](./03-core-extended.md)

> How to add custom fields to an entity owned by another module (e.g., a vertical `billing/invoice` module) — without forking the module, without breaking its upgrade path, and without sacrificing query performance.

---

## 1. Problem

You are using a vertical module from the marketplace — for example, `billing/invoice`. You need to add custom business fields (e.g., `project_code`, `cost_center`, `approval_notes`) to the invoice entity. You cannot fork the module (it receives updates and is signed by its vendor). You need the fields to be queryable, and you need to be able to uninstall your extension cleanly.

---

## 2. Design: Separate JSONB Column per Extension Namespace

Each extension adds a **new physical column** to the same table — not a separate table, not a nested path inside the base `data` column:

```sql
-- Original table (owned by billing module)
CREATE TABLE financial.billing_invoices (
  id          uuid PRIMARY KEY DEFAULT gen_uuid_v7(),
  tenant_id   uuid NOT NULL,
  version     integer NOT NULL DEFAULT 1,
  data        jsonb NOT NULL DEFAULT '{}',   -- core fields, owned by billing
  ...
);

-- After extension by my-customization module
ALTER TABLE financial.billing_invoices
  ADD COLUMN ext_kastem1 jsonb NOT NULL DEFAULT '{}';
```

The extension manifest (`spec.extend_storage` is a Core Extended-scope entity-spec attribute, and this document is its normative definition; implementations claiming only Core Basic conformance MAY ignore it):

```yaml
apiVersion: forma.dev/v1alpha1
kind: Entity
metadata:
  name: invoice-ext
  module: my-customization
spec:
  extend_storage:
    target: billing/invoice
    namespace: kastem1
  fields:
    - name: project_code
      type: string
      index: true
    - name: cost_center
      type: string
```

### Why a separate column, not a nested path?

| Approach | Uninstall | Read isolation | Risk |
|---|---|---|---|
| Nested path (`data->'ext'->'kastem1'`) | Requires `UPDATE` on every row to strip the key — expensive and risky on large tables | Every read of `data` also detoasts extension data even when not needed | ❌ |
| **Separate column (`ext_kastem1`)** | `ALTER TABLE ... DROP COLUMN` — metadata-only, instant | Queries against `data` never touch extension columns | ✅ |

### Why not `extendEntity` (Odoo `_inherit`)?

The Odoo pattern merges extension fields directly into the base module's field list in the same column. This:
- Violates module isolation (the base module's `data` column is modified)
- Creates high upgrade risk (vendor changes a field name → your extension breaks)
- Makes uninstall destructive (must rewrite the entire `data` column)

Forma's approach: the extension gets its **own column**, prefixed `ext_` to avoid collisions with framework-reserved columns (`data`, `tenant_id`, `version`, etc.).

---

## 3. Namespace Reservation

Namespace names (`kastem1`) once used **cannot be reused**, active or dropped, unless explicitly purged. Enforced via a registry table:

```sql
CREATE TABLE forma_extensions (
  resource    text NOT NULL,   -- billing/invoice
  namespace   text NOT NULL,   -- kastem1
  module      text NOT NULL,   -- my-customization
  status      text NOT NULL,   -- active | dropped
  created_at  timestamptz NOT NULL,
  PRIMARY KEY (resource, namespace)
);
```

`forma apply` rejects a namespace already recorded for the same resource. This prevents accidental collision when two modules independently pick the same namespace.

---

## 4. Nested Extend — Not Recommended

Extending an extension (`kastem1` → `kastem1_special1`) is technically possible but has three problems:

1. **Permanent coupling to physical column names** — hard to rename or de-orphan safely if the base extension is replaced or removed.
2. **Creates a migration ordering dependency** that did not previously exist (`kastem1` must be applied before `special1`).
3. **Leaks abstraction** — the `special1` module now knows that `kastem1` is an extension, not a regular entity.

**Recommended alternative:** all extensions remain **flat siblings** against the base entity, regardless of how many there are. If modules depend on each other's extensions, declare the dependency via `spec.depends` in the Module manifest (Core Basic §4.5), and access cross-extension fields through code — not through column naming:

```python
# In a script or handler
invoice.ext("kastem1").project_code
```

---

## 5. Indexing on Extension Fields

Fields with `index: true` in an extension still mean altering the base module's table DDL:

```sql
ALTER TABLE financial.billing_invoices
  ADD COLUMN _project_code VARCHAR
    GENERATED ALWAYS AS (ext_kastem1->>'project_code') STORED;
CREATE INDEX ON financial.billing_invoices (tenant_id, _project_code) WHERE deleted_at IS NULL;
```

This is a coupling point, but it is controlled:
- It happens at **migration time**, reviewable via `forma apply --dry-run`
- Fields without `index: true` (the default) do not touch DDL at all
- The extension module's `uses` declaration must declare `db: { write: [billing] }` for the target category, visible in the consent footprint

---

## 6. Developer Experience Context

### Two Layers of Change

Forma intentionally separates two kinds of change with different barriers to entry:

| Layer | Examples | Path | Requires git/devcontainer? |
|---|---|---|---|
| **Structure** | Fields, entities, relations, DB migrations | `forma apply` from resource YAML | Yes — risk equivalent to changing a production schema |
| **Business logic** | Validation rules, conditions, action handlers | Starlark `script`/`script_ref`, editable from admin panel | **No** — already editable from admin panel without redeploy, with built-in versioning & rollback |

Entity extension sits in the **structure** layer — it modifies DDL and is committed to git. Business rules that reference extension fields can stay in the **business logic** layer — editable from the admin panel.

### Git as Source of Truth

All structural changes go through git. The CLI tools (`forma apply`, `forma diff`, signing in `forma-control`, audit trail) operate on versioned files. Git is not a style preference — it is a technical prerequisite:
- `forma diff` needs a "before" state to compare — that is a git commit, not a database state in a web editor
- Artifact signing needs an immutable, provenance-clear source (commit SHA)
- Two sources of truth (web editor + git) would create drift that destroys the audit guarantees Forma is built on

What IS supported: editing via GitHub Codespaces (still commits to git underneath), and a future "Forma Studio" that provides a drag-and-drop UI for resource design but outputs YAML commits to git via the Git API — never writing directly to a database.
