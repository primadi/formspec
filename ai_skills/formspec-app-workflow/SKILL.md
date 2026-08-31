---
name: formspec-app-workflow
description: Orchestrates the full FormSpec application creation lifecycle — Discovery, Proposal, Draft (YAML spec), and Iterate (change management). Use when building a new FormSpec application from scratch, adding features to an existing app, or making changes that span multiple phases. This is the orchestrator skill; it delegates to formspec-kinds for kind syntax, schema-validation for validation, and formspec-spec-structure for spec file navigation.
metadata:
  version: "1.0"
  source: docs/ai/02-formspec-consult.md + diskusi plan mode 2026-08-10
---

# FormSpec App Workflow — Full Lifecycle Orchestrator

Guides AI through the complete lifecycle of creating and maintaining a
FormSpec application. The workflow is **4 sequential phases** — AI must not
skip phases or jump directly to writing YAML before business needs are
confirmed.

```
Discovery ──→ Proposal ──→ Draft ──→ Iterate
    │              │           │          │
    ▼              ▼           ▼          ▼
overview.md   architecture.md  spec/    changelog
              domain-model.md  modules/   + update
                               apps/    affected layers
```

---

## Phase Detection — Where Are We?

Phase is determined by **conversation state**, not file existence. The
three signals below are ordered by authority — resolve in this order.

### 1. User request type (primary)

- "Buat aplikasi X baru" / "merancang sistem Y" → **Discovery** (entry point)
- "Tambah fitur Z" / "ubah aturan bisnis" → **Iterate**
- "Perbaiki YAML / validation error" → Iterate (Draft-layer repair)

### 2. Confirmation gates (what holds progression)

Phases advance only through **explicit user approval** in the conversation:

```
Discovery ──(user approves overview)──→ Proposal ──(user approves proposal)──→ Draft ──→ Iterate
```

- If `docs/overview.md` exists but the user has **not** approved it, you are
  still in Discovery.
- If the user says "setuju, lanjut", you move to the next phase even before
  the output file is written.

### 3. Workspace artifacts (hint only — never authoritative)

File presence is context, not a state machine. Use it to orient, not to decide:

| Artifact present                                | Interpretation (hint)        |
| ----------------------------------------------- | ---------------------------- |
| No `spec/`, no `docs/`                          | Empty project → Discovery    |
| `docs/overview.md` only                         | Discovery done (if approved) |
| `docs/architecture.md` + `docs/domain-model.md` | Proposal done (if approved)  |
| `spec/modules/**` + `spec/apps/*.yaml`          | Draft done → Iterate         |

Warning: docs can be stale or written ad-hoc. A user asking for a _new_
feature on a fully-built app is in **Iterate**, not Discovery — trust the
request over the artifact.

### Default when uncertain

Ask the user. Never guess between Discovery and Iterate silently — the two
have opposite starting moves (probing questions vs. change classification).

---

## Phase 1: Discovery

**Goal**: Understand business needs in plain language. Output a summary
that a business owner can read and confirm.

**Output**: `docs/overview.md`

**Content**:

- What the application does (business purpose)
- Key business goals (numbered, clear)
- Core workflows (who does what, in what order)
- Business rules (constraints, policies, special cases)
- Tech stack note (FormSpec + database + runtime)

**Rules**:

- Use **plain language** — no FormSpec jargon, no YAML, no technical kind names
- Ask probing questions actively — don't just passively record what the user says
- If the user mentions a known business pattern (e.g., POS, inventory, clinic),
  use the probing questions from the industry template if available
- **Confirm with the user before moving to Proposal** — the summary must be
  explicitly approved

**Example probing questions**:

- "Who are the users? What roles do they have?"
- "What data is entered? By whom? How often?"
- "What are the approval or review steps?"
- "Are there any reports or dashboards needed?"
- "What external systems does this connect to?"

**Example `docs/overview.md`**:

```markdown
# Overview — Aplikasi Arisan

## Apa Itu Arisan

Arisan adalah aplikasi untuk mengelola arisan berbentuk rotating savings
club: sekelompok anggota menyetor iuran bulanan tetap ke rekening bersama,
lalu setiap bulan salah satu anggota memenangkan undian.

## Tujuan Bisnis

1. Kelola grup arisan, anggota, dan keanggotaan
2. Catat mutasi bank dan iuran anggota
3. Cocokkan iuran dengan mutasi bank
4. Jalankan undian & catat penarikan
5. Dashboard ringkasan & rekap iuran

## Tech Stack

- Framework: FormSpec (spec-first, declarative YAML)
- Database: PostgreSQL / SQLite (dev)
```

---

## Phase 2: Proposal

**Goal**: Map business needs to FormSpec constructs. Decide module boundaries,
entity characteristics, state machines, and which entities need UI overrides.

**Output**:

- `docs/architecture.md` — module boundaries, design decisions, dependencies
- `docs/domain-model.md` — entity list, key fields, state machines (Mermaid), characteristics

