package auth

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/approval"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/ui"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// This file guards the ONE thing about role grants that no gate checks.
//
// `formspec validate` does not look inside `grants` — the field is free JSON on
// the role entity — and the resolver SKIPS a grant it cannot resolve rather
// than failing the role (deliberately: kafe 10.53). So a typo in a page or
// action name, or a `row_scope` under a key the loader does not read, produces a
// role that looks configured and enforces nothing. That is the kafe 10.47 /
// 10.10 failure class, and it is exactly how the rule "hanya pesanan LUNAS yang
// masuk dapur" could come back without anyone noticing: the SEED is what kafe
// runs, not the synthetic roles the e2e test builds.
//
// So this test reads the real seed file, materializes its grants with the real
// registries, and asserts that the row scopes the seed declares survive into the
// materialized permissions.

// kafeSeedRoleGrants returns the `grants` value of one role in the kafe seed,
// exactly as the role entity would store it (raw JSON, not typed).
func kafeSeedRoleGrants(t *testing.T, roleName string) []map[string]any {
	t.Helper()
	const specPath = "../../examples/kafe/spec"

	loaded, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load kafe tree: %v", err)
	}
	for _, perr := range loaded.Errors {
		t.Fatalf("parse error: %v", &perr)
	}

	for _, m := range loaded.Manifests {
		if spec.Kind(m.Kind) != spec.KindSeed {
			continue
		}
		raw, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		seedSpec, err := manifest.RawSpecTo[spec.SeedSpec](raw)
		if err != nil {
			continue
		}
		for _, group := range seedSpec.Entities {
			if group.Entity != "role" {
				continue
			}
			for _, rec := range group.Records {
				name, _ := rec["name"].(string)
				if name != roleName {
					continue
				}
				grants, _ := rec["grants"].([]any)
				out := make([]map[string]any, 0, len(grants))
				for _, g := range grants {
					if gm, ok := g.(map[string]any); ok {
						out = append(out, gm)
					}
				}
				if len(out) == 0 {
					t.Fatalf("role %q declares no grants in the seed", roleName)
				}
				return out
			}
		}
	}
	t.Fatalf("role %q not found in any kafe seed manifest", roleName)
	return nil
}

// kafeMaterializer builds the real Materializer over the kafe tree, the same way
// the server does at boot — including the workflow lookup, because a `workflow:`
// grant is unresolvable without it (production wires both together in
// `resource/formspec.go`; a test that wired only one would be measuring a
// deployment nobody runs).
func kafeMaterializer(t *testing.T) *Materializer {
	t.Helper()
	const specPath = "../../examples/kafe/spec"

	d, err := db.OpenSQLite(filepath.Join(t.TempDir(), "kafe_grants.db"), nil)
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
	m.SetApprovalDuties(kafeWorkflowDuties(t, specPath))
	return m
}

// kafeWorkflowDuties derives the tree's approval gates from its Entities'
// transition declarations into the lookup the materializer needs — the same
// resolution `wfReg.GetByName` provides at runtime.
func kafeWorkflowDuties(t *testing.T, specPath string) func(string) (string, []spec.DutyRef, bool) {
	t.Helper()
	loader := manifest.NewLoader(specPath)
	loaded, err := loader.LoadAll()
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

// grantsFromSeed converts the seed's raw `grants` value the way the role store
// does (`roleFromRecord` unmarshals the column into []Grant).
func grantsFromSeed(t *testing.T, raw []map[string]any) []Grant {
	t.Helper()
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal grants: %v", err)
	}
	var grants []Grant
	if err := json.Unmarshal(b, &grants); err != nil {
		t.Fatalf("unmarshal grants into []Grant: %v", err)
	}
	return grants
}

// TestKafeSeed_GrantsAllResolve is the missing gate: every page and action the
// kafe seed names must exist in the real tree. A name that does not resolve is
// silently dropped by the resolver, so the role keeps "working" with strictly
// less authority than the manifest says — or, for a row scope, with MORE reach
// than intended.
func TestKafeSeed_GrantsAllResolve(t *testing.T) {
	m := kafeMaterializer(t)

	for _, role := range []string{"kasir", "barista", "dapur", "pelayan", "supervisor", "manajer"} {
		grants := grantsFromSeed(t, kafeSeedRoleGrants(t, role))
		detailed, problems := m.MaterializeDetailed(grants)
		if len(problems) > 0 {
			for _, p := range problems {
				t.Errorf("role %s: %s — a grant that resolves to nothing is dropped silently at runtime", role, p)
			}
		}
		if len(detailed) == 0 {
			t.Errorf("role %s materialized to ZERO permissions", role)
		}
	}
}

