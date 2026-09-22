package auth

import (
	"context"
	"errors"
	"testing"
)

// TestAssignment_Identity pins the wire form of a session context: the id a
// client sends back (`<role>@<value>`), the token attrs it produces, and the
// completeness rule (a partial row is not a usable context).
func TestAssignment_Identity(t *testing.T) {
	a := Assignment{Role: "sales", Dimension: "branch", Value: "A"}
	if a.ID() != "sales@A" {
		t.Errorf("ID = %q, want sales@A", a.ID())
	}
	if attrs := a.Attrs(); attrs["branch"] != "A" || len(attrs) != 1 {
		t.Errorf("Attrs = %v, want {branch: A}", attrs)
	}
	if !a.Complete() {
		t.Error("complete assignment reported incomplete")
	}
	for _, partial := range []Assignment{
		{Role: "", Dimension: "branch", Value: "A"},
		{Role: "sales", Dimension: "", Value: "A"},
		{Role: "sales", Dimension: "branch", Value: ""},
	} {
		if partial.Complete() {
			t.Errorf("%+v reported complete", partial)
		}
	}
	// A value may contain `@` — only the first separator counts.
	if id := (Assignment{Role: "sales", Dimension: "branch", Value: "a@b"}).ID(); id != "sales@a@b" {
		t.Errorf("ID = %q, want sales@a@b", id)
	}
}

