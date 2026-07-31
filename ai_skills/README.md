# Forma AI Skills

Skills in this directory are **distributed with the `forma` CLI binary**
(via `//go:embed`) and written into new Forma projects by `forma init`.

They follow the [agentskills.io](https://agentskills.io/specification) format
and are placed in `.agents/skills/` in the scaffolded project so that
VS Code Copilot (and other compatible agents) can discover and use them.

## Skill Inventory

| Skill | Folder | Purpose |
|-------|--------|---------|
| Spec Structure | `forma-spec-structure/` | Navigate `docs/spec/` — which file covers what, doc status lifecycle, contract-vs-renderer principle |
| Kinds Catalog | `forma-kinds/` | Complete catalog of all ~34 Forma resource kinds — when to use each, manifest format, gotchas |

## Maintenance

When `docs/spec/` changes (new kinds added, spec files reorganized):

1. Update the relevant `SKILL.md` file(s) in this directory
2. Run `go build ./cmd/forma` to verify the embedded files compile
3. Commit both the skill changes and the binary

### Adding a New Skill

1. Create a subdirectory: `ai_skills/<skill-name>/SKILL.md`
2. The folder name must match the `name` field in YAML frontmatter
3. Use lowercase-hyphens for the name (spec requirement)
4. The `//go:embed ai_skills/*/SKILL.md` pattern will automatically pick it up

### Format Reference

See https://agentskills.io/specification for the complete SKILL.md format.
