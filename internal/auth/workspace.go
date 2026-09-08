package auth

import (
	"context"
	"errors"
	"fmt"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// isNotFound reports whether err is the store's "no row" sentinel
// (FindByField wraps jsonb-persist.ErrNotFound when no record matches).
func isNotFound(err error) bool {
	return errors.Is(err, db.ErrNotFound)
}

// WorkspaceRegistry manages named workspaces backed by the embedded
// formspec.core/workspace entity (internal/auth/module/master/workspace/
// entity.yaml). It is the single registry consulted by WorkspaceMiddleware
// (plan docs_internal/plan/named-workspaces.md): manifest-declared
// `kind: Workspace` seeds and CLI-created workspaces both land here.
//
// Rows live under a fixed storage scope (WorkspaceRegistryScope) — the
// registry is not itself scoped per workspace, because its rows ARE the
// workspaces. The URL slug equals the workspace ID used everywhere else
// (stores, sessions, permissions); the registry only validates existence.
//
// This is framework code — it calls the store directly without permission
// checks, because workspace registration is trusted infrastructure.
const (
	// WorkspaceRegistryScope is the storage scope (tenant_id column value)
	// of the workspace registry rows themselves. Namespaced so it can never
	// collide with a real workspace slug.
	WorkspaceRegistryScope = "formspec:workspaces"
	// CoreWorkspaceEntity is the formspec.core entity backing the registry.
	CoreWorkspaceEntity = "workspace"
)

// WorkspaceInfo is one registered workspace.
type WorkspaceInfo struct {
	// ID is the record ID in the registry store.
	ID string
	// Slug is the URL-facing workspace identifier — equals the workspace ID
	// used as tenant scope everywhere (/{ws}/... URLs, store rows, sessions).
	Slug string
	// DisplayName is the human-readable workspace name.
	DisplayName string
	// OwnerUserID is the optional seeded owner (user ID). Empty = none.
	OwnerUserID string
}

// WorkspaceRegistry is the workspace lookup/store backed by the
// formspec.core/workspace entity.
type WorkspaceRegistry struct {
	store *db.EntityStore
}

// NewWorkspaceRegistry creates a workspace registry backed by the given
// EntityStore (formspec.core/workspace).
func NewWorkspaceRegistry(store *db.EntityStore) *WorkspaceRegistry {
	return &WorkspaceRegistry{store: store}
}

// Registered reports whether slug is a registered workspace. Implements
// api.WorkspaceResolver.
func (r *WorkspaceRegistry) Registered(ctx context.Context, slug string) (bool, error) {
	rec, err := r.store.FindByField(ctx, WorkspaceRegistryScope, "slug", slug)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("auth: lookup workspace %q: %w", slug, err)
	}
	return rec != nil, nil
}

// Ensure registers the workspace when missing; when it already exists the
// display name/owner are updated from the seed (manifest wins on reload —
// it is the declarative source). Returns true when the row was created.
func (r *WorkspaceRegistry) Ensure(ctx context.Context, info WorkspaceInfo) (created bool, err error) {
	rec, err := r.store.FindByField(ctx, WorkspaceRegistryScope, "slug", info.Slug)
	if err != nil {
		if isNotFound(err) {
			rec = nil
		} else {
			return false, fmt.Errorf("auth: lookup workspace %q: %w", info.Slug, err)
		}
	}
	if rec != nil {
		updates := map[string]any{}
		if info.DisplayName != "" && rec.Data["name"] != info.DisplayName {
			updates["name"] = info.DisplayName
		}
		if info.OwnerUserID != "" && rec.Data["owner_user_id"] != info.OwnerUserID {
			updates["owner_user_id"] = info.OwnerUserID
		}
		if len(updates) > 0 {
			if err := r.store.UpdateFields(ctx, WorkspaceRegistryScope, rec.ID, updates); err != nil {
				return false, fmt.Errorf("auth: update workspace %q: %w", info.Slug, err)
			}
		}
		return false, nil
	}
	data := map[string]any{
		"name": info.DisplayName,
		"slug": info.Slug,
	}
	if info.OwnerUserID != "" {
		data["owner_user_id"] = info.OwnerUserID
	}
	if _, err := r.store.Insert(ctx, db.InsertParams{
		WorkspaceID: WorkspaceRegistryScope,
		CreatedBy:   "system",
		Data:        data,
	}); err != nil {
		return false, fmt.Errorf("auth: insert workspace %q: %w", info.Slug, err)
	}
	return true, nil
}

// List returns all registered workspaces, ordered by slug.
func (r *WorkspaceRegistry) List(ctx context.Context) ([]WorkspaceInfo, error) {
	res, err := r.store.List(ctx, db.ListParams{
		WorkspaceID: WorkspaceRegistryScope,
		Page:        1,
		PerPage:     1000,
	})
	if err != nil {
		return nil, fmt.Errorf("auth: list workspaces: %w", err)
	}
	out := make([]WorkspaceInfo, 0, len(res.Data))
	for _, rec := range res.Data {
		out = append(out, workspaceFromRecord(rec.ID, rec.Data))
	}
	return out, nil
}

// Delete removes the workspace registration (soft delete). Returns an error
// when the slug is not registered.
func (r *WorkspaceRegistry) Delete(ctx context.Context, slug string) error {
	rec, err := r.store.FindByField(ctx, WorkspaceRegistryScope, "slug", slug)
	if err != nil {
		if isNotFound(err) {
			return ErrWorkspaceNotFound
		}
		return fmt.Errorf("auth: lookup workspace %q: %w", slug, err)
	}
	if rec == nil {
		return ErrWorkspaceNotFound
	}
	if err := r.store.SoftDelete(ctx, WorkspaceRegistryScope, rec.ID); err != nil {
		return fmt.Errorf("auth: delete workspace %q: %w", slug, err)
	}
	return nil
}

// Count returns the number of registered workspaces (used by the dev seed
// check "registry empty → seed default").
func (r *WorkspaceRegistry) Count(ctx context.Context) (int, error) {
	list, err := r.List(ctx)
	if err != nil {
		return 0, err
	}
	return len(list), nil
}

// ErrWorkspaceNotFound is returned when a workspace slug is not registered.
var ErrWorkspaceNotFound = fmt.Errorf("auth: workspace not found")

// workspaceFromRecord maps a registry record to WorkspaceInfo.
func workspaceFromRecord(id string, data map[string]any) WorkspaceInfo {
	return WorkspaceInfo{
		ID:          id,
		Slug:        asString(data["slug"]),
		DisplayName: asString(data["name"]),
		OwnerUserID: asString(data["owner_user_id"]),
	}
}

// asString coerces a JSONB value to string (nil-safe).
func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