**Skills to delegate to**:

- `formspec-kinds` — for choosing the right kind and characteristic per entity
- `formspec-spec-structure` — for navigating which spec doc covers what

**Content — `architecture.md`**:

- Module breakdown (bounded contexts) with rationale
- Entity characteristic assignments (master/transaction/reference/summary) with justification
- Key design decisions (why plain_crud vs doc_status, why 3 modules vs 1, etc.)
- Module dependencies (who depends on whom)
- Which entities need UI overrides vs auto-derived

**Content — `domain-model.md`**:

- ER diagram (Mermaid) showing all entities and relationships
- Per entity: field list (key fields only — not exhaustive), characteristics, state machine diagram, actions
- Indexes and uniqueness constraints
- Not exhaustive — detail lives in spec YAML (see Phase 3)

**Rules**:

- For each entity, explicitly decide: `master`, `transaction`, `reference`, or `summary`
- For each entity, decide: `plain_crud` or `doc_status` lifecycle
- State machines: diagram the states and transitions, name the triggering actions
- Mark entities that likely need UI overrides (custom Form layout, custom Page composition)
- **Confirm with the user before moving to Draft**

**Decision checklist** (AI must answer all before proceeding):

- [ ] Module boundaries defined and justified
- [ ] All entities have characteristic assigned
- [ ] All state machines diagrammed (if any)
- [ ] Module dependencies mapped
- [ ] UI override decisions made (derived vs custom)

---

## Phase 3: Draft

**Goal**: Write FormSpec YAML manifests. Start from entities, add overrides
only where needed, validate everything.

**Output**: `spec/modules/<module>/*.yaml` + `spec/apps/<app>.yaml`

**Skills to delegate to**:

- `formspec-kinds` — for correct YAML syntax per kind
- `schema-validation` — for `formspec validate` → classify → repair → re-validate

**Write order** (strict):

1. `Module` manifests (`module.yaml`) — one per bounded context
2. `Entity` manifests (`entity.yaml`) — all entities in all modules
3. UI overrides — only for entities flagged in Proposal
   - `Form` (`form.yaml`) — when field order/layout/visibility differs from default
   - `Table` (`table.yaml`) — when column selection/sort differs from default
   - `Page` (`page.yaml`) — when composition spans multiple entities
4. `App` manifest (`apps/<name>.yaml`) — curation + menu (Dashboard landing
   at top if present; modules ordered by access frequency — see
   `formspec-kinds` Menu section)
5. Other kinds as needed (`Dashboard`, `Report`, `Config`, `Workflow`, etc.)

**Validation gate** (must pass before Phase 3 is complete):

```bash
formspec validate --spec spec
```

Expected: `0 problem(s) found`. If errors: use `schema-validation` skill to fix.

**Rules**:

- Every Entity must have: `spec.version: v1`, `metadata.description`, correct characteristic
- Use `relation: { type: belongs_to, resource: <mod.entity> }` — never bare `target:`
- `expose` is an array of `{type, actions}` — never `all`/`read`/`none`
- `lifecycle` is a string enum — never `{doc_status: true}`
- 80-95% of entities need ZERO UI overrides — don't over-engineer
- For features not yet implemented in the engine, add YAML comment: `# TODO: menunggu implementasi <fitur>`
- After every file write, run `formspec validate` to catch errors early

**Example entity write flow**:

```
1. Write spec/modules/billing/module.yaml
2. Write spec/modules/billing/invoice/entity.yaml
3. Write spec/modules/billing/customer/entity.yaml
4. Run: formspec validate --spec spec
5. Fix any errors (delegate to schema-validation)
6. (No UI overrides needed for these entities)
7. Write spec/apps/billing-app.yaml
8. Run: formspec validate --spec spec
```

---

## Phase 4: Iterate

**Goal**: Handle changes after the initial spec is complete. Maintain
traceability between documentation and spec.

**Trigger**: Any change request — new feature, modified business rule,
restructured module, UI layout adjustment.

**Change classification** — determine which layer(s) are affected:

| Change type                                                   | Affected layers              | Action                           |
| ------------------------------------------------------------- | ---------------------------- | -------------------------------- |
| Business requirement change (new goal, new workflow)          | Discovery → Proposal → Draft | Update all three, top-down       |
| Technical design change (split module, change characteristic) | Proposal → Draft             | Update proposal first, then spec |
| Spec detail change (add field, change validation, add index)  | Draft only                   | Update spec YAML only            |

**Top-down update rule**:

```
Business need changes?
  → Update docs/overview.md first
  → Then update docs/architecture.md + docs/domain-model.md
  → Then update spec/ YAML files
  → Run formspec validate
  → Write changelog

Design changes?
  → Update docs/architecture.md + docs/domain-model.md
  → Then update spec/ YAML files
  → Run formspec validate
  → Write changelog

Field/validation changes?
  → Update spec/ YAML files only
  → Run formspec validate
  → Write changelog
```

