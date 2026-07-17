package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/primadi/forma/internal/action"
	"github.com/primadi/forma/internal/auth"
	"github.com/primadi/forma/renderers/jsonbpersist"
	"github.com/primadi/forma/internal/validation"
	"github.com/primadi/forma/pkg/spec"
)

// HandlerFactory creates HTTP handlers backed by an EntityStore.
type HandlerFactory struct {
	registry     EntityStoreProvider
	dispatcher   *action.Dispatcher
	specLookup   func(module, name string) (*spec.EntitySpec, bool) // optional — enables sort/filter validation, hooks, and event resolution
	deliveryDeps action.DeliveryDeps
}

// EntityStoreProvider abstracts the entity registry for handler use.
type EntityStoreProvider interface {
	GetEntityStore(module, name string) (*db.EntityStore, error)
}

// NewHandlerFactory creates a handler factory.
func NewHandlerFactory(registry EntityStoreProvider) *HandlerFactory {
	return &HandlerFactory{registry: registry, dispatcher: action.NewDispatcher()}
}

// SetDispatcher sets the action dispatcher used for custom action execution.
func (f *HandlerFactory) SetDispatcher(d *action.Dispatcher) {
	f.dispatcher = d
}

// SetSpecLookup wires entity-spec resolution, enabling `sort` and field
// filter query params on list endpoints (validated against the spec), and
// hook/event resolution on create, update, and custom actions.
func (f *HandlerFactory) SetSpecLookup(fn func(module, name string) (*spec.EntitySpec, bool)) {
	f.specLookup = fn
}

// SetDeliveryDeps wires the event-delivery dependencies (hub, outbox, event
// log) used to fan out declared events after a successful action.
func (f *HandlerFactory) SetDeliveryDeps(deps action.DeliveryDeps) {
	f.deliveryDeps = deps
}

// resolveAction returns the *spec.Action named name from es, or nil if es
// is nil or declares no such action — the common case for most
// entities/actions today, in which case the hook/impl execution wired into
// HandleCreate/HandleUpdate below is a complete no-op (preserves exact
// prior behavior).
func resolveAction(es *spec.EntitySpec, name string) *spec.Action {
	if es == nil {
		return nil
	}
	for i, a := range es.Actions {
		if a.Name == name {
			return &es.Actions[i]
		}
	}
	return nil
}

// emitsOf returns a's declared Emits event name, or "" if a is nil.
func emitsOf(a *spec.Action) string {
	if a == nil {
		return ""
	}
	return a.Emits
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

		workspaceID := workspaceFromContext(ctx)

		// Pagination: spec §558-559 — non-numeric/negative → VALIDATION_ERROR (422)
		page, perPage, err := parsePaginationParams(r)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
			return
		}

		sortParam, filters, err := f.parseListQuery(r, module, entity)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
			return
		}

		result, err := store.List(ctx, db.ListParams{
			WorkspaceID: workspaceID,
			Page:        page,
			PerPage:     perPage,
			Sort:        sortParam,
			Filters:     filters,
			Search:      r.URL.Query().Get("search"),
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
			Links: buildListLinks(r.URL, result.Page, result.PerPage, result.TotalPages),
		})
	}
}

// reservedListParams are query keys with framework meaning — everything else
// is treated as a field filter.
var reservedListParams = map[string]bool{
	"page": true, "per_page": true, "search": true, "sort": true,
}

// filterOps are the supported bracket operators: field[op]=value.
var filterOps = map[string]bool{
	"eq": true, "neq": true, "gt": true, "gte": true, "lt": true, "lte": true,
	"like": true, "in": true, "nin": true,
}

