package subscription

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/primadi/formspec/internal/starlark"
	"github.com/primadi/formspec/internal/stream"
)

// StreamingWorker consumes durable (Tier 2) subscriptions from a stream
// backend (todo 7.3.2, docs/spec/backend/02-core-extended.md §3).
//
// For each durable subscription it reads claimed entries from the
// subscription's stream (named by the fully-qualified event), applies
// filter/transform Starlark over the event payload, dispatches to the
// handler, and acks. A failed entry is left pending (at-least-once
// redelivery) until it reaches max_retry, then it is dead-lettered to a
// "{stream}.dead" stream and acked so it stops looping.
//
// The worker talks only to the stream.Stream abstraction — never to a
// backend (Redis/Kafka/in-memory) directly.
type StreamingWorker struct {
	reg        *Registry
	stream     stream.Stream
	dispatcher *Dispatcher
	batchSize  int
	interval   time.Duration
	now        func() time.Time

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	running bool
}

// StreamingWorkerOption configures the worker.
type StreamingWorkerOption func(*StreamingWorker)

// WithStreamInterval sets the poll interval (default 500ms).
func WithStreamInterval(d time.Duration) StreamingWorkerOption {
	return func(w *StreamingWorker) { w.interval = d }
}

// WithStreamBatchSize sets the read batch size per stream (default 10).
func WithStreamBatchSize(n int) StreamingWorkerOption {
	return func(w *StreamingWorker) { w.batchSize = n }
}

// NewStreamingWorker creates a streaming worker for durable subscriptions.
func NewStreamingWorker(reg *Registry, s stream.Stream, dispatcher *Dispatcher, opts ...StreamingWorkerOption) *StreamingWorker {
	w := &StreamingWorker{
		reg:        reg,
		stream:     s,
		dispatcher: dispatcher,
		batchSize:  10,
		interval:   500 * time.Millisecond,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Start begins the background streaming loop.
func (w *StreamingWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		return
	}
	w.ctx, w.cancel = context.WithCancel(ctx)
	w.running = true
	w.wg.Add(1)
	go w.runLoop()
	log.Printf("[subscription-stream] started (poll=%v, batch=%d)", w.interval, w.batchSize)
}

// Stop signals the worker to shut down and waits for completion.
func (w *StreamingWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.running = false
	w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	log.Printf("[subscription-stream] stopped")
}

// IsRunning reports whether the worker is running.
func (w *StreamingWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// runLoop polls durable subscriptions on the configured interval.
func (w *StreamingWorker) runLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.pollOnce(w.ctx)
		}
	}
}

// pollOnce processes every durable subscription's streams once.
func (w *StreamingWorker) pollOnce(ctx context.Context) {
	for _, sub := range w.reg.Durable() {
		for _, ev := range sub.Spec.Events {
			w.processStream(ctx, sub, ev)
		}
	}
}

// processStream reads and processes one (subscription, event) stream.
func (w *StreamingWorker) processStream(ctx context.Context, sub DurableSub, eventName string) {
	streamName := stream.NormalizeStreamName(eventName)
	group := sub.Module + "/" + sub.Name
	consumer := "formspec-worker"

	maxRetry := sub.Spec.MaxRetry
	if maxRetry <= 0 {
		maxRetry = 3
	}

	entries, err := w.stream.Read(ctx, streamName, group, consumer, sub.Spec.Position, w.batchSize)
	if err != nil {
		log.Printf("[subscription-stream] read %s: %v", streamName, err)
		return
	}
	for _, e := range entries {
		w.processEntry(ctx, sub, eventName, streamName, group, e, maxRetry)
	}

	// Enforce retention after processing (best-effort).
	if sub.Spec.Retention != "" {
		if err := w.stream.Trim(ctx, streamName, sub.Spec.Retention); err != nil {
			log.Printf("[subscription-stream] trim %s: %v", streamName, err)
		}
	}
}

