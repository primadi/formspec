# FormSpec AI Skills

Skills in this directory are **distributed with the `formspec` CLI binary**
(via `//go:embed`) and written into new FormSpec projects by `formspec init`.

They follow the [agentskills.io](https://agentskills.io/specification) format
and are placed in `.agents/skills/` in the scaffolded project so that
VS Code Copilot (and other compatible agents) can discover and use them.

## Skill Inventory

| Skill | Folder | Purpose |
|-------|--------|---------|
| Spec Structure | `formspec-spec-structure/` | Navigate `docs/spec/` — which file covers what, doc status lifecycle, contract-vs-renderer principle |
| Kinds Catalog | `formspec-kinds/` | Complete catalog of all 33 FormSpec resource kinds grouped in 4 categories (Curation/Data/UI/Infra) + UI 3-layer wrapping model |
| App Workflow | `formspec-app-workflow/` | Full lifecycle orchestrator — Discovery → Proposal → Draft (YAML spec) → Iterate (change management) |
| Schema Validation | `schema-validation/` | Run `formspec validate`, interpret engine vs schema errors, and repair all manifests to canonical form (generate → validate → fix → re-validate) |
| Entity Authoring | `entity-authoring/` | Author Entity manifests — field types, characteristic, natural_key, state machine, actions/uses, expose (todo 10.6.2) |
| Form Layout | `form-layout/` | Form kind — layout modes, widget per field type, FormSpecExpr (visible_when/compute), child table |
| Entity Extension | `entity-extension-authoring/` | Add fields/validation to vendor entities — extension vs shadow copy vs new action |
| Module Vendoring | `module-vendoring/` | modules/ vs vendors/ (read-only), overrides/, formspec.lock, trust tiers |

## Maintenance

When `docs/spec/` changes (new kinds added, spec files reorganized):

1. Update the relevant `SKILL.md` file(s) in this directory
2. Run `go build ./cmd/formspec` to verify the embedded files compile
3. Commit both the skill changes and the binary

### Adding a New Skill

1. Create a subdirectory: `ai_skills/<skill-name>/SKILL.md`
2. The folder name must match the `name` field in YAML frontmatter
3. Use lowercase-hyphens for the name (spec requirement)
4. The `//go:embed ai_skills/*/SKILL.md` pattern will automatically pick it up

### Format Reference

See https://agentskills.io/specification for the complete SKILL.md format.
