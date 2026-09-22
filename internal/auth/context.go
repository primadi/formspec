// ─── Session context: (principal, role, dimension value) ───
//
// A session is always specific: who the caller is, which role they act as, and
// in which branch. A principal's `assignments` list holds every context they
// may act in; login picks exactly one. See plan
// `docs_internal/plan/session-context-role-branch.md` (TODO 3.8) and the
// normative contract in `docs/spec/backend/01-core-basic.md` §8.6.
//
// The choice is deliberately NOT a "list of roles + list of branches" that
// combine freely: the boundary is a single (role, dimension, value) triple, so
// a session can never widen itself, and an audit answers "as which role, in
// which branch" for every action.

package auth

import (
	"context"
	"errors"
	"strings"
)

// ErrContextRequired is returned when a login cannot proceed without an
// explicit session context: the principal holds more than one assignment and
// none was chosen, the chosen one no longer exists (revoked), or the role
// backing it was removed. Callers must fail closed and ask the principal to
// choose again — never fall back to an unscoped session.
var ErrContextRequired = errors.New("auth: choose a session context")

// ContextChoice is one entry of the picker a client shows when a login needs
// the caller to choose a session context.
type ContextChoice struct {
	// ID is the identifier the client sends back as the login/switch
	// `assignment` value. Shape: `<role>@<value>`.
	ID        string `json:"id"`
	Role      string `json:"role"`
	Dimension string `json:"dimension"`
	Value     string `json:"value"`
}

// ContextRequiredError carries the choices a client needs to render the picker.
// It satisfies errors.Is(err, ErrContextRequired).
type ContextRequiredError struct {
	Choices []ContextChoice
}

// Error implements error.
func (e *ContextRequiredError) Error() string {
	if len(e.Choices) == 0 {
		return "auth: this session context is no longer valid — sign in again"
	}
	return "auth: choose a session context (role + branch) to continue"
}

// Is makes errors.Is(err, ErrContextRequired) true.
func (e *ContextRequiredError) Is(target error) bool { return target == ErrContextRequired }

// Complete reports whether every part of the assignment is present. The core
// user entity declares all three as required; an incomplete row is not a
// usable context and is ignored when choices are built.
func (a Assignment) Complete() bool {
	return a.Role != "" && a.Dimension != "" && a.Value != ""
}

// ID returns the wire identifier of the assignment: `<role>@<value>`.
//
// Role names cannot contain `@` (schema pattern `^[a-z][a-z0-9_]*$`), so the
// first `@` is always the separator — a value may contain `@` freely without
// escaping. The id is self-describing, so a choice stored by a client (e.g. in
// localStorage for the OAuth flow) still means the same thing after the
// assignment list is reordered.
func (a Assignment) ID() string { return a.Role + "@" + a.Value }

// Attrs returns the token `attrs` payload for this context: the dimension name
// mapped to its value (e.g. {"branch": "KFE-JKT-01"}). This is what
// `row_scope: {from: session}` reads.
func (a Assignment) Attrs() map[string]string {
	return map[string]string{a.Dimension: a.Value}
}

// splitAssignmentID splits `<role>@<value>` at the FIRST `@`.
func splitAssignmentID(id string) (role, value string, ok bool) {
	role, value, ok = strings.Cut(id, "@")
	if !ok || role == "" || value == "" {
		return "", "", false
	}
	return role, value, true
}

// usableAssignments returns the complete assignments of a principal, in
// declaration order.
func usableAssignments(user *User) []Assignment {
	if user == nil {
		return nil
	}
	out := make([]Assignment, 0, len(user.Assignments))
	for _, a := range user.Assignments {
		if a.Complete() {
			out = append(out, a)
		}
	}
	return out
}

// choicesFor builds the picker payload from a principal's assignments.
func choicesFor(user *User) []ContextChoice {
	assignments := usableAssignments(user)
	out := make([]ContextChoice, 0, len(assignments))
	for _, a := range assignments {
		out = append(out, ContextChoice{
			ID:        a.ID(),
			Role:      a.Role,
			Dimension: a.Dimension,
			Value:     a.Value,
		})
	}
	return out
}

// resolveAssignment picks the session context for a login or a context switch.
//
//	0 assignments                → (nil, nil): no boundary (legacy behavior —
//	                               owner / service account, union of roles,
//	                               cross-branch reads via `read_all`).
//	1 assignment, no explicit id → that one, chosen automatically.
//	>1 and no explicit id        → ErrContextRequired + choices.
//	explicit id, found           → that one.
//	explicit id, unknown/revoked → ErrContextRequired + choices (fail closed:
//	                               a revoked assignment must never degrade to
//	                               "keep the old context" or "no boundary").
func resolveAssignment(user *User, id string) (*Assignment, error) {
	assignments := usableAssignments(user)
	if id == "" {
		switch len(assignments) {
		case 0:
			return nil, nil
		case 1:
			return &assignments[0], nil
		default:
			return nil, &ContextRequiredError{Choices: choicesFor(user)}
		}
	}
	role, value, ok := splitAssignmentID(id)
	if ok {
		for i := range assignments {
			if assignments[i].Role == role && assignments[i].Value == value {
				return &assignments[i], nil
			}
		}
	}
	return nil, &ContextRequiredError{Choices: choicesFor(user)}
}

// contextStillValid reports whether a context-scoped session may be renewed:
// the exact assignment must still be on the principal AND the role it names
// must still exist. Either change means the boundary the session was granted
// for is gone — the next token must not be issued, and the caller is asked to
// choose again (fail closed).
func (s *Service) contextStillValid(ctx context.Context, workspaceID string, user *User, role, dimension, value string) bool {
	if role == "" || dimension == "" || value == "" {
		return false
	}
	found := false
	for _, a := range user.Assignments {
		if a.Role == role && a.Dimension == dimension && a.Value == value {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	// The role itself must still resolve. Without a role store wired (unit
	// tests, minimal embedding) there is nothing to check against — the
	// permission resolver treats an unknown role as "no grants", which is
	// denied anyway.
	if s.roleStore == nil {
		return true
	}
	if _, err := s.roleStore.GetByName(ctx, workspaceID, role); err != nil {
		return false
	}
	return true
}
