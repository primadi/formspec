package api

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/events"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonbpersist"
)

// setupWSPermissionRegistry registers one exposed entity (clinic/visit,
// plural visits) so WSHub.Broadcast can resolve EventMessage.Resource
// "clinic/visit" to the {module}.{plural}.view permission (2.6.6).
func setupWSPermissionRegistry(t *testing.T) *entity.Registry {
	t.Helper()
	dir := t.TempDir()
	d, err := db.OpenSQLite(filepath.Join(dir, "wshub_perm.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	reg := entity.NewRegistry(d, db.DriverSQLite, dir)
	visitSpec := spec.EntitySpec{
		Version: "v1",
		Plural:  "visits",
		Fields:  []spec.Field{{Name: "status", Type: spec.FieldString}},
	}
	registerTestEntity(t, d, reg, "clinic", "visit", visitSpec)
	return reg
}

func expectMessage(t *testing.T, c *wsConn, want bool) {
	t.Helper()
	select {
	case <-c.send:
		if !want {
			t.Fatalf("connection %s unexpectedly received the broadcast", c.id)
		}
	case <-time.After(150 * time.Millisecond):
		if want {
			t.Fatalf("connection %s did not receive the broadcast", c.id)
		}
	}
}

// TestWSHub_Broadcast_FiltersByPermission verifies 2.6.6: a connection whose
// identity lacks {module}.{plural}.view for the event's resource never
// receives it, while one that has it (or has no identity at all, matching
// today's no-auth-validator-configured fallback) does.
func TestWSHub_Broadcast_FiltersByPermission(t *testing.T) {
	reg := setupWSPermissionRegistry(t)
	hub := NewWSHub(reg)

	authorized := &wsConn{id: "authorized", workspace: "acme",
		identity: &auth.Identity{UserID: "u1", Permissions: []string{"clinic.visits.view"}},
		send:     make(chan events.EventMessage, 4)}
	unauthorized := &wsConn{id: "unauthorized", workspace: "acme",
		identity: &auth.Identity{UserID: "u2", Permissions: []string{"pharmacy.otc-sales.view"}},
		send:     make(chan events.EventMessage, 4)}
	noIdentity := &wsConn{id: "no-identity", workspace: "acme",
		send: make(chan events.EventMessage, 4)}

	// All three subscribe to everything; only the permission filter (2.6.6)
	// distinguishes who may receive the event.
	authorized.subscribe("*", "")
	unauthorized.subscribe("*", "")
	noIdentity.subscribe("*", "")

	hub.register(authorized)
	hub.register(unauthorized)
	hub.register(noIdentity)

	hub.Broadcast("acme", events.EventMessage{Event: "completed", Resource: "clinic/visit"})

	expectMessage(t, authorized, true)
	expectMessage(t, unauthorized, false)
	expectMessage(t, noIdentity, true)
}

// TestWSHub_Broadcast_UnresolvableResourceDeliversUnfiltered verifies the
// fail-open behavior for cases the hub can't reason about — an unknown
// resource or a hub with no registry — so it never mistakes "can't derive a
// permission" for "deny".
func TestWSHub_Broadcast_UnresolvableResourceDeliversUnfiltered(t *testing.T) {
	reg := setupWSPermissionRegistry(t)
	hub := NewWSHub(reg)

	conn := &wsConn{id: "c", workspace: "acme",
		identity: &auth.Identity{UserID: "u1", Permissions: []string{"clinic.visits.view"}},
		send:     make(chan events.EventMessage, 4)}
	conn.subscribe("*", "")
	hub.register(conn)

	hub.Broadcast("acme", events.EventMessage{Event: "completed", Resource: "unknown/thing"})
	expectMessage(t, conn, true)

	hubNoRegistry := NewWSHub(nil)
	conn2 := &wsConn{id: "c2", workspace: "acme",
		identity: &auth.Identity{UserID: "u1", Permissions: []string{}},
		send:     make(chan events.EventMessage, 4)}
	conn2.subscribe("*", "")
	hubNoRegistry.register(conn2)
	hubNoRegistry.Broadcast("acme", events.EventMessage{Event: "completed", Resource: "clinic/visit"})
	expectMessage(t, conn2, true)
}