// processEntry applies filter/transform and dispatches one stream entry.
func (w *StreamingWorker) processEntry(ctx context.Context, sub DurableSub, eventName, streamName, group string, e stream.Entry, maxRetry int) {
	workspaceID, _ := e.Data["workspace_id"].(string)
	resource, _ := e.Data["resource"].(string)
	occurredAt, _ := e.Data["occurred_at"].(string)
	payload, _ := e.Data["payload"].(map[string]any)
	if payload == nil {
		payload = map[string]any{}
	}

	env := eventEnv(eventName, resource, workspaceID, occurredAt, payload)

	// filter — skip (and ack) entries the subscription doesn't care about.
	if sub.Spec.Filter != "" {
		ok, _, err := starlark.EvaluateGuard(sub.Spec.Filter, env)
		if err != nil {
			log.Printf("[subscription-stream] %s/%s filter error on %s: %v (skipping)", sub.Module, sub.Name, e.ID, err)
			_ = w.stream.Ack(ctx, streamName, group, e.ID)
			return
		}
		if !ok {
			_ = w.stream.Ack(ctx, streamName, group, e.ID)
			return
		}
	}

	// transform — replace the payload with the expression's result.
	if sub.Spec.Transform != "" {
		result, err := starlark.EvalExpr(sub.Spec.Transform, env)
		if err != nil {
			log.Printf("[subscription-stream] %s/%s transform error on %s: %v (skipping)", sub.Module, sub.Name, e.ID, err)
			_ = w.stream.Ack(ctx, streamName, group, e.ID)
			return
		}
		if m, ok := result.(map[string]any); ok {
			payload = m
		}
	}

	// dispatch to the handler (same path as Tier 1).
	if err := w.dispatcher.dispatchOne(ctx, workspaceID, eventName, resource, payload, sub.Spec); err != nil {
		if e.Attempts >= maxRetry {
			w.deadLetter(ctx, sub, eventName, streamName, group, e, err)
			_ = w.stream.Ack(ctx, streamName, group, e.ID)
		} else {
			log.Printf("[subscription-stream] %s/%s entry %s attempt %d failed (will retry): %v", sub.Module, sub.Name, e.ID, e.Attempts, err)
		}
		return
	}
	_ = w.stream.Ack(ctx, streamName, group, e.ID)
}

// deadLetter appends a failed entry to the "{stream}.dead" stream and logs.
func (w *StreamingWorker) deadLetter(ctx context.Context, sub DurableSub, eventName, streamName, group string, e stream.Entry, cause error) {
	deadStream := streamName + ".dead"
	data := map[string]any{
		"original_id":  e.ID,
		"event":        eventName,
		"subscription": sub.Module + "/" + sub.Name,
		"payload":      e.Data,
		"error":        cause.Error(),
		"attempts":     e.Attempts,
		"dead_at":      w.now().UTC().Format(time.RFC3339Nano),
	}
	if _, err := w.stream.Append(ctx, deadStream, data); err != nil {
		log.Printf("[subscription-stream] dead-letter %s: %v", deadStream, err)
	}
	log.Printf("[subscription-stream] dead-lettered %s/%s entry %s after %d attempts: %v", sub.Module, sub.Name, e.ID, e.Attempts, cause)
}

// eventEnv builds the Starlark environment for filter/transform expressions:
// `event.name`, `event.resource`, `event.workspace_id`, `event.occurred_at`,
// plus the event payload fields directly (02-core-extended.md §3).
func eventEnv(eventName, resource, workspaceID, occurredAt string, payload map[string]any) map[string]any {
	env := make(map[string]any, len(payload)+1)
	env["event"] = map[string]any{
		"name":         eventName,
		"resource":     resource,
		"workspace_id": workspaceID,
		"occurred_at":  occurredAt,
	}
	for k, v := range payload {
		env[k] = v
	}
	return env
}
