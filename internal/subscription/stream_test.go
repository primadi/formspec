package subscription

import (
	"context"
	"fmt"
	"testing"

	"github.com/primadi/formspec/internal/action"
	"github.com/primadi/formspec/internal/stream"
	"github.com/primadi/formspec/pkg/spec"
)

// failingExecutor records params and always fails — used to exercise the
// retry / dead-letter path of the streaming worker.
type failingExecutor struct {
	calls []action.ExecuteParams
}

func (e *failingExecutor) Execute(_ context.Context, _ spec.Action, params action.ExecuteParams) (*action.ExecuteResult, error) {
	e.calls = append(e.calls, params)
	return nil, fmt.Errorf("handler failed")
}

// appendEvent writes an event to the stream the way the publisher side
// (Dispatcher.appendToStream) does.
func appendEvent(t *testing.T, s stream.Stream, eventName string, payload map[string]any) {
	t.Helper()
	_, err := s.Append(context.Background(), eventName, map[string]any{
		"workspace_id": "ws-1",
		"resource":     "billing/invoice",
		"event":        eventName,
		"payload":      payload,
		"occurred_at":  "2026-08-25T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDispatcher_DurableAppendsToStream(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.audit-log"},
		Durable: "durable",
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	s := stream.NewMemory()
	d := NewDispatcher(reg, disp)
	d.SetStream(s)

	err := d.Dispatch(context.Background(), "ws-1", "billing.invoice.on_submit", "billing/invoice", map[string]any{"id": "INV-1"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	// Durable → appended to the stream, NOT dispatched directly.
	if len(rec.calls) != 0 {
		t.Fatalf("durable subscription should not dispatch directly, got %d calls", len(rec.calls))
	}
	entries, err := s.Read(context.Background(), "billing.invoice.on_submit", "billing/audit", "w", "earliest", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("stream: want 1 entry, got %d", len(entries))
	}
	if entries[0].Data["workspace_id"] != "ws-1" {
		t.Errorf("workspace_id: want ws-1, got %v", entries[0].Data["workspace_id"])
	}
}

func TestStreamingWorker_ProcessesDurableSubscription(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.audit-log"},
		Durable: "durable",
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	s := stream.NewMemory()
	d := NewDispatcher(reg, disp)
	d.SetStream(s)
	appendEvent(t, s, "billing.invoice.on_submit", map[string]any{"id": "INV-1"})

	w := NewStreamingWorker(reg, s, d)
	w.pollOnce(context.Background())

	if len(rec.calls) != 1 {
		t.Fatalf("handler called %d times, want 1", len(rec.calls))
	}
	if rec.calls[0].Params["id"] != "INV-1" {
		t.Errorf("payload id: want INV-1, got %v", rec.calls[0].Params["id"])
	}
	// The entry must be acked after successful processing.
	entries, _ := s.Read(context.Background(), "billing.invoice.on_submit", "billing/audit", "w", "earliest", 10)
	if len(entries) != 0 {
		t.Fatalf("entry should be acked after processing, got %d pending", len(entries))
	}
}

func TestStreamingWorker_Filter(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.audit-log"},
		Durable: "durable",
		Filter:  "amount > 100",
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	s := stream.NewMemory()
	d := NewDispatcher(reg, disp)
	d.SetStream(s)
	appendEvent(t, s, "billing.invoice.on_submit", map[string]any{"id": "INV-1", "amount": 50})
	appendEvent(t, s, "billing.invoice.on_submit", map[string]any{"id": "INV-2", "amount": 200})

	w := NewStreamingWorker(reg, s, d)
	w.pollOnce(context.Background())

	// Only the amount=200 entry passes the filter.
	if len(rec.calls) != 1 {
		t.Fatalf("handler called %d times, want 1 (filtered)", len(rec.calls))
	}
	if rec.calls[0].Params["id"] != "INV-2" {
		t.Errorf("handler should receive INV-2, got %v", rec.calls[0].Params["id"])
	}
	// Both entries acked (filtered-out entries are skipped, not retried).
	entries, _ := s.Read(context.Background(), "billing.invoice.on_submit", "billing/audit", "w", "earliest", 10)
	if len(entries) != 0 {
		t.Fatalf("all entries should be acked, got %d pending", len(entries))
	}
}

func TestStreamingWorker_Transform(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:  []string{"billing.invoice.on_submit"},
		Handler: spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.audit-log"},
		Durable: "durable",
		// Replace the payload with a derived dict.
		Transform: `{"total": amount * 2, "currency": "IDR"}`,
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	s := stream.NewMemory()
	d := NewDispatcher(reg, disp)
	d.SetStream(s)
	appendEvent(t, s, "billing.invoice.on_submit", map[string]any{"id": "INV-1", "amount": 100})

	w := NewStreamingWorker(reg, s, d)
	w.pollOnce(context.Background())

	if len(rec.calls) != 1 {
		t.Fatalf("handler called %d times, want 1", len(rec.calls))
	}
	if rec.calls[0].Params["total"] != int64(200) {
		t.Errorf("transformed total: want 200, got %v", rec.calls[0].Params["total"])
	}
	if rec.calls[0].Params["currency"] != "IDR" {
		t.Errorf("transformed currency: want IDR, got %v", rec.calls[0].Params["currency"])
	}
}

func TestStreamingWorker_RetryThenDeadLetter(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:   []string{"billing.invoice.on_submit"},
		Handler:  spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.audit-log"},
		Durable:  "durable",
		MaxRetry: 2,
	})

	disp := action.NewDispatcher()
	fail := &failingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, fail)

	s := stream.NewMemory()
	d := NewDispatcher(reg, disp)
	d.SetStream(s)
	appendEvent(t, s, "billing.invoice.on_submit", map[string]any{"id": "INV-1"})

	w := NewStreamingWorker(reg, s, d)
	// Attempt 1: fails, attempts=1 < maxRetry=2 → stays pending.
	w.pollOnce(context.Background())
	// Attempt 2: fails, attempts=2 >= maxRetry=2 → dead-letter + ack.
	w.pollOnce(context.Background())
	// Nothing left.
	w.pollOnce(context.Background())

	if len(fail.calls) != 2 {
		t.Fatalf("handler called %d times, want 2 (retry then dead-letter)", len(fail.calls))
	}
	// Dead-letter stream holds the failed entry.
	dead, err := s.Read(context.Background(), "billing.invoice.on_submit.dead", "billing/audit", "w", "earliest", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dead) != 1 {
		t.Fatalf("dead-letter stream: want 1 entry, got %d", len(dead))
	}
	if dead[0].Data["error"] == "" {
		t.Error("dead-letter entry should carry the failure reason")
	}
	// Original stream is drained.
	entries, _ := s.Read(context.Background(), "billing.invoice.on_submit", "billing/audit", "w", "earliest", 10)
	if len(entries) != 0 {
		t.Fatalf("original stream should be drained, got %d pending", len(entries))
	}
}

