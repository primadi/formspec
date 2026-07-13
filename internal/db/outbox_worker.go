package db

import (
	"context"
	"log"
	"sync"
	"time"
)

// EventHandler processes a single outbox event.
// Implementations should be idempotent and handle their own retries.
type EventHandler interface {
	// HandleEvent delivers an event from the outbox.
	// Returns an error if delivery fails permanently (worker will mark as failed).
	HandleEvent(ctx context.Context, workspaceID, eventName, resource, payload string) error
}

// EventHandlerFunc is an adapter that allows a plain function to be used as
// an EventHandler.
type EventHandlerFunc func(ctx context.Context, workspaceID, eventName, resource, payload string) error

// HandleEvent implements EventHandler by calling f.
func (f EventHandlerFunc) HandleEvent(ctx context.Context, workspaceID, eventName, resource, payload string) error {
	return f(ctx, workspaceID, eventName, resource, payload)
}

// OutboxWorkerConfig configures the outbox background worker.
type OutboxWorkerConfig struct {
	PollInterval    time.Duration // How often to poll for new events (default 1s)
	BatchSize       int           // Max events to fetch per poll (default 10)
	MaxRetries      int           // Max delivery retries before failing permanently (default 5)
	CleanupAge      time.Duration // Age after which completed events are deleted (default 24h)
	CleanupInterval time.Duration // How often to run cleanup (default 1h)
}

// OutboxWorker polls the outbox table and delivers events via the EventHandler.
//
// It runs a background goroutine that:
//  1. Polls for pending events on a configurable interval
//  2. Delivers each event via the registered EventHandler
//  3. Marks events completed/failed based on delivery outcome
//  4. Periodically cleans up old completed events
//
// Start creates a cancellable background loop. Stop signals shutdown.
type OutboxWorker struct {
	store   *OutboxStore
	handler EventHandler
	config  OutboxWorkerConfig

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.RWMutex
	running bool
}

// OutboxWorkerOption configures an OutboxWorker.
type OutboxWorkerOption func(*OutboxWorkerConfig)

// WithPollInterval sets how often to poll for new events.
func WithPollInterval(d time.Duration) OutboxWorkerOption {
	return func(c *OutboxWorkerConfig) { c.PollInterval = d }
}

// WithBatchSize sets the max events to claim per poll cycle.
func WithBatchSize(n int) OutboxWorkerOption {
	return func(c *OutboxWorkerConfig) { c.BatchSize = n }
}

// WithMaxRetries sets the max delivery attempts before permanent failure.
func WithMaxRetries(n int) OutboxWorkerOption {
	return func(c *OutboxWorkerConfig) { c.MaxRetries = n }
}

// WithCleanupAge sets the age threshold for deleting completed events.
func WithCleanupAge(d time.Duration) OutboxWorkerOption {
	return func(c *OutboxWorkerConfig) { c.CleanupAge = d }
}

// NewOutboxWorker creates a new outbox worker with the given handler.
// The worker must be started with Start().
func NewOutboxWorker(store *OutboxStore, handler EventHandler, opts ...OutboxWorkerOption) *OutboxWorker {
	cfg := OutboxWorkerConfig{
		PollInterval:    1 * time.Second,
		BatchSize:       10,
		MaxRetries:      5,
		CleanupAge:      24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return &OutboxWorker{
		store:   store,
		handler: handler,
		config:  cfg,
	}
}

// Start begins the background polling loop. It returns immediately.
// The worker runs until Stop() is called or the context is cancelled.
func (w *OutboxWorker) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.running = true

	w.wg.Add(1)
	go w.runLoop()

	log.Printf("[outbox-worker] started (poll=%v, batch=%d, maxRetries=%d)",
		w.config.PollInterval, w.config.BatchSize, w.config.MaxRetries)
}

// Stop signals the worker to shut down and waits for completion.
func (w *OutboxWorker) Stop() {
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
	log.Printf("[outbox-worker] stopped")
}

// IsRunning returns whether the worker is currently running.
func (w *OutboxWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// runLoop is the main goroutine: poll → deliver → sleep → repeat.
func (w *OutboxWorker) runLoop() {
	defer w.wg.Done()

	pollTicker := time.NewTicker(w.config.PollInterval)
	defer pollTicker.Stop()

	cleanupTicker := time.NewTicker(w.config.CleanupInterval)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return

		case <-pollTicker.C:
			w.processBatch(w.ctx)

		case <-cleanupTicker.C:
			w.runCleanup(w.ctx)
		}
	}
}

// processBatch claims and delivers a batch of pending events.
func (w *OutboxWorker) processBatch(ctx context.Context) {
	records, err := w.store.Dequeue(ctx, w.config.BatchSize)
	if err != nil {
		log.Printf("[outbox-worker] dequeue error: %v", err)
		return
	}

	for _, rec := range records {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := w.handler.HandleEvent(ctx, rec.WorkspaceID, rec.EventName, rec.Resource, rec.Payload)
		if err != nil {
			log.Printf("[outbox-worker] deliver failed event=%s id=%s: %v", rec.EventName, rec.ID, err)
			if markErr := w.store.MarkFailed(ctx, rec.ID, w.config.MaxRetries); markErr != nil {
				log.Printf("[outbox-worker] mark failed event=%s id=%s: %v", rec.EventName, rec.ID, markErr)
			}
		} else {
			if markErr := w.store.MarkCompleted(ctx, rec.ID); markErr != nil {
				log.Printf("[outbox-worker] mark completed event=%s id=%s: %v", rec.EventName, rec.ID, markErr)
			}
		}
	}
}

// runCleanup deletes old completed events.
func (w *OutboxWorker) runCleanup(ctx context.Context) {
	deleted, err := w.store.Cleanup(ctx, w.config.CleanupAge)
	if err != nil {
		log.Printf("[outbox-worker] cleanup error: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("[outbox-worker] cleaned %d completed events", deleted)
	}
}