// parseListQuery extracts sort + field filters from the query string
// (design doc §4.3):
//
//	?sort=-created_at&status=confirmed&total[gte]=100&status[in]=a,b
//
// Fields must exist on the entity and be addressable in SQL — normative
// columns, or data fields declared with index/unique/natural_key (those have
// generated columns). Unknown or non-filterable fields → error (422 upstream).
// Without a spec lookup wired, sort/filters are rejected rather than passed
// through to fail obscurely in SQL.
func (f *HandlerFactory) parseListQuery(r *http.Request, module, entity string) (string, map[string]db.FilterOp, error) {
	q := r.URL.Query()

	hasExtras := q.Get("sort") != ""
	if !hasExtras {
		for key := range q {
			if !reservedListParams[key] {
				hasExtras = true
				break
			}
		}
	}
	if !hasExtras {
		return "", nil, nil
	}

	if f.specLookup == nil {
		return "", nil, fmt.Errorf("sort/filter params are not supported on this deployment")
	}
	es, ok := f.specLookup(module, entity)
	if !ok {
		return "", nil, fmt.Errorf("entity spec not found: %s/%s", module, entity)
	}

	filterable := map[string]bool{}
	for _, fld := range es.Fields {
		if fld.Index || fld.Unique || fld.NaturalKey {
			filterable[fld.Name] = true
		}
	}
	checkField := func(name string) error {
		if db.IsNormativeColumn(name) || filterable[name] {
			return nil
		}
		for _, fld := range es.Fields {
			if fld.Name == name {
				return fmt.Errorf("field %q is not filterable/sortable — declare index: true on it", name)
			}
		}
		return fmt.Errorf("unknown field %q", name)
	}

	// Sort
	sortParam := q.Get("sort")
	if sortParam != "" {
		if err := checkField(strings.TrimPrefix(sortParam, "-")); err != nil {
			return "", nil, fmt.Errorf("sort: %w", err)
		}
	}

	// Filters
	var filters map[string]db.FilterOp
	for key, values := range q {
		if reservedListParams[key] || len(values) == 0 {
			continue
		}
		field, op := key, "eq"
		if i := strings.IndexByte(key, '['); i > 0 && strings.HasSuffix(key, "]") {
			field, op = key[:i], key[i+1:len(key)-1]
			if !filterOps[op] {
				return "", nil, fmt.Errorf("filter %q: unknown operator %q", field, op)
			}
		}
		if err := checkField(field); err != nil {
			return "", nil, err
		}

		var value any = values[0]
		if op == "in" || op == "nin" {
			parts := strings.Split(values[0], ",")
			anyParts := make([]any, len(parts))
			for i, p := range parts {
				anyParts[i] = strings.TrimSpace(p)
			}
			value = anyParts
		}

		if filters == nil {
			filters = map[string]db.FilterOp{}
		}
		filters[field] = db.FilterOp{Op: op, Value: value}
	}

	return sortParam, filters, nil
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

		workspaceID := workspaceFromContext(ctx)
		id := r.PathValue("id")

		rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
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

// HandleCreate returns a POST / handler for the given entity. If the entity
// declares a create action with its own impl and/or hooks: entries scoped
// to create, they run as a synchronous, cancelable before-phase (Core
// Extended §8) ahead of the actual insert — the base guard (Insert itself)
// always runs afterward and can never be skipped or replaced. Most entities
// declare neither, in which case this is byte-for-byte the original
// decode → Insert → fetch flow.
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
		if body == nil {
			body = make(map[string]any)
		}

		workspaceID := workspaceFromContext(ctx)
		createdBy := userFromContext(ctx)

		var entitySpec *spec.EntitySpec
		if f.specLookup != nil {
			entitySpec, _ = f.specLookup(module, entity)
		}
		actionSpec := resolveAction(entitySpec, "create")
		var hooks []spec.HookDecl
		if entitySpec != nil {
			hooks = entitySpec.Hooks
		}

		execParams := &action.ExecuteParams{
			Module: module, Entity: entity, ActionName: "create",
			Resource: body, Params: body, WorkspaceID: workspaceID, UserID: createdBy,
		}
		if err := action.RunBeforePhase(ctx, f.dispatcher, hooks, actionSpec, "create", execParams); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "HOOK_ABORTED", err.Error())
			return
		}

		id, err := store.Insert(ctx, db.InsertParams{
			WorkspaceID: workspaceID,
			CreatedBy:   createdBy,
			Data:        execParams.Resource,
		})
		if err != nil {
			writeStoreError(w, err)
			return
		}

		// Fetch the created record to return
		rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "created but fetch failed: "+err.Error())
			return
		}

		if rec != nil {
			action.RunAfterPhase(ctx, f.dispatcher, hooks, actionSpec, "create", action.ExecuteParams{
				Module: module, Entity: entity, ActionName: "create", ResourceID: id,
				Resource: rec.Data, WorkspaceID: workspaceID, UserID: createdBy,
			})
			if entitySpec != nil {
				if emitted := action.ResolveEmission(entitySpec.Events, emitsOf(actionSpec), rec.Data); emitted != nil {
					action.DeliverEvents(ctx, f.deliveryDeps, workspaceID, module+"/"+entity, []action.EventEmission{*emitted})
				}
			}
		}

		writeJSON(w, http.StatusCreated, SingleResponse{
			Data: rec,
			Meta: MetaSingle{RequestID: requestIDFromContext(ctx), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
	}
}

// HandleUpdate returns a PATCH /{id} handler for the given entity. Like
// HandleCreate, an entity-declared update action's impl and hooks: entries
// run as a synchronous, cancelable before-phase ahead of the actual update;
// the base guard (Update itself) always runs afterward.
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

		workspaceID := workspaceFromContext(ctx)
		updatedBy := userFromContext(ctx)
		id := r.PathValue("id")

		// Get current version and data — PATCH is a partial update, so the
		// submitted body is merged onto the existing record rather than
		// replacing it outright (Update()'s SQL overwrites the whole JSON
		// blob, and required-field validation runs against the merged
		// result, not just the fields the caller happened to resend).
		current, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}

		merged := current.Data
		if merged == nil {
			merged = make(map[string]any, len(body))
		}
		for k, v := range body {
			merged[k] = v
		}

		var entitySpec *spec.EntitySpec
		if f.specLookup != nil {
			entitySpec, _ = f.specLookup(module, entity)
		}
		actionSpec := resolveAction(entitySpec, "update")
		var hooks []spec.HookDecl
		if entitySpec != nil {
			hooks = entitySpec.Hooks
		}

		execParams := &action.ExecuteParams{
			Module: module, Entity: entity, ActionName: "update", ResourceID: id,
			Resource: merged, ResourceVersion: current.Version, Params: body,
			WorkspaceID: workspaceID, UserID: updatedBy,
		}
		if err := action.RunBeforePhase(ctx, f.dispatcher, hooks, actionSpec, "update", execParams); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "HOOK_ABORTED", err.Error())
			return
		}

		newVersion, err := store.Update(ctx, db.UpdateParams{
			WorkspaceID: workspaceID,
			ID:          id,
			Version:     current.Version,
			UpdatedBy:   updatedBy,
			Data:        execParams.Resource,
		})
		if err != nil {
			writeStoreError(w, err)
			return
		}

		// Fetch updated record
		rec, _ := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
		// Attach new version
		if rec != nil {
			rec.Version = newVersion
			action.RunAfterPhase(ctx, f.dispatcher, hooks, actionSpec, "update", action.ExecuteParams{
				Module: module, Entity: entity, ActionName: "update", ResourceID: id,
				Resource: rec.Data, WorkspaceID: workspaceID, UserID: updatedBy,
			})
			if entitySpec != nil {
				if emitted := action.ResolveEmission(entitySpec.Events, emitsOf(actionSpec), rec.Data); emitted != nil {
					action.DeliverEvents(ctx, f.deliveryDeps, workspaceID, module+"/"+entity, []action.EventEmission{*emitted})
				}
			}
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

		workspaceID := workspaceFromContext(ctx)
		id := r.PathValue("id")

		if err := store.SoftDelete(ctx, workspaceID, id); err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// parsePaginationParams parses and validates pagination query parameters.
// Spec §558-559: non-numeric or negative values → error for VALIDATION_ERROR (422).
func parsePaginationParams(r *http.Request) (page, perPage int, err error) {
	pageStr := r.URL.Query().Get("page")
	perPageStr := r.URL.Query().Get("per_page")

	page = 1
	if pageStr != "" {
		n, e := strconv.Atoi(pageStr)
		if e != nil || n < 0 {
			return 0, 0, fmt.Errorf("page: invalid value %q", pageStr)
		}
		if n > 0 {
			page = n
		}
	}

	perPage = 20
	if perPageStr != "" {
		n, e := strconv.Atoi(perPageStr)
		if e != nil || n < 0 {
			return 0, 0, fmt.Errorf("per_page: invalid value %q", perPageStr)
		}
		if n > 0 {
			perPage = n
		}
	}

	return page, perPage, nil
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
// Spec §16: list { data, meta: {page, per_page, total, total_pages}, links }
type ListResponse struct {
	Data  any       `json:"data"`
	Meta  MetaList  `json:"meta"`
	Links ListLinks `json:"links"`
}

// MetaList is metadata for list responses.
type MetaList struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// ListLinks contains pagination links per spec §16.
type ListLinks struct {
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
	Next  string `json:"next,omitempty"`
	Prev  string `json:"prev,omitempty"`
}

// ErrorResponse is the standard error envelope.
// Spec §16: error { error: {code, message, details}, meta }
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
	Meta  MetaSingle  `json:"meta"`
}

// ErrorDetail holds error information with optional structured details.
type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details []ErrorDetailItem `json:"details,omitempty"`
}

