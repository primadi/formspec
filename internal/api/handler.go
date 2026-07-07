package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/forma/forma/internal/auth"
	"github.com/forma/forma/internal/db"
	"github.com/forma/forma/internal/validation"
	"github.com/forma/forma/pkg/spec"
)

// HandlerFactory creates HTTP handlers backed by an EntityStore.
type HandlerFactory struct {
	registry EntityStoreProvider
}

// EntityStoreProvider abstracts the entity registry for handler use.
type EntityStoreProvider interface {
	GetEntityStore(module, name string) (*db.EntityStore, error)
}

// NewHandlerFactory creates a handler factory.
func NewHandlerFactory(registry EntityStoreProvider) *HandlerFactory {
	return &HandlerFactory{registry: registry}
}

// HandleList returns a GET / handler for the given entity.
func (f *HandlerFactory) HandleList(module, entity string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		store, err := f.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "entity not found: "+err.Error())
			return
		}

		tenantID := tenantFromContext(ctx)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

		result, err := store.List(ctx, db.ListParams{
			TenantID: tenantID,
			Page:     page,
			PerPage:  perPage,
			Search:   r.URL.Query().Get("search"),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, ListResponse{
			Data: result.Data,
			Meta: MetaList{
				Page:       result.Page,
				PerPage:    result.PerPage,
				Total:      result.Total,
				TotalPages: result.TotalPages,
			},
		})
	}
}

// HandleFind returns a GET /{id} handler for the given entity.
func (f *HandlerFactory) HandleFind(module, entity string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		store, err := f.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "entity not found: "+err.Error())
			return
		}

		tenantID := tenantFromContext(ctx)
		id := r.PathValue("id")

		rec, err := store.GetByID(ctx, db.GetByIDParams{TenantID: tenantID, ID: id})
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: rec,
			Meta: MetaSingle{RequestID: requestIDFromContext(ctx), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
	}
}

// HandleCreate returns a POST / handler for the given entity.
func (f *HandlerFactory) HandleCreate(module, entity string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		store, err := f.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "entity not found: "+err.Error())
			return
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid JSON: "+err.Error())
			return
		}

		tenantID := tenantFromContext(ctx)
		createdBy := userFromContext(ctx)

		id, err := store.Insert(ctx, db.InsertParams{
			TenantID:  tenantID,
			CreatedBy: createdBy,
			Data:      body,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}

		// Fetch the created record to return
		rec, err := store.GetByID(ctx, db.GetByIDParams{TenantID: tenantID, ID: id})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "created but fetch failed: "+err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, SingleResponse{
			Data: rec,
			Meta: MetaSingle{RequestID: requestIDFromContext(ctx), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
	}
}

// HandleUpdate returns a PATCH /{id} handler for the given entity.
func (f *HandlerFactory) HandleUpdate(module, entity string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		store, err := f.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "entity not found: "+err.Error())
			return
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid JSON: "+err.Error())
			return
		}

		tenantID := tenantFromContext(ctx)
		updatedBy := userFromContext(ctx)
		id := r.PathValue("id")

		// Get current version
		current, err := store.GetByID(ctx, db.GetByIDParams{TenantID: tenantID, ID: id})
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}

		newVersion, err := store.Update(ctx, db.UpdateParams{
			TenantID:  tenantID,
			ID:        id,
			Version:   current.Version,
			UpdatedBy: updatedBy,
			Data:      body,
		})
		if err != nil {
			code := http.StatusInternalServerError
			if isConflictError(err) {
				code = http.StatusConflict
			}
			writeError(w, code, "INTERNAL_ERROR", err.Error())
			return
		}

		// Fetch updated record
		rec, _ := store.GetByID(ctx, db.GetByIDParams{TenantID: tenantID, ID: id})
		// Attach new version
		if rec != nil {
			rec.Version = newVersion
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: rec,
			Meta: MetaSingle{RequestID: requestIDFromContext(ctx), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
	}
}

// HandleDelete returns a DELETE /{id} handler for the given entity.
func (f *HandlerFactory) HandleDelete(module, entity string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		store, err := f.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "entity not found: "+err.Error())
			return
		}

		tenantID := tenantFromContext(ctx)
		id := r.PathValue("id")

		if err := store.SoftDelete(ctx, tenantID, id); err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Response envelopes (Core §16) ---