**Changelog**:

- File: `docs_internal/changelog/YYYY-MM-DD-NNN-<deskripsi-singkat>.md`
- `NNN` = 3-digit sequence number, resets per day
- Content: what changed, which layer(s), why, files affected
- Example: `docs_internal/changelog/2026-08-10-001-add-discount-field-to-invoice.md`

**Consistency check** (run after every iteration):

1. Does `architecture.md` still match the actual module structure in `spec/`?
2. Does `domain-model.md` still list all entities with correct characteristics?
3. Does `formspec validate --spec spec` pass?
4. Is the changelog entry written?

---

## Key Rules Across All Phases

1. **Phases cannot be skipped.** AI must get explicit confirmation before
   moving to the next phase. Never jump from Discovery directly to Draft.

2. **Docs ≠ Spec.** Documentation (`docs/`) contains design decisions,
   rationale, and big-picture context. Spec (`spec/`) contains precise
   machine-enforceable definitions. They serve different audiences and
   should not duplicate each other.

3. **No redundancy.** Detailed field definitions, validations, indexes,
   and permissions live ONLY in spec YAML. Docs summarize at a higher level.
   When a field changes, update spec only — not docs (unless the change
   reflects a design decision).

4. **95% of cases: Entity is enough.** The engine auto-derives Table,
   Form (create+edit), Page (detail), REST API, and admin menu from Entity
   alone. Only declare UI kinds when the auto-derived result is genuinely
   insufficient.

5. **Permission = resource + action.** Never hardcode role names in YAML.
   `required_permission: <module>.<entity>.<action>`.

6. **Validate early, validate often.** Run `formspec validate` after every
   significant change, not just at the end. Catch errors immediately.

7. **For unimplemented features**, add `# TODO:` comments in YAML rather
   than omitting the declaration. This ensures the spec is complete even
   if the engine can't yet enforce it.

8. **Changelog discipline.** Every change, no matter how small, gets a
   changelog entry. This is non-negotiable for traceability.

---

## No-MCP Tool Map (Current Mechanism)

There is no MCP server / `formspec-consult` yet (that is deferred design —
`docs/ai/`, todo.md Fase 10). The agent works directly in the project using
the available CLI + files. Map each workflow need to the no-MCP equivalent:

| MCP design (deferred)                                    | No-MCP equivalent today                                                                  |
| -------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `read_workspace_manifest()` / `list_installed_modules()` | Read `spec/apps/*.yaml` + `spec/modules/*/module.yaml` directly                          |
| `list_skills()`                                          | In VS Code Copilot: `/skills` — or read `.agents/skills/<name>/SKILL.md`                 |
| `read_skill(name)`                                       | Read the skill file from `.agents/skills/<name>/SKILL.md`                                |
| `list_kind_schemas(kind)`                                | Read `schemas/kinds/<Kind>.schema.json` (generated)                                      |
| `validate_spec(yaml)`                                    | Run `formspec validate --spec spec` in the terminal (engine + JSON Schema) — must exit 0 |
| `propose_spec_file(path, content)`                       | Write the YAML file directly to `spec/...`, then validate                                |
| `apply_draft(session, file)`                             | Commit / stage the file; use `git diff` + `git status` for review                        |
| `check_naming_conflict(name)`                            | `formspec validate` catches duplicates; grep `spec/` for the name                        |
| `restart_server()` / dev control                         | Run `formspec dev` in the terminal (restart manually)                                    |

Rules:

- **Validate after every significant write** — `formspec validate --spec spec`
  is the gate for Draft and Iterate. Target: `0 problem(s) found`.
- Build the binary first if it is stale: `make build`, or run
  `go run ./cmd/formspec validate --spec <path>` from the formspec repo.
- Keep this skill's content **MCP-agnostic** — when Fase 10 lands, the map
  above becomes an optional transport, not a rewrite.

---

## Skill Delegation Map

| When AI needs to...                     | Delegate to               |
| --------------------------------------- | ------------------------- |
| Know the correct YAML syntax for a kind | `formspec-kinds`          |
| Know which spec doc covers a topic      | `formspec-spec-structure` |
| Validate and fix YAML manifests         | `schema-validation`       |
| Understand the overall workflow         | (this skill)              |

---

## Quick Reference: Phase Outputs

| Phase     | Output File(s)                                  | Audience           | Validation                                     |
| --------- | ----------------------------------------------- | ------------------ | ---------------------------------------------- |
| Discovery | `docs/overview.md`                              | Business owner     | User confirms                                  |
| Proposal  | `docs/architecture.md` + `docs/domain-model.md` | Developer + owner  | User confirms                                  |
| Draft     | `spec/modules/**/*.yaml` + `spec/apps/*.yaml`   | Engine + developer | `formspec validate` passes                     |
| Iterate   | Changelog + updated files                       | Developer          | `formspec validate` passes + consistency check |
