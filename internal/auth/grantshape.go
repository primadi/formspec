package auth

import (
	"fmt"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// This file is the single reader of the RAW `grants` value of a role.
//
// Why raw, when the rest of the package works with typed `[]Grant`: `grants` is
// free JSON in a jsonb column, with no schema behind it. `json.Unmarshal` into
// `Grant` silently DROPS a key it does not know, so `row_scopes:` instead of
// `row_scope:` produces a role that looks configured and carries no row
// restriction at all — the 10.67 measurement, where the kitchen read drafts
// again. A typed read cannot see the typo; only the raw value can.
//
// So validation happens on the raw value (here), and the typed read stays the
// happy path. Problems are reported with a `RowScope` flag because the two
// kinds are not equally dangerous: a misspelled page name costs the grant, while
// an unreadable ROW RESTRICTION is fail-OPEN (the permission survives, the
// restriction does not), so the runtime must deny rather than log and continue.

// GrantShapeProblem is one defect found in a role's raw `grants` value.
type GrantShapeProblem struct {
	// Path locates the defect (e.g. `grants[0].actions[2].row_scopes`).
	Path string
	// Message explains what is wrong and what to write instead.
	Message string
	// RowScope marks a defect that touches a row restriction. The runtime denies
	// the permission when any such defect exists; defects that only lose a grant
	// are logged (the permission simply is not granted).
	RowScope bool
}

func (p GrantShapeProblem) String() string { return p.Path + ": " + p.Message }

// ValidateGrantListShape validates a raw `grants` value (as stored on a role
// record, or written in a `kind: Seed` manifest) against the shape the
// Materializer reads.
//
// It is deliberately registry-free: it can say "this key is misspelled" or "this
// row_scope entry declares no value source" without knowing the spec tree. What
// it cannot check — that the page resolves, that the action is in the page's
// footprint, that the row_scope field exists on the target entity — needs the
// registries and is done by the Materializer (whose problems carry the same
// RowScope flag).
func ValidateGrantListShape(raw any) []GrantShapeProblem {
	if raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return []GrantShapeProblem{{Path: "grants", Message: "must be a list of page grants"}}
	}
	var problems []GrantShapeProblem
	for i, rawGrant := range list {
		path := fmt.Sprintf("grants[%d]", i)
		grant, ok := rawGrant.(map[string]any)
		if !ok {
			problems = append(problems, GrantShapeProblem{Path: path, Message: "must be an object with `page` and `actions`"})
			continue
		}
		page, _ := grant["page"].(string)
		if strings.TrimSpace(page) == "" {
			problems = append(problems, GrantShapeProblem{Path: path, Message: "`page` is required"})
		}

		_, hasActions := grant["actions"]
		_, hasTabs := grant["tabs"]
		if !hasActions && !hasTabs {
			problems = append(problems, GrantShapeProblem{Path: path, Message: "must declare either `actions` or `tabs`"})
		}
		for _, key := range []string{"actions", "tabs"} {
			if v, ok := grant[key]; ok {
				problems = append(problems, validateGrantGroup(v, path+"."+key)...)
			}
		}
	}
	return problems
}

func validateGrantGroup(raw any, path string) []GrantShapeProblem {
	list, ok := raw.([]any)
	if !ok {
		return []GrantShapeProblem{{Path: path, Message: "must be a list"}}
	}
	var problems []GrantShapeProblem
	for i, entry := range list {
		entryPath := fmt.Sprintf("%s[%d]", path, i)
		m, ok := entry.(map[string]any)
		if !ok {
			problems = append(problems, GrantShapeProblem{Path: entryPath, Message: "must be an object"})
			continue
		}
		// A tab carries a label plus its own actions; an action carries a name.
		if _, isTab := m["tab"]; isTab {
			tab, _ := m["tab"].(string)
			if strings.TrimSpace(tab) == "" {
				problems = append(problems, GrantShapeProblem{Path: entryPath, Message: "`tab` is required"})
			}
			actions, ok := m["actions"]
			if !ok {
				problems = append(problems, GrantShapeProblem{Path: entryPath, Message: "tab must declare `actions`"})
				continue
			}
			problems = append(problems, validateGrantGroup(actions, entryPath+".actions")...)
			continue
		}
		problems = append(problems, validateGrantAction(m, entryPath)...)
	}
	return problems
}

func validateGrantAction(m map[string]any, path string) []GrantShapeProblem {
	var problems []GrantShapeProblem
	name, _ := m["name"].(string)
	if strings.TrimSpace(name) == "" {
		problems = append(problems, GrantShapeProblem{Path: path, Message: "`name` is required"})
	}

	// The typo this whole file exists for. `row_scopes` is not read by anything,
	// so without this check the role keeps its permission and loses the
	// restriction — fail-open, silently.
	if _, wrong := m["row_scopes"]; wrong {
		problems = append(problems, GrantShapeProblem{
			Path:     path + ".row_scopes",
			Message:  "unknown key `row_scopes` — the row restriction must be written as `row_scope`; as written it is ignored and the action is granted on EVERY row",
			RowScope: true,
		})
	}

	rawScope, hasScope := m["row_scope"]
	if !hasScope {
		return problems
	}
	list, ok := rawScope.([]any)
	if !ok {
		problems = append(problems, GrantShapeProblem{
			Path: path + ".row_scope", Message: "must be a list of filters", RowScope: true,
		})
		return problems
	}
	// Convert the raw list through the shared FilterSpec reader, then apply the
	// same value-source rules the entity's own `row_scope` is held to.
	filters, err := decodeFilterSpecs(list)
	if err != nil {
		problems = append(problems, GrantShapeProblem{Path: path + ".row_scope", Message: err.Error(), RowScope: true})
		return problems
	}
	if err := spec.ValidateRowScopeFilters(path+".row_scope", filters, nil); err != nil {
		problems = append(problems, GrantShapeProblem{Path: path + ".row_scope", Message: err.Error(), RowScope: true})
	}
	return problems
}

// decodeFilterSpecs reads a raw filter list into []spec.FilterSpec without a
// YAML/JSON round trip, so the field allowlist above cannot be bypassed by a
// shape the decoder would have rejected.
func decodeFilterSpecs(list []any) ([]spec.FilterSpec, error) {
	out := make([]spec.FilterSpec, 0, len(list))
	for i, raw := range list {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("entry %d must be an object", i)
		}
		fs := spec.FilterSpec{}
		// A key that is not a string (or is absent) is read as empty, which the
		// shared validator then rejects — never silently ignored.
		fs.Field, _ = m["field"].(string)
		fs.Op, _ = m["op"].(string)
		fs.Value, _ = m["value"].(string)
		fs.From, _ = m["from"].(string)
		fs.Attr, _ = m["attr"].(string)
		fs.Param, _ = m["param"].(string)
		for _, key := range []string{"field", "op", "value", "from", "attr", "param"} {
			if v, present := m[key]; present {
				if _, isString := v.(string); !isString {
					return nil, fmt.Errorf("entry %d: `%s` must be a string", i, key)
				}
			}
		}
		out = append(out, fs)
	}
	return out, nil
}