// SingleResponse wraps a single record.
type SingleResponse struct {
	Data any        `json:"data"`
	Meta MetaSingle `json:"meta"`
}

// MetaSingle is metadata for single-record responses.
type MetaSingle struct {
	RequestID string `json:"request_id,omitempty"`
	Timestamp string `json:"timestamp"`
}

// ListResponse wraps a paginated list.
type ListResponse struct {
	Data any      `json:"data"`
	Meta MetaList `json:"meta"`
}

// MetaList is metadata for list responses.
type MetaList struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
	Meta  MetaSingle  `json:"meta"`
}

// ErrorDetail holds error information.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- Context helpers ---

type contextKey string

const (
	ctxTenantID  contextKey = "tenant_id"
	ctxUserID    contextKey = "user_id"
	ctxIdentity  contextKey = "identity"
	ctxRequestID contextKey = "request_id"
)

// IdentityFromContext extracts the authenticated identity from the request context.
// Returns nil if no identity is present (unauthenticated request).
func IdentityFromContext(ctx context.Context) *auth.Identity {
	id, _ := ctx.Value(ctxIdentity).(*auth.Identity)
	return id
}

// WithIdentity stores an Identity on the context.
func WithIdentity(ctx context.Context, id *auth.Identity) context.Context {
	return context.WithValue(ctx, ctxIdentity, id)
}

// tenantFromContext extracts the tenant ID from the request context.
// Priority: Identity.WorkspaceID > explicit tenant_id > default "demo".
func tenantFromContext(ctx context.Context) string {
	// Prefer workspace from identity (set by auth middleware)
	if id := IdentityFromContext(ctx); id != nil && id.WorkspaceID != "" {
		return id.WorkspaceID
	}
	v, _ := ctx.Value(ctxTenantID).(string)
	if v == "" {
		return "demo"
	}
	return v
}

// userFromContext extracts the user ID from the request context.
// Priority: Identity.UserID > explicit user_id > default "anonymous".
func userFromContext(ctx context.Context) string {
	// Prefer user from identity (set by auth middleware)
	if id := IdentityFromContext(ctx); id != nil && id.UserID != "" {
		return id.UserID
	}
	v, _ := ctx.Value(ctxUserID).(string)
	if v == "" {
		return "anonymous"
	}
	return v
}

// requestIDFromContext extracts the request ID from the context.
func requestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxRequestID).(string)
	return v
}

// WithTenant sets the tenant ID on the context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxTenantID, tenantID)
}

// WithUser sets the user ID on the context.
func WithUser(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxUserID, userID)
}

// WithRequestID sets the request ID on the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxRequestID, requestID)
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{Code: code, Message: message},
		Meta:  MetaSingle{Timestamp: time.Now().UTC().Format(time.RFC3339)},
	})
}

func isConflictError(err error) bool {
	s := err.Error()
	return contains(s, "version conflict") || contains(s, "not found")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// writeValidationErrors writes one or more validation errors as 422 VALIDATION_ERROR.
func writeValidationErrors(w http.ResponseWriter, errs []error) {
	msg := ""
	for i, e := range errs {
		if i > 0 {
			msg += "; "
		}
		msg += e.Error()
	}
	writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", msg)
}

// HandleCustomAction returns a handler for a named custom action on an entity.
// It validates action params before delegating to the action's impl.
// For now (Fase 1.6), params are validated but execution returns 501.
// Full execution (script_ref, native) comes in Fase 2.
func (f *HandlerFactory) HandleCustomAction(module, entity, actionName string, actionSpec spec.Action) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse request body as params
		var params map[string]any
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid JSON: "+err.Error())
			return
		}

		// Validate action params if declared
		if actionSpec.Params != nil && len(actionSpec.Params.Validate) > 0 {
			if errs := validation.ValidateActionParams(params, actionSpec.Params.Validate); len(errs) > 0 {
				writeValidationErrors(w, errs)
				return
			}
		}

		// TODO(Fase 2): execute action impl (script_ref, native, etc.)
		writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
			"custom action execution not yet implemented: "+actionName)
	}
}
