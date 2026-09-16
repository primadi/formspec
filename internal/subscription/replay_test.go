package subscription

import (
	"context"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/stream"
	"github.com/primadi/formspec/pkg/spec"
)

// appendWarehouseEvent writes an event the way the publisher side
// (Dispatcher.appendToStream) does, for the summary-replay tests.
func appendWarehouseEvent(t *testing.T, s stream.Stream, eventName, resource string, payload map[string]any) {
	t.Helper()
	_, err := s.Append(context.Background(), eventName, map[string]any{
		"workspace_id": "ws-1",
		"resource":     resource,
		"event":        eventName,
		"payload":      payload,
		"occurred_at":  "2026-09-15T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
}

// newReplayFixture wires a durable subscription over a memory stream, with an
// optional filter/transform, and returns the worker plus the recording
// executor that stands in for the projection-maintaining handler.
func newReplayFixture(t *testing.T, filter, transform string) (*StreamingWorker, stream.Stream, *recordingExecutor) {
	t.Helper()
	reg := NewRegistry()
	reg.Add("warehouse", "stock-projection", &spec.SubscriptionSpec{
		Events:    []string{"warehouse.stock-movement.on_post"},
		Handler:   spec.ImplDecl{Type: spec.ImplNative, Ref: "warehouse.apply-stock-movement"},
		Durable:   "durable",
		Filter:    filter,
		Transform: transform,
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	s := stream.NewMemory()
	d := NewDispatcher(reg, disp)
	d.SetStream(s)

	return NewStreamingWorker(reg, s, d), s, rec
}

func TestReplaySummaryProjection_ReplaysFromEarliest(t *testing.T) {
	w, s, rec := newReplayFixture(t, "", "")

	appendWarehouseEvent(t, s, "warehouse.stock-movement.on_post", "warehouse/stock-movement", map[string]any{"id": "M1", "quantity": 5})
	appendWarehouseEvent(t, s, "warehouse.stock-movement.on_post", "warehouse/stock-movement", map[string]any{"id": "M2", "quantity": -3})
	// A second workspace must be skipped when the run is scoped to one.
	_, err := s.Append(context.Background(), "warehouse.stock-movement.on_post", map[string]any{
		"workspace_id": "ws-2",
		"resource":     "warehouse/stock-movement",
		"event":        "warehouse.stock-movement.on_post",
		"payload":      map[string]any{"id": "M3", "quantity": 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	res := w.ReplaySummaryProjection(context.Background(), []ReplayStream{
		{EventName: "warehouse.stock-movement.on_post", Resource: "warehouse/stock-movement"},
	}, ReplayOptions{WorkspaceID: "ws-1", RunID: "test-run-1"})

	if res.Read != 3 {
		t.Errorf("read: got %d, want 3 (every entry is seen, even the skipped ones)", res.Read)
	}
	if res.Dispatched != 2 {
		t.Errorf("dispatched: got %d, want 2", res.Dispatched)
	}
	if res.Skipped != 1 {
		t.Errorf("skipped: got %d, want 1 (other workspace)", res.Skipped)
	}
	if res.Failed != 0 {
		t.Errorf("failed: got %d, want 0", res.Failed)
	}
	if res.Incomplete() {
		t.Error("Incomplete: want false for a clean run")
	}
	if len(rec.calls) != 2 {
		t.Fatalf("handler calls: got %d, want 2", len(rec.calls))
	}
	if rec.calls[0].Params["id"] != "M1" {
		t.Errorf("replay order: first call got %v, want the earliest entry M1", rec.calls[0].Params["id"])
	}
}

// TestReplaySummaryProjection_LeavesLiveGroupIntact is the property that makes
// a rebuild safe to run against a live server: replay uses its own consumer
// group, so the streaming worker's cursor and pending entries are untouched.
func TestReplaySummaryProjection_LeavesLiveGroupIntact(t *testing.T) {
	w, s, _ := newReplayFixture(t, "", "")
	appendWarehouseEvent(t, s, "warehouse.stock-movement.on_post", "warehouse/stock-movement", map[string]any{"id": "M1"})

	res := w.ReplaySummaryProjection(context.Background(), []ReplayStream{
		{EventName: "warehouse.stock-movement.on_post", Resource: "warehouse/stock-movement"},
	}, ReplayOptions{RunID: "test-run-2"})
	if res.Dispatched != 1 {
		t.Fatalf("dispatched: got %d, want 1", res.Dispatched)
	}

	// The live group ("warehouse/stock-projection") still has the entry pending
	// — replay acked it only in its own disposable group.
	entries, err := s.Read(context.Background(), "warehouse.stock-movement.on_post", "warehouse/stock-projection", "live", "earliest", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("live group: got %d pending entries, want 1 — replay must not consume the live cursor", len(entries))
	}
}

// TestReplaySummaryProjection_RerunReplaysAgain guards the RunID contract: a
// second rebuild must not resume from the first run's cursor, or the projection
// would silently never be rebuilt twice.
func TestReplaySummaryProjection_RerunReplaysAgain(t *testing.T) {
	w, s, rec := newReplayFixture(t, "", "")
	appendWarehouseEvent(t, s, "warehouse.stock-movement.on_post", "warehouse/stock-movement", map[string]any{"id": "M1"})

	mk := func(runID string) ReplayResult {
		return w.ReplaySummaryProjection(context.Background(), []ReplayStream{
			{EventName: "warehouse.stock-movement.on_post", Resource: "warehouse/stock-movement"},
		}, ReplayOptions{RunID: runID})
	}

	if res := mk("run-a"); res.Dispatched != 1 {
		t.Fatalf("first run dispatched %d, want 1", res.Dispatched)
	}
	if res := mk("run-b"); res.Dispatched != 1 {
		t.Fatalf("second run dispatched %d, want 1 (a fresh group replays from earliest)", res.Dispatched)
	}
	if len(rec.calls) != 2 {
		t.Errorf("handler calls: got %d, want 2", len(rec.calls))
	}
}

// TestReplaySummaryProjection_AppliesFilterAndTransform pins that a rebuild
// goes through the same filter → transform path as live delivery — otherwise a
// rebuilt projection would differ from one built by normal operation.
func TestReplaySummaryProjection_AppliesFilterAndTransform(t *testing.T) {
	w, s, rec := newReplayFixture(t, "quantity > 0", `{"id": id, "replayed": True}`)

	appendWarehouseEvent(t, s, "warehouse.stock-movement.on_post", "warehouse/stock-movement", map[string]any{"id": "M1", "quantity": 5})
	appendWarehouseEvent(t, s, "warehouse.stock-movement.on_post", "warehouse/stock-movement", map[string]any{"id": "M2", "quantity": -1})

	res := w.ReplaySummaryProjection(context.Background(), []ReplayStream{
		{EventName: "warehouse.stock-movement.on_post", Resource: "warehouse/stock-movement"},
	}, ReplayOptions{RunID: "test-run-3"})

	if res.Dispatched != 1 {
		t.Errorf("dispatched: got %d, want 1 (filter drops quantity <= 0)", res.Dispatched)
	}
	if res.Skipped != 1 {
		t.Errorf("skipped: got %d, want 1", res.Skipped)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("handler calls: got %d, want 1", len(rec.calls))
	}
	if rec.calls[0].Params["replayed"] != true {
		t.Errorf("transform not applied: params %+v", rec.calls[0].Params)
	}
}

// TestReplaySummaryProjection_CountsHandlerFailure keeps a broken handler from
// looking like a successful rebuild.
func TestReplaySummaryProjection_CountsHandlerFailure(t *testing.T) {
	reg := NewRegistry()
	reg.Add("warehouse", "stock-projection", &spec.SubscriptionSpec{
		Events:  []string{"warehouse.stock-movement.on_post"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "warehouse.apply-stock-movement"},
		Durable: "durable",
	})
	disp := action.NewDispatcher()
	disp.RegisterExecutor(spec.ImplNative, &failingExecutor{})

	s := stream.NewMemory()
	d := NewDispatcher(reg, disp)
	d.SetStream(s)
	w := NewStreamingWorker(reg, s, d)

	appendWarehouseEvent(t, s, "warehouse.stock-movement.on_post", "warehouse/stock-movement", map[string]any{"id": "M1"})

	res := w.ReplaySummaryProjection(context.Background(), []ReplayStream{
		{EventName: "warehouse.stock-movement.on_post", Resource: "warehouse/stock-movement"},
	}, ReplayOptions{RunID: "test-run-4"})

	if res.Failed != 1 || res.Dispatched != 0 {
		t.Errorf("got failed=%d dispatched=%d, want failed=1 dispatched=0", res.Failed, res.Dispatched)
	}
	if !res.Incomplete() {
		t.Error("Incomplete: want true when a handler failed")
	}
	// The run must terminate even though the handler failed (the entry is acked
	// in the replay group, so the next read does not return it forever).
	if res.Read != 1 {
		t.Errorf("read: got %d, want 1 — a failure must not loop", res.Read)
	}
}

// TestReplaySummaryProjection_SubscriberFilterNarrowsRun covers --subscriber.
func TestReplaySummaryProjection_SubscriberFilterNarrowsRun(t *testing.T) {
	reg := NewRegistry()
	reg.Add("warehouse", "wanted", &spec.SubscriptionSpec{
		Events:  []string{"warehouse.stock-movement.on_post"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "warehouse.a"},
		Durable: "durable",
	})
	reg.Add("warehouse", "unwanted", &spec.SubscriptionSpec{
		Events:  []string{"warehouse.stock-movement.on_post"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "warehouse.b"},
		Durable: "durable",
	})
	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	s := stream.NewMemory()
	d := NewDispatcher(reg, disp)
	d.SetStream(s)
	w := NewStreamingWorker(reg, s, d)

	appendWarehouseEvent(t, s, "warehouse.stock-movement.on_post", "warehouse/stock-movement", map[string]any{"id": "M1"})

	res := w.ReplaySummaryProjection(context.Background(), []ReplayStream{
		{EventName: "warehouse.stock-movement.on_post", Resource: "warehouse/stock-movement"},
	}, ReplayOptions{RunID: "test-run-5", Subscribers: []string{"warehouse/wanted"}})

	if len(res.Streams) != 1 {
		t.Fatalf("streams: got %d, want 1 (%+v)", len(res.Streams), res.Streams)
	}
	if res.Streams[0].Subscription != "warehouse/wanted" {
		t.Errorf("replayed %q, want warehouse/wanted", res.Streams[0].Subscription)
	}
	if len(rec.calls) != 1 {
		t.Errorf("handler calls: got %d, want 1", len(rec.calls))
	}
}

// TestReplaySummaryProjection_EmptyStreamIsNotAnError: a projection whose stream
// has nothing to replay yields a zero result, not a failure — the CLI decides
// how to report "no history".
func TestReplaySummaryProjection_EmptyStreamIsNotAnError(t *testing.T) {
	w, _, _ := newReplayFixture(t, "", "")

	res := w.ReplaySummaryProjection(context.Background(), []ReplayStream{
		{EventName: "warehouse.stock-movement.on_post", Resource: "warehouse/stock-movement"},
	}, ReplayOptions{RunID: "test-run-6"})

	if res.Read != 0 || res.Dispatched != 0 {
		t.Errorf("got read=%d dispatched=%d, want zeros", res.Read, res.Dispatched)
	}
	if res.Incomplete() {
		t.Error("Incomplete: want false for an empty stream")
	}
}