// TestKafeSeed_KitchenRowScopeSurvives pins the actual rule: the kitchen roles
// carry the "paid and onward" predicate on `list`, and the waiter carries the
// servable one. If the seed's `row_scope` key were misspelled or nested wrong,
// the scope would vanish here — and the kitchen would read drafts again (10.67).
func TestKafeSeed_KitchenRowScopeSurvives(t *testing.T) {
	m := kafeMaterializer(t)

	type want struct {
		role   string
		status string
	}
	for _, tc := range []want{
		{role: "barista", status: "paid,in_kitchen,ready,served"},
		{role: "dapur", status: "paid,in_kitchen,ready,served"},
		{role: "pelayan", status: "ready,served,completed"},
	} {
		grants := grantsFromSeed(t, kafeSeedRoleGrants(t, tc.role))
		detailed, problems := m.MaterializeDetailed(grants)
		if len(problems) > 0 {
			t.Fatalf("role %s: unresolved grants %v", tc.role, problems)
		}

		var listScope []spec.FilterSpec
		for _, d := range detailed {
			if d.Permission == "cafe-order.orders.list" {
				listScope = d.RowScope
			}
		}
		if len(listScope) == 0 {
			t.Fatalf("role %s: cafe-order.orders.list carries NO row scope — the kitchen would read drafts again", tc.role)
		}

		var statusPred, sessionPred *spec.FilterSpec
		for i := range listScope {
			switch listScope[i].Field {
			case "status":
				statusPred = &listScope[i]
			case "branch_id":
				sessionPred = &listScope[i]
			}
		}
		if statusPred == nil {
			t.Fatalf("role %s: no `status` predicate on list (got %#v)", tc.role, listScope)
		}
		if statusPred.Op != "in" || statusPred.Value != tc.status {
			t.Errorf("role %s: status predicate = {op:%q value:%q}, want {in %q}", tc.role, statusPred.Op, statusPred.Value, tc.status)
		}
		// The branch half (GAP-08) must be a SESSION value, not a literal: a
		// literal branch would pin every kitchen to one branch in the manifest.
		if sessionPred == nil || sessionPred.From != "session" {
			t.Fatalf("role %s: branch predicate = %#v, want from: session", tc.role, sessionPred)
		}
	}
}

// TestKafeSeed_CashierHasNoStatusScope is the counterweight: the POS role must
// keep seeing the drafts it is composing. Declaring a status scope here would
// make the cashier's own new orders invisible to them.
func TestKafeSeed_CashierHasNoStatusScope(t *testing.T) {
	m := kafeMaterializer(t)
	grants := grantsFromSeed(t, kafeSeedRoleGrants(t, "kasir"))

	detailed, problems := m.MaterializeDetailed(grants)
	if len(problems) > 0 {
		t.Fatalf("kasir: unresolved grants %v", problems)
	}
	for _, d := range detailed {
		if d.Permission != "cafe-order.orders.list" {
			continue
		}
		for _, sc := range d.RowScope {
			if sc.Field == "status" {
				t.Fatalf("kasir's list carries a status row scope (%#v) — the cashier must see the drafts they are composing", sc)
			}
		}
	}
}

// The seed is data, so it must survive the same JSON round trip the store
// performs. Kept separate from the resolution tests so a shape break is reported
// as a shape break.
func TestKafeSeed_GrantsRoundTripThroughJSON(t *testing.T) {
	raw := kafeSeedRoleGrants(t, "barista")
	grants := grantsFromSeed(t, raw)
	if len(grants) != len(raw) {
		t.Fatalf("round trip changed the number of grants: %d → %d", len(raw), len(grants))
	}
	scopes := 0
	for _, g := range grants {
		for _, a := range g.Actions {
			scopes += len(a.RowScope)
		}
	}
	if scopes == 0 {
		t.Fatal("no row_scope survived the JSON round trip — the seed key and the Go tag disagree")
	}
	_ = context.Background
}