func TestStreamingWorker_RetentionTrim(t *testing.T) {
	reg := NewRegistry()
	reg.Add("billing", "audit", &spec.SubscriptionSpec{
		Events:    []string{"billing.invoice.on_submit"},
		Handler:   spec.ImplDecl{Type: spec.ImplNative, Ref: "billing.audit-log"},
		Durable:   "durable",
		Retention: "2",
	})

	disp := action.NewDispatcher()
	rec := &recordingExecutor{}
	disp.RegisterExecutor(spec.ImplNative, rec)

	s := stream.NewMemory()
	d := NewDispatcher(reg, disp)
	d.SetStream(s)
	for i := 0; i < 5; i++ {
		appendEvent(t, s, "billing.invoice.on_submit", map[string]any{"id": fmt.Sprintf("INV-%d", i)})
	}

	w := NewStreamingWorker(reg, s, d)
	w.pollOnce(context.Background())

	// All 5 processed and acked; retention trims the stream to the last 2.
	if len(rec.calls) != 5 {
		t.Fatalf("handler called %d times, want 5", len(rec.calls))
	}
	entries, _ := s.Read(context.Background(), "billing.invoice.on_submit", "billing/audit", "w", "earliest", 10)
	if len(entries) != 0 {
		t.Fatalf("all entries should be acked, got %d pending", len(entries))
	}
}
