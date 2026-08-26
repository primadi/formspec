package job

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/internal/events"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// recordingHub captures broadcasts for assertions.
type recordingHub struct {
	msgs []events.EventMessage
}

func (h *recordingHub) Broadcast(workspaceID string, msg events.EventMessage) {
	h.msgs = append(h.msgs, msg)
}
func (h *recordingHub) HasListeners(string) bool { return true }

func newTestTracker(t *testing.T) (*Tracker, *recordingHub, *db.JobStore) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "job.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	r := db.NewMigrationRunner(d, db.DriverSQLite)
	if err := r.EnsureSystemTables(context.Background()); err != nil {
		t.Fatalf("EnsureSystemTables: %v", err)
	}
	store := db.NewJobStore(d, db.DriverSQLite)
	hub := &recordingHub{}
	tr := NewTracker(store, hub, "test-secret")
	return tr, hub, store
}

func TestTracker_CreateProgressComplete(t *testing.T) {
	tr, hub, store := newTestTracker(t)
	ctx := context.Background()

	row, err := tr.Create(ctx, "ws-1", "demo", "report", "generate", "")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "pending" {
		t.Errorf("status: want pending, got %q", row.Status)
	}

	if err := tr.Start(ctx, row.ID); err != nil {
		t.Fatal(err)
	}
	if err := tr.Progress(ctx, "ws-1", row.ID, 40, "processing batch 2/5"); err != nil {
		t.Fatal(err)
	}
	if err := tr.Complete(ctx, "ws-1", row.ID, map[string]any{"rows": 100}); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "completed" || got.Progress != 100 {
		t.Errorf("job: want completed/100, got %s/%d", got.Status, got.Progress)
	}
	if got.Result["rows"] != float64(100) {
		t.Errorf("result: want rows=100, got %v", got.Result)
	}

	// Hub received progress + completed events on the jobs channel.
	if len(hub.msgs) != 2 {
		t.Fatalf("hub events: want 2, got %d", len(hub.msgs))
	}
	if hub.msgs[0].Event != "progress" || hub.msgs[0].Resource != "jobs" {
		t.Errorf("event[0]: want progress/jobs, got %s/%s", hub.msgs[0].Event, hub.msgs[0].Resource)
	}
	if hub.msgs[0].Payload["progress"] != 40 {
		t.Errorf("event[0] progress: want 40, got %v", hub.msgs[0].Payload["progress"])
	}
	if hub.msgs[1].Event != "completed" {
		t.Errorf("event[1]: want completed, got %s", hub.msgs[1].Event)
	}
}

func TestTracker_Fail(t *testing.T) {
	tr, hub, store := newTestTracker(t)
	ctx := context.Background()

	row, err := tr.Create(ctx, "ws-1", "demo", "report", "generate", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Fail(ctx, "ws-1", row.ID, "boom"); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "failed" || got.Error != "boom" {
		t.Errorf("job: want failed/boom, got %s/%s", got.Status, got.Error)
	}
	if len(hub.msgs) != 1 || hub.msgs[0].Event != "failed" {
		t.Fatalf("hub: want 1 failed event, got %d (%v)", len(hub.msgs), hub.msgs)
	}
}

func TestTracker_Get(t *testing.T) {
	tr, _, _ := newTestTracker(t)
	ctx := context.Background()

	row, err := tr.Create(ctx, "ws-1", "demo", "report", "generate", "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.Get(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != row.ID {
		t.Fatalf("Get: want job %s, got %v", row.ID, got)
	}
	missing, err := tr.Get(ctx, "999999")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("Get missing: want nil, got %v", missing)
	}
}
