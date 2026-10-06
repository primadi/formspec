package auth

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/approval"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// ─── The same gate as kafe_seed_grants_test.go, for EVERY example ───
//
// That file guards kafe's seed, because the SEED is what kafe runs — and the
// failure it guards is invisible: `formspec validate` does not look inside
// `grants` (free JSON on the role entity), and the resolver SKIPS a grant it
// cannot resolve rather than failing the role (deliberately, kafe 10.53). A
// typo therefore produces a role that looks configured and enforces nothing.
//
// Guarding only kafe left the other trees unguarded, and that is not
// hypothetical: adding a duty grant to `crc-management` and `service-demo` was
// exactly the kind of change this test exists to check, and nothing would have
// caught a mistyped page or step name in it. Examples are what people copy, so a
// broken grant there is copied too.
//
// The test discovers the trees instead of listing them, so a new example with a
// role seed is covered the moment it exists.

// exampleSpecDirs returns every example tree that declares a role seed.
func exampleSpecDirs(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob("../../examples/*/spec")
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	var out []string
	for _, dir := range matches {
		if len(roleSeedGrants(t, dir)) > 0 {
			out = append(out, dir)
		}
	}
	if len(out) == 0 {
		t.Fatal("no example declares role seeds — the discovery glob is wrong")
	}
	return out
}

// roleSeedGrants returns every role's raw `grants` value in a tree's seed
// manifests, keyed by role name. Raw (not typed) on purpose: it is exactly what
// the role entity stores.
func roleSeedGrants(t *testing.T, specPath string) map[string][]map[string]any {
	t.Helper()
	loaded, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load %s: %v", specPath, err)
	}
	out := map[string][]map[string]any{}
	for _, m := range loaded.Manifests {
		if spec.Kind(m.Kind) != spec.KindSeed {
			continue
		}
		raw, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		seedSpec, err := manifest.RawSpecTo[spec.SeedSpec](raw)
		if err != nil || seedSpec == nil {
			continue
		}
		for _, group := range seedSpec.Entities {
			if !isRoleEntityRef(group.Entity) {
				continue
			}
			for _, rec := range group.Records {
				name, _ := rec["name"].(string)
				if name == "" {
					continue
				}
				grants, _ := rec["grants"].([]any)
				rawGrants := make([]map[string]any, 0, len(grants))
				for _, g := range grants {
					if gm, ok := g.(map[string]any); ok {
						rawGrants = append(rawGrants, gm)
					}
				}
				out[name] = rawGrants
			}
		}
	}
	return out
}

// isRoleEntityRef matches the role entity, which the framework module owns
// (`formspec.core.role`) but a project may also name with its module prefix.
// (The CLI has the same predicate for the deploy-time shape check; it is two
// lines and lives in `package main`, so it is stated here rather than exported.)
func isRoleEntityRef(name string) bool {
	return name == "role" || strings.HasSuffix(name, ".role")
}

// materializerFor builds the real Materializer over one example tree the way the
// server does at boot — registries plus the workflow lookup a `workflow:` grant
// needs.
func materializerFor(t *testing.T, specPath string) *Materializer {
	t.Helper()
	d, err := db.OpenSQLite(filepath.Join(t.TempDir(), "grants.db"), nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, specPath)
	if err := reg.LoadEntities(); err != nil {
		t.Fatalf("load entities: %v", err)
	}
	uiReg := ui.NewRegistry()
	if errs := uiReg.LoadDir(specPath); len(errs) > 0 {
		t.Fatalf("load ui manifests: %v", errs)
	}
	m := NewMaterializer(uiReg, reg)
	m.SetApprovalDuties(workflowDutiesFor(t, specPath))
	return m
}

// workflowDutiesFor derives a tree's approval gates from its Entities'
// transition declarations into the lookup the materializer needs — the same
// resolution `wfReg.GetByName` provides at runtime.
func workflowDutiesFor(t *testing.T, specPath string) func(string) (string, []spec.DutyRef, bool) {
	t.Helper()
	loaded, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}
	wfReg := approval.NewRegistry()
	for _, raw := range loaded.Manifests {
		if spec.Kind(raw.Kind) != spec.KindEntity || raw.Spec == nil {
			continue
		}
		specMap, ok := raw.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || es == nil {
			continue
		}
		wfReg.AddEntity(raw.Metadata.Module, raw.Metadata.Name, es)
	}
	return func(name string) (string, []spec.DutyRef, bool) {
		module, a, ok := wfReg.GetByName(name)
		if !ok || a == nil {
			return "", nil, false
		}
		return module, spec.ApprovalDuties(module, name, a), true
	}
}

