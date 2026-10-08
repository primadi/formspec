package api

import (
	"testing"

	"github.com/primadi/formspec/internal/auth"
)

// sessionContextOf feeds `/_meta/me` the two facts a context switcher needs:
// which context the session is ACTING IN, and which ones the principal MAY act
// in (kafe 6.5.10 / backend §8.7).
//
// Both halves matter and neither can be derived from the other. `roles` says
// what the principal IS — with a context-scoped session the token carries one
// role plus `attrs`, so `roles` cannot say which branch the session is bound
// to; and the choices live on the principal record, not in the token (a token
// that listed them would be a token that could be edited to widen itself).

func TestSessionContextOf_ScopedSessionReportsItsBoundary(t *testing.T) {
	id := &auth.Identity{
		UserID:      "u-1",
		WorkspaceID: "kafe",
		Roles:       []string{"sales"},
		Attributes:  map[string]string{"branch_id": "KFE-JKT-01"},
	}
	user := &auth.User{ID: "u-1", Assignments: []auth.Assignment{
		{Role: "sales", Dimension: "branch_id", Value: "KFE-JKT-01"},
		{Role: "admin", Dimension: "branch_id", Value: "KFE-BDG-01"},
	}}

	ctx, choices := sessionContextOf(id, user)

	if ctx == nil {
		t.Fatal("a context-scoped session must report its context — otherwise the UI cannot say where the caller is")
	}
	if ctx.Role != "sales" || ctx.Dimension != "branch_id" || ctx.Value != "KFE-JKT-01" {
		t.Fatalf("context = %#v, want {sales, branch_id, KFE-JKT-01}", ctx)
	}
	// The dimension NAME is recovered from the assignments: the token carries
	// only `{branch_id: ...}`, and matching it back is what makes the label
	// readable rather than a raw attribute key.
	if len(choices) != 2 {
		t.Fatalf("choices = %#v, want both assignments", choices)
	}
	if choices[0].ID != "sales@KFE-JKT-01" || choices[1].ID != "admin@KFE-BDG-01" {
		t.Fatalf("choice ids = %q, %q — the switcher posts these back verbatim", choices[0].ID, choices[1].ID)
	}
}

// A boundary-less session (owner / service account, §8.7) has no dimension, and
// an incomplete assignment is not a context anyone can choose — it is skipped
// rather than offered.
func TestSessionContextOf_BoundarylessAndIncomplete(t *testing.T) {
	id := &auth.Identity{UserID: "owner-1", WorkspaceID: "kafe", Roles: []string{"owner"}}
	user := &auth.User{Assignments: []auth.Assignment{
		{Role: "owner"}, // incomplete — no dimension/value
		{Role: "", Dimension: "branch_id", Value: "B1"}, // incomplete — no role
	}}

	ctx, choices := sessionContextOf(id, user)
	if len(choices) != 0 {
		t.Fatalf("choices = %#v, want none (incomplete rows are not contexts)", choices)
	}
	// The role is still reported: the session IS acting as someone, even though
	// it has no boundary. A switcher with nothing to switch to stays hidden.
	if ctx == nil || ctx.Role != "owner" {
		t.Fatalf("context = %#v, want a role-only context", ctx)
	}
	if ctx.Dimension != "" || ctx.Value != "" {
		t.Fatalf("context = %#v, want no dimension for a boundary-less session", ctx)
	}
}

// Several roles means the session is NOT context-scoped (the legacy union), so
// there is no single role to report — claiming one would misstate the boundary
// in force.
func TestSessionContextOf_UnionSessionReportsNoSingleRole(t *testing.T) {
	id := &auth.Identity{
		UserID: "u-2", WorkspaceID: "kafe",
		Roles:      []string{"sales", "admin"},
		Attributes: map[string]string{"branch_id": "B1"},
	}
	ctx, _ := sessionContextOf(id, &auth.User{})
	if ctx == nil {
		t.Fatal("expected a context carrying the session's attribute")
	}
	if ctx.Role != "" {
		t.Fatalf("role = %q, want empty — a union session has no single role", ctx.Role)
	}
	// The raw attribute is still reported rather than dropping it: the session
	// DOES carry a boundary value, and hiding it would make the UI claim the
	// caller is unbounded while the server filters by branch.
	if ctx.Dimension != "branch_id" || ctx.Value != "B1" {
		t.Fatalf("context = %#v, want the raw attribute reported", ctx)
	}
}
