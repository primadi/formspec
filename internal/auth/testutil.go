// ─── Test helpers ───
//
// Production code no longer seeds users (auth is uniform across dev and
// prod — the first admin comes from the setup wizard). SeedDevUser survives
// here purely as a test convenience for bootstrapping users in tests.

package auth

import "context"

// SeedDevUser creates a user with the admin role and wildcard permissions.
// Idempotent — if the username already exists, it is left untouched.
// TEST-ONLY: never call from production code.
func (s *Service) SeedDevUser(ctx context.Context, workspaceID, username, password string) error {
	if _, err := s.users.GetByUsername(ctx, workspaceID, username); err == nil {
		return nil // already seeded
	}
	return s.users.CreateUser(ctx, workspaceID, &User{
		Username:     username,
		PasswordHash: password, // hashed inside CreateUser
		WorkspaceID:  workspaceID,
		Roles:        []string{"admin"},
		Permissions:  []string{"*"},
		Active:       true,
	})
}
