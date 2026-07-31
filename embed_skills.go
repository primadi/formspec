// Package forma embeds AI skills distributed with the CLI binary.
// These skills are written to new projects by `forma init` so that
// VS Code Copilot (and compatible agents) can assist with Forma app
// development using domain-specific knowledge.
//
// Skills follow the agentskills.io specification:
// https://agentskills.io/specification
package forma

import "embed"

// AISkillsFS embeds all SKILL.md files from the ai_skills/ directory.
// Pattern ai_skills/*/SKILL.md matches each skill's SKILL.md in its
// own subdirectory (e.g., ai_skills/forma-kinds/SKILL.md).
//
//go:embed ai_skills/*/SKILL.md
var AISkillsFS embed.FS