// ErrorDetailItem is a single structured error detail.
type ErrorDetailItem struct {
	Level   string `json:"level"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// --- Context helpers ---

type contextKey string

const (
	ctxWorkspaceID  contextKey = "tenant_id"
	ctxUserID       contextKey = "user_id"
	ctxIdentity     contextKey = "identity"
	ctxRequestID    contextKey = "request_id"
	ctxURLWorkspace contextKey = "url_workspace" // extracted from URL before identity override
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

// URLWorkspaceFromContext extracts the URL-original workspace (before identity override).
func URLWorkspaceFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxURLWorkspace).(string)
	return v
}

// WithURLWorkspace stores the URL-original workspace on the context.
func WithURLWorkspace(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, ctxURLWorkspace, workspaceID)
}

// workspaceFromContext extracts the workspace ID from the request context.
// Priority: Identity.WorkspaceID > context value > default "demo".
func workspaceFromContext(ctx context.Context) string {
	// Prefer workspace from identity (set by auth middleware)
	if id := IdentityFromContext(ctx); id != nil && id.WorkspaceID != "" {
		return id.WorkspaceID
	}
	v, _ := ctx.Value(ctxWorkspaceID).(string)
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

// WithWorkspace sets the workspace ID on the context.
func WithWorkspace(ctx context.Context, workspaceID string) context.Context {
	return context.WithValue(ctx, ctxWorkspaceID, workspaceID)
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

// writeErrorWithDetails writes an error with structured details.
func writeErrorWithDetails(w http.ResponseWriter, status int, code, message string, details []ErrorDetailItem) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{Code: code, Message: message, Details: details},
		Meta:  MetaSingle{Timestamp: time.Now().UTC().Format(time.RFC3339)},
	})
}

// buildListLinks constructs pagination links for list responses per spec §16.
func buildListLinks(u *url.URL, page, perPage, totalPages int) ListLinks {
	q := u.Query()
	links := ListLinks{}

	q.Set("page", "1")
	q.Set("per_page", fmt.Sprintf("%d", perPage))
	u.RawQuery = q.Encode()
	links.First = u.String()

	if totalPages > 0 {
		q.Set("page", fmt.Sprintf("%d", totalPages))
		u.RawQuery = q.Encode()
		links.Last = u.String()
	}

	if page < totalPages {
		q.Set("page", fmt.Sprintf("%d", page+1))
		u.RawQuery = q.Encode()
		links.Next = u.String()
	}

	if page > 1 {
		q.Set("page", fmt.Sprintf("%d", page-1))
		u.RawQuery = q.Encode()
		links.Prev = u.String()
	}

	return links
}

func isConflictError(err error) bool {
	s := err.Error()
	return contains(s, "version conflict") || contains(s, "not found")
}

// isValidationError reports whether err stems from a field/rule validation
// failure in the storage layer (required, rule, or immutable violations).
func isValidationError(err error) bool {
	return errors.Is(err, db.ErrValidationRule) ||
		errors.Is(err, db.ErrValidationRequired) ||
		errors.Is(err, db.ErrImmutableFieldChanged)
}

// writeStoreError maps a storage-layer error to the right HTTP status + code:
// validation → 422 VALIDATION_ERROR, version conflict → 409 CONFLICT,
// anything else → 500 INTERNAL_ERROR.
func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case isValidationError(err):
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
	case isConflictError(err):
		writeError(w, http.StatusConflict, "CONFLICT", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
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
// Spec §16: errors include structured details array with level, optional field, and message.
func writeValidationErrors(w http.ResponseWriter, errs []error) {
	msg := ""
	details := make([]ErrorDetailItem, 0, len(errs))
	for i, e := range errs {
		if i > 0 {
			msg += "; "
		}
		errStr := e.Error()
		msg += errStr
		details = append(details, ErrorDetailItem{
			Level:   "error",
			Message: errStr,
		})
	}
	writeErrorWithDetails(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", msg, details)
}

// HandleCustomAction returns a handler for a named custom action on an entity.
// It validates action params, evaluates conditions, dispatches to the impl executor,
// and handles the result (including state machine transitions).
func (f *HandlerFactory) HandleCustomAction(module, entity, actionName string, actionSpec spec.Action) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		workspaceID := workspaceFromContext(ctx)
		userID := userFromContext(ctx)
		resourceID := r.PathValue("id")

		// Parse request body as params
		var params map[string]any
		if r.Body != nil && r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
				writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid JSON: "+err.Error())
				return
			}
		}
		if params == nil {
			params = make(map[string]any)
		}

		// Validate action params if declared
		if actionSpec.Params != nil && len(actionSpec.Params.Validate) > 0 {
			if errs := validation.ValidateActionParams(params, actionSpec.Params.Validate); len(errs) > 0 {
				writeValidationErrors(w, errs)
				return
			}
		}

		// Load current resource data from the entity store
		var resourceData map[string]any
		var resourceVersion int
		store, err := f.registry.GetEntityStore(module, entity)
		if err == nil && resourceID != "" {
			rec, getErr := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: resourceID})
			if getErr == nil && rec != nil {
				resourceData = rec.Data
				resourceVersion = rec.Version
				if resourceData == nil {
					resourceData = make(map[string]any)
				}
			}
		}
		if resourceData == nil {
			resourceData = make(map[string]any)
		}

		// Evaluate action conditions (state-level validation, spec §13)
		if len(actionSpec.Conditions) > 0 {
			if err := action.EvaluateConditions(actionSpec.Conditions, resourceData, params); err != nil {
				writeError(w, http.StatusUnprocessableEntity, "CONDITION_FAILED", err.Error())
				return
			}
		}

		// Dispatch to the appropriate executor via the action dispatcher
		identity := IdentityFromContext(ctx)
		var identityInfo *action.IdentityInfo
		if identity != nil {
			identityInfo = &action.IdentityInfo{
				UserID:      identity.UserID,
				WorkspaceID: identity.WorkspaceID,
				Permissions: identity.Permissions,
				Roles:       identity.Roles,
			}
		}

		var entitySpec *spec.EntitySpec
		if f.specLookup != nil {
			entitySpec, _ = f.specLookup(module, entity)
		}
		var hooks []spec.HookDecl
		if entitySpec != nil {
			hooks = entitySpec.Hooks
		}

		execParams := action.ExecuteParams{
			Module:          module,
			Entity:          entity,
			ActionName:      actionName,
			ResourceID:      resourceID,
			Resource:        resourceData,
			ResourceVersion: resourceVersion,
			Params:          params,
			WorkspaceID:     workspaceID,
			UserID:          userID,
			Identity:        identityInfo,
		}

		// hooks: entries scoped to this action name run as a before-phase
		// around the dispatch below. actionSpec itself is NOT passed as the
		// "own impl" (unlike create/update) — Dispatch already executes it;
		// passing it here too would run it twice.
		if err := action.RunBeforePhase(ctx, f.dispatcher, hooks, nil, actionName, &execParams); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "HOOK_ABORTED", err.Error())
			return
		}

		result, err := f.dispatcher.Dispatch(ctx, actionSpec, execParams)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "ACTION_ERROR", err.Error())
			return
		}

		action.RunAfterPhase(ctx, f.dispatcher, hooks, nil, actionName, execParams)
		if entitySpec != nil {
			if emitted := action.ResolveEmission(entitySpec.Events, actionSpec.Emits, execParams.Resource); emitted != nil {
				action.DeliverEvents(ctx, f.deliveryDeps, workspaceID, module+"/"+entity, []action.EventEmission{*emitted})
			}
		}

		writeJSON(w, http.StatusOK, SingleResponse{
			Data: result.Data,
			Meta: MetaSingle{RequestID: requestIDFromContext(ctx), Timestamp: time.Now().UTC().Format(time.RFC3339)},
		})
	}
}
