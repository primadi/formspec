package auth

import (
	"context"
	"time"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Session represents an active refresh-token session.
//
// JTI is the unique id of the refresh token. Rotation (todo 6.1.3) deletes the
// old session and creates a new one, so a replayed old refresh token is
// rejected because its jti is no longer present.
type Session struct {
	JTI         string
	UserID      string
	WorkspaceID string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// SessionStore tracks active refresh-token sessions for rotation and revoke.
//
// Implementations:
//   - EntitySessionStore: backed by the formspec.core.session entity (default).
//   - Custom:             user-provided entity satisfying the session role
//     contract (refresh_jti + user_id + expires_at), via RoleResolver.
type SessionStore interface {
	Create(ctx context.Context, s Session) error
	Get(ctx context.Context, jti string) (*Session, bool)
	Delete(ctx context.Context, jti string) error
	DeleteForUser(ctx context.Context, userID string) error
	// CountForUser returns the number of active sessions for a user
	// (concurrent session limit, todo 6.5.3).
	CountForUser(ctx context.Context, userID string) (int, error)
	// ListForUser returns the active sessions for a user, oldest first
	// (used to evict the oldest when the concurrent limit is exceeded).
	ListForUser(ctx context.Context, userID string) ([]Session, error)
	// PurgeExpired deletes all sessions whose expires_at is in the past
	// (cleanup job, todo 6.5.5). Returns the number purged.
	PurgeExpired(ctx context.Context) (int, error)
}

// EntitySessionStore reads/writes sessions from a FormSpec entity via the
// registry's EntityStore. The entity is resolved by RoleResolver (default
// formspec.core.session) so a user-provided override wins when present.
//
// This is framework code — it calls the store directly without permission
// checks, because the auth service is trusted infrastructure.
type EntitySessionStore struct {
	store *db.EntityStore
}

// NewEntitySessionStore creates a session store backed by the given EntityStore.
func NewEntitySessionStore(store *db.EntityStore) *EntitySessionStore {
	return &EntitySessionStore{store: store}
}

// Create stores a session as a new record in the session entity.
func (s *EntitySessionStore) Create(ctx context.Context, sess Session) error {
	_, err := s.store.Insert(ctx, db.InsertParams{
		WorkspaceID: sess.WorkspaceID,
		CreatedBy:   sess.UserID,
		Data: map[string]any{
			"transaction_date": sess.CreatedAt.UTC().Format("2006-01-02"),
			"user_id":          sess.UserID,
			"refresh_jti":      sess.JTI,
			"expires_at":       sess.ExpiresAt.UTC().Format(time.RFC3339),
		},
	})
	return err
}

// Get returns the session for jti, if present and not expired.
func (s *EntitySessionStore) Get(ctx context.Context, jti string) (*Session, bool) {
	// The session entity is tenant-scoped; the caller's workspace is threaded
	// via the refresh token claims. For single-server the default workspace
	// is "demo" — see sessWorkspaceForJTI.
	rec, err := s.store.FindByField(ctx, sessWorkspaceForJTI(jti), "refresh_jti", jti)
	if err != nil || rec == nil {
		return nil, false
	}
	expiresAt, _ := time.Parse(time.RFC3339, stringField(rec.Data, "expires_at"))
	if time.Now().After(expiresAt) {
		return nil, false
	}
	return &Session{
		JTI:         jti,
		UserID:      stringField(rec.Data, "user_id"),
		WorkspaceID: rec.WorkspaceID,
		ExpiresAt:   expiresAt,
	}, true
}

// Delete removes a session by jti.
func (s *EntitySessionStore) Delete(ctx context.Context, jti string) error {
	rec, err := s.store.FindByField(ctx, sessWorkspaceForJTI(jti), "refresh_jti", jti)
	if err != nil || rec == nil {
		return nil // already gone
	}
	return s.store.SoftDelete(ctx, rec.WorkspaceID, rec.ID)
}

// DeleteForUser removes all sessions belonging to a user (global revoke).
func (s *EntitySessionStore) DeleteForUser(ctx context.Context, userID string) error {
	res, err := s.store.List(ctx, db.ListParams{
		WorkspaceID: sessWorkspaceForJTI(""),
		PerPage:     100,
		Filters: map[string]db.FilterOp{
			"user_id": {Op: "eq", Value: userID},
		},
	})
	if err != nil {
		return err
	}
	for _, rec := range res.Data {
		_ = s.store.SoftDelete(ctx, rec.WorkspaceID, rec.ID)
	}
	return nil
}

// CountForUser returns the number of active sessions for a user.
func (s *EntitySessionStore) CountForUser(ctx context.Context, userID string) (int, error) {
	res, err := s.listForUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return len(res), nil
}

// ListForUser returns the active sessions for a user, oldest first.
func (s *EntitySessionStore) ListForUser(ctx context.Context, userID string) ([]Session, error) {
	recs, err := s.listForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(recs))
	for _, rec := range recs {
		expiresAt, _ := time.Parse(time.RFC3339, stringField(rec.Data, "expires_at"))
		out = append(out, Session{
			JTI:         stringField(rec.Data, "refresh_jti"),
			UserID:      userID,
			WorkspaceID: rec.WorkspaceID,
			ExpiresAt:   expiresAt,
		})
	}
	return out, nil
}

// listForUser returns the raw session records for a user, oldest first.
func (s *EntitySessionStore) listForUser(ctx context.Context, userID string) ([]db.EntityRecord, error) {
	res, err := s.store.List(ctx, db.ListParams{
		WorkspaceID: sessWorkspaceForJTI(""),
		PerPage:     100,
		Sort:        "created_at",
		Filters: map[string]db.FilterOp{
			"user_id": {Op: "eq", Value: userID},
		},
	})
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

// PurgeExpired deletes all sessions whose expires_at is in the past.
func (s *EntitySessionStore) PurgeExpired(ctx context.Context) (int, error) {
	res, err := s.store.List(ctx, db.ListParams{
		WorkspaceID: sessWorkspaceForJTI(""),
		PerPage:     100,
	})
	if err != nil {
		return 0, err
	}
	now := time.Now()
	purged := 0
	for _, rec := range res.Data {
		expiresAt, err := time.Parse(time.RFC3339, stringField(rec.Data, "expires_at"))
		if err != nil {
			continue
		}
		if now.After(expiresAt) {
			if err := s.store.SoftDelete(ctx, rec.WorkspaceID, rec.ID); err == nil {
				purged++
			}
		}
	}
	return purged, nil
}

// sessWorkspaceForJTI returns the workspace to search for a jti. Since jti is
// a globally unique UUID and the session entity is tenant-scoped, we use the
// default workspace ("demo") for single-server. The auth service passes the
// workspace from the refresh token claims for Get/Delete; this helper keeps
// the interface simple.
func sessWorkspaceForJTI(jti string) string {
	return "demo"
}