// TestAllExamples_SeedGrantsResolve is the general form of
// TestKafeSeed_GrantsAllResolve: every page, action and duty a role seed names
// must resolve in the tree it ships with.
func TestAllExamples_SeedGrantsResolve(t *testing.T) {
	for _, specPath := range exampleSpecDirs(t) {
		name := filepath.Base(filepath.Dir(specPath))
		t.Run(name, func(t *testing.T) {
			m := materializerFor(t, specPath)
			for role, raw := range roleSeedGrants(t, specPath) {
				if len(raw) == 0 {
					// A role with no grants is legitimate (a placeholder, or a
					// service account); only unresolvable NAMES are the bug.
					continue
				}
				b, err := json.Marshal(raw)
				if err != nil {
					t.Fatalf("role %s: marshal grants: %v", role, err)
				}
				var grants []Grant
				if err := json.Unmarshal(b, &grants); err != nil {
					t.Fatalf("role %s: unmarshal grants: %v", role, err)
				}
				detailed, problems := m.MaterializeDetailed(grants)
				for _, p := range problems {
					t.Errorf("role %s: %s — a grant that resolves to nothing is dropped silently at runtime", role, p)
				}
				if len(detailed) == 0 {
					t.Errorf("role %s materialized to ZERO permissions", role)
				}
			}
		})
	}
}

// TestAllExamples_ApprovalStepsAreSatisfiable is the other half of the same
// guarantee, and it is the one 5.13.9 was about.
//
// A workflow step is satisfiable when SOMETHING can satisfy it: a duty that a
// role is actually granted, or a role that a seed declares. Checking only that
// the manifest parses is how three crc workflows and one service-demo workflow
// named roles that did not exist in their own trees — `hasAnyRole` compares
// names literally, so those approvals could not be approved by anyone, forever,
// with every gate green.
func TestAllExamples_ApprovalStepsAreSatisfiable(t *testing.T) {
	for _, specPath := range exampleSpecDirs(t) {
		name := filepath.Base(filepath.Dir(specPath))
		t.Run(name, func(t *testing.T) {
			roles := roleSeedGrants(t, specPath)
			// Every duty a role actually holds, so a step's `permission` can be
			// checked against it rather than merely existing.
			granted := map[string]bool{}
			for _, raw := range roles {
				for _, g := range raw {
					page, _ := g["page"].(string)
					if !strings.HasPrefix(page, "workflow:") {
						continue
					}
					actions, _ := g["actions"].([]any)
					for _, a := range actions {
						am, ok := a.(map[string]any)
						if !ok {
							continue
						}
						if action, ok := am["name"].(string); ok {
							granted[strings.TrimPrefix(page, "workflow:")+":"+action] = true
						}
					}
				}
			}

			loaded, err := manifest.NewLoader(specPath).LoadAll()
			if err != nil {
				t.Fatalf("load %s: %v", specPath, err)
			}
			for _, raw := range loaded.Manifests {
				if spec.Kind(raw.Kind) != spec.KindEntity || raw.Spec == nil {
					continue
				}
				specMap, ok := raw.Spec.(map[string]any)
				if !ok {
					continue
				}
				es, err := manifest.RawSpecToEntitySpec(specMap)
				if err != nil || es == nil || es.StateMachine == nil {
					continue
				}
				for _, tr := range es.StateMachine.Transitions {
					if tr.Action == "" || tr.Approval == nil {
						continue
					}
					// The gate's name is `{entity}.{transition}` — the same key a
					// grant's `workflow:{name}` and the stored row use.
					gateName := raw.Metadata.Name + "." + tr.Action
					for i, step := range tr.Approval.Steps {
						if len(step.Roles) == 0 && step.Permission == "" {
							t.Errorf("%s step %d(%s): declares neither roles nor permission — nobody can approve it",
								gateName, i, step.Name)
						}
						for _, r := range step.Roles {
							if _, ok := roles[r]; !ok {
								t.Errorf("%s step %d(%s): role %q is not declared in %s — that approval can never be granted",
									gateName, i, step.Name, r, name)
							}
						}
						if step.Permission != "" {
							if !granted[gateName+":"+step.Name] {
								t.Errorf("%s step %d(%s): no role is granted the duty %q — declaring it is half the contract, granting it is the other half",
									gateName, i, step.Name, spec.StepPermission(raw.Metadata.Module, gateName, step))
							}
						}
						// An escalation target is a DUTY too, and it is not a step:
						// unchecked, it names a permission nobody holds and the
						// stalled approval can never be unblocked.
						if step.Escalation != nil && step.Escalation.Reassign != "" {
							if !granted[gateName+":"+step.Escalation.Reassign] {
								t.Errorf("%s step %d(%s): escalation reassigns to %q but no role is granted that duty — the takeover would never be possible",
									gateName, i, step.Name, step.Escalation.Reassign)
							}
						}
					}
				}
			}
		})
	}
}
