package summary

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/subscription"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// setupRegistry loads the fixture spec tree into a real entity registry and
// pairs it with the durable subscription that maintains the stock-level
// projection. A real registry (not a stub) is the point: the plan is resolved
// from what the engine actually registered.
func setupRegistry(t *testing.T) (*entity.Registry, *subscription.Registry) {
	t.Helper()

	d, err := db.OpenSQLite(filepath.Join(t.TempDir(), "summary_test.db"), nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, "fixtures/spec")
	for _, err := range reg.LoadEntities() {
		t.Fatalf("load entities: %v", err)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}

	subReg := subscription.NewRegistry()
	subReg.Add("warehouse", "stock-projection", &spec.SubscriptionSpec{
		Events:  []string{"warehouse.stock-movement.on_post"},
		Handler: spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "warehouse/apply-stock-movement"},
		Durable: "durable",
	})
	return reg, subReg
}

func TestPlanRebuild_ResolvesContractAndStreams(t *testing.T) {
	reg, subReg := setupRegistry(t)

	plan, err := PlanRebuild(reg, subReg, "warehouse/stock-level")
	if err != nil {
		t.Fatalf("PlanRebuild: %v", err)
	}

	if plan.Module != "warehouse" || plan.Entity != "stock-level" {
		t.Errorf("resolved %s/%s, want warehouse/stock-level", plan.Module, plan.Entity)
	}
	// `rebuild.strategy: partial` with a window must survive into the plan —
	// it is what tells an operator whether --reset is legal.
	if plan.Strategy != "partial" {
		t.Errorf("strategy: got %q, want partial", plan.Strategy)
	}
	if plan.Window != "month" {
		t.Errorf("window: got %q, want month", plan.Window)
	}
	if plan.JoinKey != "m.ingredient_id = i.id" {
		t.Errorf("join_key: got %q", plan.JoinKey)
	}

	wantSources := []string{"warehouse/ingredient", "warehouse/stock-movement"}
	if strings.Join(plan.Sources, ",") != strings.Join(wantSources, ",") {
		t.Errorf("sources: got %v, want %v", plan.Sources, wantSources)
	}

	// The table comes from the synced DDL, so a reset can delete exactly the
	// projection's rows.
	if plan.Table == "" || !strings.HasSuffix(plan.Table, "stock_levels") {
		t.Errorf("table: got %q, want a *_stock_levels table", plan.Table)
	}

	if len(plan.Streams) != 1 {
		t.Fatalf("streams: got %d (%v), want 1", len(plan.Streams), plan.Streams)
	}
	got := plan.Streams[0]
	if got.EventName != "warehouse.stock-movement.on_post" || got.Subscription != "warehouse/stock-projection" {
		t.Errorf("stream: got %+v", got)
	}

	// ingredient has no durable subscriber, so it cannot be replayed. Reporting
	// it (rather than dropping it) is what keeps a partial rebuild from looking
	// like a complete one.
	if len(plan.Orphaned) != 1 || plan.Orphaned[0] != "warehouse/ingredient" {
		t.Errorf("orphaned: got %v, want [warehouse/ingredient]", plan.Orphaned)
	}
}

func TestPlanRebuild_AcceptsBareSummaryName(t *testing.T) {
	reg, subReg := setupRegistry(t)

	plan, err := PlanRebuild(reg, subReg, "stock-level")
	if err != nil {
		t.Fatalf("PlanRebuild(bare): %v", err)
	}
	if plan.Module != "warehouse" || plan.Entity != "stock-level" {
		t.Errorf("resolved %s/%s, want warehouse/stock-level", plan.Module, plan.Entity)
	}
}

func TestPlanRebuild_RejectsNonSummaryEntity(t *testing.T) {
	reg, subReg := setupRegistry(t)

	for _, ref := range []string{"warehouse/ingredient", "warehouse/stock-movement"} {
		plan, err := PlanRebuild(reg, subReg, ref)
		if err == nil {
			t.Fatalf("PlanRebuild(%q): expected rejection, got plan %+v", ref, plan)
		}
		if !strings.Contains(err.Error(), "only summary entities") {
			t.Errorf("PlanRebuild(%q): error %q should explain the summary restriction", ref, err)
		}
	}
}

func TestPlanRebuild_UnknownBareNameListsCandidates(t *testing.T) {
	reg, subReg := setupRegistry(t)

	_, err := PlanRebuild(reg, subReg, "no-such-projection")
	if err == nil {
		t.Fatal("expected error for unknown bare name")
	}
	if !strings.Contains(err.Error(), "warehouse/stock-level") {
		t.Errorf("error %q should list the known summary entities", err)
	}
}

func TestPlanRebuild_RejectsStrategyNone(t *testing.T) {
	reg, subReg := setupRegistry(t)

	_, err := PlanRebuild(reg, subReg, "warehouse/frozen-snapshot")
	if err == nil {
		t.Fatal("expected rejection for rebuild.strategy: none")
	}
	if !strings.Contains(err.Error(), "none") {
		t.Errorf("error %q should name the strategy", err)
	}
}

func TestPlanRebuild_RejectsDeclaredSourcesMissing(t *testing.T) {
	reg, subReg := setupRegistry(t)

	_, err := PlanRebuild(reg, subReg, "warehouse/legacy-summary")
	if err == nil {
		t.Fatal("expected rejection for a summary with no sources")
	}
	if !strings.Contains(err.Error(), "no `sources`") {
		t.Errorf("error %q should say the entity declares no sources", err)
	}
}

// TestPlanRebuild_SourceWithoutSubscriberIsOrphanedNotFatal covers a projection
// that declares a source nothing listens to: the plan must still be produced,
// with the gap named.
func TestPlanRebuild_SourceWithoutSubscriberIsOrphanedNotFatal(t *testing.T) {
	reg, subReg := setupRegistry(t)

	plan, err := PlanRebuild(reg, subReg, "warehouse/warehouse-cost")
	if err != nil {
		t.Fatalf("PlanRebuild: %v", err)
	}
	if len(plan.Streams) != 0 {
		t.Errorf("streams: got %v, want none", plan.Streams)
	}
	if len(plan.Orphaned) != 1 || plan.Orphaned[0] != "warehouse/ingredient" {
		t.Errorf("orphaned: got %v, want [warehouse/ingredient]", plan.Orphaned)
	}
}

// TestReplayStreams_DedupesByEventAndSource keeps the replay input one row per
// (event, source) even when several subscriptions feed the same projection —
// the streaming worker fans out to subscriptions itself.
func TestReplayStreams_DedupesByEventAndSource(t *testing.T) {
	reg, subReg := setupRegistry(t)
	subReg.Add("warehouse", "second-projection", &spec.SubscriptionSpec{
		Events:  []string{"warehouse.stock-movement.on_post"},
		Handler: spec.ImplDecl{Type: spec.ImplScriptRef, Ref: "warehouse/apply-stock-movement-2"},
		Durable: "durable",
	})

	plan, err := PlanRebuild(reg, subReg, "warehouse/stock-level")
	if err != nil {
		t.Fatalf("PlanRebuild: %v", err)
	}
	if len(plan.Streams) != 2 {
		t.Fatalf("streams: got %d, want 2 (one per subscription)", len(plan.Streams))
	}
	if got := plan.ReplayStreams(); len(got) != 1 {
		t.Errorf("ReplayStreams: got %d rows, want 1 after dedupe: %+v", len(got), got)
	}
}
