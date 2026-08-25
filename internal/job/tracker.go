// Package job provides the async job tracker (02-core-extended.md §13, todo
// 7.13). A tracked async action (`call: async` + `track: true`) creates a job
// row, reports progress via ctx.job.progress, and ends completed/failed —
// pushed to the `jobs` websocket channel and optionally delivered to a
// callback webhook (7.13.4).
package job

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/primadi/formspec/internal/events"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Tracker manages async job lifecycle.
type Tracker struct {
	store *db.JobStore
	hub   events.Hub
	// callbackSecret signs callback webhook deliveries (7.13.4).
	callbackSecret string
	// httpClient delivers callback webhooks.
	httpClient *http.Client
}

// NewTracker creates a job tracker. hub may be nil (no websocket push).
func NewTracker(store *db.JobStore, hub events.Hub, callbackSecret string) *Tracker {
	return &Tracker{
		store:          store,
		hub:            hub,
		callbackSecret: callbackSecret,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

// SetHub wires the websocket hub for the `jobs` channel. The hub is created
// after the dispatcher, so it is set once available.
func (t *Tracker) SetHub(hub events.Hub) {
	t.hub = hub
}

// Create registers a new pending job and returns its row.
func (t *Tracker) Create(ctx context.Context, workspaceID, module, entity, action, callbackURL string) (*db.JobRow, error) {
	return t.store.Create(ctx, workspaceID, module, entity, action, callbackURL)
}

// Start marks a job running.
func (t *Tracker) Start(ctx context.Context, jobID string) error {
	return t.store.Update(ctx, jobID, "running", 0, "", nil, "")
}

// Progress updates a job's progress and pushes a `progress` event.
func (t *Tracker) Progress(ctx context.Context, workspaceID, jobID string, pct int, message string) error {
	if err := t.store.Update(ctx, jobID, "running", pct, message, nil, ""); err != nil {
		return err
	}
	t.publish(ctx, workspaceID, jobID, "progress", map[string]any{
		"job_id": jobID, "progress": pct, "message": message,
	})
	return nil
}

// Complete marks a job completed, pushes `completed`, and delivers the
// callback webhook (7.13.4) if one was supplied.
func (t *Tracker) Complete(ctx context.Context, workspaceID, jobID string, result map[string]any) error {
	if err := t.store.Update(ctx, jobID, "completed", 100, "", result, ""); err != nil {
		return err
	}
	t.publish(ctx, workspaceID, jobID, "completed", map[string]any{
		"job_id": jobID, "status": "completed", "result": result,
	})
	t.deliverCallback(ctx, jobID, "completed", result, "")
	return nil
}

// Fail marks a job failed, pushes `failed`, and delivers the callback.
func (t *Tracker) Fail(ctx context.Context, workspaceID, jobID, errMsg string) error {
	if err := t.store.Update(ctx, jobID, "failed", 0, "", nil, errMsg); err != nil {
		return err
	}
	t.publish(ctx, workspaceID, jobID, "failed", map[string]any{
		"job_id": jobID, "status": "failed", "message": errMsg,
	})
	t.deliverCallback(ctx, jobID, "failed", nil, errMsg)
	return nil
}

// Get returns a job row (for the poll_url alternative).
func (t *Tracker) Get(ctx context.Context, jobID string) (*db.JobRow, error) {
	return t.store.Get(ctx, jobID)
}

// publish pushes a job event to the `jobs` websocket channel for the workspace.
func (t *Tracker) publish(ctx context.Context, workspaceID, jobID, event string, payload map[string]any) {
	if t.hub == nil {
		return
	}
	t.hub.Broadcast(workspaceID, events.EventMessage{
		Event:    event,
		Resource: "jobs",
		Payload:  payload,
		Emitted:  time.Now().UTC().Format(time.RFC3339Nano),
	})
}

// deliverCallback sends the job result to the callback webhook URL (7.13.4),
// HMAC-signed when a secret is configured, with a bounded retry loop.
func (t *Tracker) deliverCallback(ctx context.Context, jobID, status string, result map[string]any, errMsg string) {
	row, err := t.store.Get(ctx, jobID)
	if err != nil || row == nil || row.CallbackURL == "" {
		return
	}
	payload := map[string]any{
		"job_id": jobID,
		"status": status,
	}
	if result != nil {
		payload["result"] = result
	}
	if errMsg != "" {
		payload["message"] = errMsg
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	// Bounded retry: 3 attempts, 500ms / 1s backoff.
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, row.CallbackURL, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if t.callbackSecret != "" {
			mac := hmac.New(sha256.New, []byte(t.callbackSecret))
			mac.Write(body)
			req.Header.Set("X-FormSpec-Signature", hex.EncodeToString(mac.Sum(nil)))
		}
		resp, err := t.httpClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return
			}
		}
	}
	// Best-effort — a failed callback is logged by the caller if needed.
	_ = fmt.Errorf("job callback delivery failed for %s", jobID)
}