// TestResolveAssignment pins the automatic rules (TODO 3.8): none = no
// boundary, one = chosen, several = must choose, unknown id = fail closed.
func TestResolveAssignment(t *testing.T) {
	two := &User{Assignments: []Assignment{
		{Role: "sales", Dimension: "branch", Value: "A"},
		{Role: "admin", Dimension: "branch", Value: "B"},
	}}
	one := &User{Assignments: []Assignment{{Role: "sales", Dimension: "branch", Value: "A"}}}
	none := &User{}

	t.Run("no assignments means no boundary", func(t *testing.T) {
		got, err := resolveAssignment(none, "")
		if err != nil || got != nil {
			t.Fatalf("resolveAssignment = (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("single assignment is chosen automatically", func(t *testing.T) {
		got, err := resolveAssignment(one, "")
		if err != nil || got == nil || got.ID() != "sales@A" {
			t.Fatalf("resolveAssignment = (%v, %v), want sales@A", got, err)
		}
	})

	t.Run("several assignments require a choice", func(t *testing.T) {
		got, err := resolveAssignment(two, "")
		var need *ContextRequiredError
		if !errors.As(err, &need) {
			t.Fatalf("expected ContextRequiredError, got (%v, %v)", got, err)
		}
		if len(need.Choices) != 2 {
			t.Fatalf("choices = %d, want 2", len(need.Choices))
		}
		if !errors.Is(err, ErrContextRequired) {
			t.Error("errors.Is(err, ErrContextRequired) = false")
		}
	})

	t.Run("explicit id picks that context", func(t *testing.T) {
		got, err := resolveAssignment(two, "admin@B")
		if err != nil || got == nil || got.Role != "admin" || got.Value != "B" {
			t.Fatalf("resolveAssignment = (%v, %v), want admin@B", got, err)
		}
	})

	t.Run("revoked id fails closed with choices", func(t *testing.T) {
		got, err := resolveAssignment(two, "sales@Z")
		var need *ContextRequiredError
		if !errors.As(err, &need) {
			t.Fatalf("expected ContextRequiredError, got (%v, %v)", got, err)
		}
		if len(need.Choices) != 2 {
			t.Errorf("choices = %d, want the remaining 2", len(need.Choices))
		}
	})

	t.Run("incomplete rows are ignored", func(t *testing.T) {
		user := &User{Assignments: []Assignment{{Role: "sales", Value: "A"}}}
		got, err := resolveAssignment(user, "")
		if err != nil || got != nil {
			t.Fatalf("resolveAssignment = (%v, %v), want (nil, nil) — an incomplete row is not a context", got, err)
		}
	})
}

// cashierWithTwoContexts creates a principal holding (sales@A, admin@B).
func cashierWithTwoContexts(t *testing.T, svc *Service, workspace string) *User {
	t.Helper()
	user := &User{
		Username:     "kasir",
		PasswordHash: "kasir12345",
		Active:       true,
		Roles:        []string{"sales", "admin"},
		Assignments: []Assignment{
			{Role: "sales", Dimension: "branch", Value: "A"},
			{Role: "admin", Dimension: "branch", Value: "B"},
		},
	}
	if err := svc.users.CreateUser(context.Background(), workspace, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	stored, err := svc.users.GetByUsername(context.Background(), workspace, "kasir")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	return stored
}

// TestService_LoginRequiresContextChoice: a principal with two contexts gets no
// token until it says which boundary it wants.
func TestService_LoginRequiresContextChoice(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	cashierWithTwoContexts(t, svc, "demo")

	_, err := svc.Login(ctx, "demo", "", "kasir", "kasir12345")
	var need *ContextRequiredError
	if !errors.As(err, &need) {
		t.Fatalf("Login = %v, want ContextRequiredError (no token without a choice)", err)
	}
	if len(need.Choices) != 2 {
		t.Fatalf("choices = %d, want 2", len(need.Choices))
	}
	ids := []string{need.Choices[0].ID, need.Choices[1].ID}
	if ids[0] != "sales@A" || ids[1] != "admin@B" {
		t.Errorf("choices = %v, want [sales@A admin@B]", ids)
	}
}

// TestService_LoginWithChosenContextScopesRoleAndAttrs: the session acts as the
// chosen role only (never the union of roles) and carries the chosen dimension
// value in `attrs` — which is what `row_scope: {from: session}` reads.
func TestService_LoginWithChosenContextScopesRoleAndAttrs(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	cashierWithTwoContexts(t, svc, "demo")

	pair, err := svc.LoginWithContext(ctx, "demo", "", "kasir", "kasir12345", "sales@A")
	if err != nil {
		t.Fatalf("LoginWithContext: %v", err)
	}

	validator := NewJWTValidator("test-secret", "formspec", "")
	id, err := validator.Validate(ctx, pair.AccessToken)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(id.Roles) != 1 || id.Roles[0] != "sales" {
		t.Errorf("roles = %v, want [sales] — the session must not carry the union", id.Roles)
	}
	if got := id.Attributes["branch"]; got != "A" {
		t.Errorf("attrs[branch] = %q, want A", got)
	}

	// The other context is reachable by naming it, and then the boundary flips.
	other, err := svc.LoginWithContext(ctx, "demo", "", "kasir", "kasir12345", "admin@B")
	if err != nil {
		t.Fatalf("LoginWithContext(admin@B): %v", err)
	}
	idOther, err := validator.Validate(ctx, other.AccessToken)
	if err != nil {
		t.Fatalf("Validate(admin@B): %v", err)
	}
	if len(idOther.Roles) != 1 || idOther.Roles[0] != "admin" {
		t.Errorf("roles = %v, want [admin]", idOther.Roles)
	}
	if got := idOther.Attributes["branch"]; got != "B" {
		t.Errorf("attrs[branch] = %q, want B", got)
	}
}

// TestService_RefreshFailsClosedWhenAssignmentRevoked: revoking an assignment
// must not let an open session keep its old boundary — the next refresh asks
// for a new choice instead.
func TestService_RefreshFailsClosedWhenAssignmentRevoked(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	user := cashierWithTwoContexts(t, svc, "demo")

	pair, err := svc.LoginWithContext(ctx, "demo", "", "kasir", "kasir12345", "sales@A")
	if err != nil {
		t.Fatalf("LoginWithContext: %v", err)
	}

	// Revoke the context the session was granted for.
	if err := svc.users.SetAssignments(ctx, "demo", user.ID, []Assignment{
		{Role: "admin", Dimension: "branch", Value: "B"},
	}); err != nil {
		t.Fatalf("SetAssignments: %v", err)
	}

	_, err = svc.Refresh(ctx, pair.RefreshToken)
	var need *ContextRequiredError
	if !errors.As(err, &need) {
		t.Fatalf("Refresh = %v, want ContextRequiredError after revocation", err)
	}
}

// TestService_SwitchContext: switching keeps the principal logged in, issues a
// token for the new boundary, and retires the previous session.
func TestService_SwitchContext(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()
	cashierWithTwoContexts(t, svc, "demo")

	pair, err := svc.LoginWithContext(ctx, "demo", "", "kasir", "kasir12345", "sales@A")
	if err != nil {
		t.Fatalf("LoginWithContext: %v", err)
	}

	switched, err := svc.SwitchContextByToken(ctx, pair.RefreshToken, "", "admin@B")
	if err != nil {
		t.Fatalf("SwitchContextByToken: %v", err)
	}
	validator := NewJWTValidator("test-secret", "formspec", "")
	id, err := validator.Validate(ctx, switched.AccessToken)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(id.Roles) != 1 || id.Roles[0] != "admin" || id.Attributes["branch"] != "B" {
		t.Errorf("switched session = roles %v attrs %v, want admin/B", id.Roles, id.Attributes)
	}

	// The replaced session is gone: its refresh token cannot be reused.
	if _, err := svc.Refresh(ctx, pair.RefreshToken); err == nil {
		t.Error("expected the pre-switch refresh token to be rejected")
	}

	// Switching to a context the principal does not hold fails closed.
	if _, err := svc.SwitchContextByToken(ctx, switched.RefreshToken, "", "sales@Z"); !errors.Is(err, ErrContextRequired) {
		t.Errorf("switch to unknown context = %v, want ErrContextRequired", err)
	}
}

// TestService_LoginWithoutAssignmentsKeepsLegacyBehavior: principals without
// contexts (owner / service account) keep the union of roles and a session
// with no boundary — the feature is additive.
func TestService_LoginWithoutAssignmentsKeepsLegacyBehavior(t *testing.T) {
	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	if err := svc.SeedDevUser(ctx, "demo", "owner", "owner12345"); err != nil {
		t.Fatalf("SeedDevUser: %v", err)
	}
	pair, err := svc.Login(ctx, "demo", "", "owner", "owner12345")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	validator := NewJWTValidator("test-secret", "formspec", "")
	id, err := validator.Validate(ctx, pair.AccessToken)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(id.Roles) != 1 || id.Roles[0] != "admin" {
		t.Errorf("roles = %v, want [admin] (legacy union path)", id.Roles)
	}
	if len(id.Attributes) != 0 {
		t.Errorf("attrs = %v, want none for a boundary-less session", id.Attributes)
	}
}
