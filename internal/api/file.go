package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Storage is the minimal object-store contract used by the file upload and
// download routes (todo 7.17.1). It mirrors the ctx.storage() primitive
// (Upload/Download) so any backend that implements it — filesystem, MinIO/S3,
// etc. — can be wired via SetStorageResolver.
type Storage interface {
	Upload(ctx context.Context, path string, data []byte) error
	Download(ctx context.Context, path string) ([]byte, error)
}

// Global size-limit defaults (todo 7.17.7). Overridable via
// FORMSPEC_UPLOAD_MAX_MB / FORMSPEC_DOWNLOAD_MAX_MB; per-field
// max_size_mb / max_download_mb lower the effective limit further.
const (
	DefaultUploadLimitMB   = 100
	DefaultDownloadLimitMB = 200

	// maxChunkPartBytes caps a single chunk part body (todo 7.17.5).
	maxChunkPartBytes = 64 << 20

	// DefaultLinkTTL is the validity window when a link is issued without
	// an explicit signed_url_ttl.
	DefaultLinkTTL = 15 * time.Minute

	// Link modes returned by the issue-link route: "token" = app-issued
	// link consumed via /storage/link/{token}; "presigned" = MinIO/S3
	// presigned URL served by the object store directly.
	linkModeToken     = "token"
	linkModePresigned = "presigned"
)

// storageCaps is the extended capability view of a resolved Storage. All
// capabilities are structural (interface assertions) so backends opt in
// per method — the same pattern internal/starlark uses.
type storageCaps struct {
	store Storage
	st    interface {
		Stat(ctx context.Context, path string) (int64, error)
	}
	del interface {
		Delete(ctx context.Context, path string) error
	}
	link interface {
		Link(ctx context.Context, path string, ttl time.Duration) (string, error)
	}
	chunk interface {
		InitChunkUpload(ctx context.Context, path string) (string, error)
		PutChunk(ctx context.Context, uploadID string, partNo int, data []byte) error
		CompleteChunkUpload(ctx context.Context, uploadID string) (string, error)
	}
}

// resolveStorage fetches the wired Storage and its optional capabilities.
func (f *HandlerFactory) resolveStorage() (*storageCaps, error) {
	if f.storage == nil {
		return nil, fmt.Errorf("storage not configured")
	}
	store, err := f.storage()
	if err != nil {
		return nil, err
	}
	caps := &storageCaps{store: store}
	if s, ok := store.(interface {
		Stat(ctx context.Context, path string) (int64, error)
	}); ok {
		caps.st = s
	}
	if s, ok := store.(interface {
		Delete(ctx context.Context, path string) error
	}); ok {
		caps.del = s
	}
	if s, ok := store.(interface {
		Link(ctx context.Context, path string, ttl time.Duration) (string, error)
	}); ok {
		caps.link = s
	}
	if s, ok := store.(interface {
		InitChunkUpload(ctx context.Context, path string) (string, error)
		PutChunk(ctx context.Context, uploadID string, partNo int, data []byte) error
		CompleteChunkUpload(ctx context.Context, uploadID string) (string, error)
	}); ok {
		caps.chunk = s
	}
	return caps, nil
}

// effectiveUploadLimitMB returns min(global upload limit, per-field limit).
func (f *HandlerFactory) effectiveUploadLimitMB(field *spec.Field) int {
	limit := f.uploadLimitMB
	if field != nil && field.Storage != nil && field.Storage.MaxSizeMB > 0 && field.Storage.MaxSizeMB < limit {
		limit = field.Storage.MaxSizeMB
	}
	return limit
}

// effectiveDownloadLimitMB returns min(global download limit, per-field limit).
func (f *HandlerFactory) effectiveDownloadLimitMB(field *spec.Field) int {
	limit := f.downloadLimitMB
	if field != nil && field.Storage != nil && field.Storage.MaxDownloadMB > 0 && field.Storage.MaxDownloadMB < limit {
		limit = field.Storage.MaxDownloadMB
	}
	return limit
}

// parseLinkTTL resolves the link validity window from a field's
// signed_url_ttl (Go duration string, e.g. "15m"), falling back to the
// default.
func parseLinkTTL(field *spec.Field) time.Duration {
	if field != nil && field.Storage != nil && field.Storage.SignedURLTTL != "" {
		if d, err := time.ParseDuration(field.Storage.SignedURLTTL); err == nil && d > 0 {
			return d
		}
	}
	return DefaultLinkTTL
}

// HandleFileUpload returns a POST /{module}/{entity}/{id}/{field} handler
// (todo 7.17.1). It stores an uploaded file in the object store and records
// the object key on the entity's file field. Permission = update on the
// entity; StorageSpec (allowed_types, max_size_mb) is enforced server-side.
func (f *HandlerFactory) HandleFileUpload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		workspaceID := workspaceFromContext(ctx)
		module := r.PathValue("module")
		entity := r.PathValue("entity")
		id := r.PathValue("id")
		fieldName := r.PathValue("field")

		if !f.can(ctx, module, entity, "update") {
			writeError(w, http.StatusForbidden, "FORBIDDEN",
				"missing permission: "+module+"."+entity+".update")
			return
		}

		store, err := f.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"entity not found: "+err.Error())
			return
		}

		es, ok := f.entitySpec(module, entity)
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"entity not found: "+module+"/"+entity)
			return
		}

		// The target field must exist and be a file/attachment type.
		var field *spec.Field
		for i := range es.Fields {
			if es.Fields[i].Name == fieldName {
				field = &es.Fields[i]
				break
			}
		}
		if field == nil || field.Type != spec.FieldFile {
			writeError(w, http.StatusBadRequest, "INVALID_FIELD",
				"field is not a file field: "+fieldName)
			return
		}

		// The record must exist so the object key can be attached to it.
		if _, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id}); err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"record not found: "+err.Error())
			return
		}

		// Parse multipart — a single file under the "file" part.
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST",
				"invalid multipart form: "+err.Error())
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST",
				"missing file part: "+err.Error())
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST",
				"read file: "+err.Error())
			return
		}

		// StorageSpec enforcement — server is the authority. Effective
		// limit = min(global upload limit, per-field max_size_mb) (7.17.7).
		if st := field.Storage; st != nil {
			if limitMB := f.effectiveUploadLimitMB(field); int64(len(data)) > int64(limitMB)*1024*1024 {
				writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
					fmt.Sprintf("file exceeds max_size_mb=%d", limitMB))
				return
			}
			if len(st.AllowedTypes) > 0 &&
				!allowedFileType(st.AllowedTypes, header.Header.Get("Content-Type"), header.Filename) {
				writeError(w, http.StatusBadRequest, "FILE_TYPE_NOT_ALLOWED",
					"file type not allowed for field "+fieldName)
				return
			}
		}

		// Object key: {workspace}/{module}/{entity}/{id}/{field}/{uuid}-{name}
		key := fmt.Sprintf("%s/%s/%s/%s/%s/%s-%s",
			workspaceID, module, entity, id, fieldName,
			db.NewUUIDv7(), sanitizeFilename(header.Filename))

		storage, err := f.storage()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "STORAGE_UNAVAILABLE",
				"storage not configured: "+err.Error())
			return
		}
		if err := storage.Upload(ctx, key, data); err != nil {
			writeError(w, http.StatusInternalServerError, "STORAGE_UPLOAD_FAILED",
				err.Error())
			return
		}

		// Attach the object key to the record's file field. When the field
		// declares max_count > 1, the field stores an ARRAY of keys (multi-file);
		// otherwise a single key string. max_count caps the array length
		// (todo 7.17.2).
		if field.Storage != nil && field.Storage.MaxCount > 1 {
			rec, getErr := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
			if getErr != nil {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"record not found: "+getErr.Error())
				return
			}
			var keys []string
			if existing, ok := rec.Data[fieldName].([]any); ok {
				for _, k := range existing {
					if s, ok := k.(string); ok {
						keys = append(keys, s)
					}
				}
			} else if s, ok := rec.Data[fieldName].(string); ok && s != "" {
				keys = append(keys, s)
			}
			if len(keys) >= field.Storage.MaxCount {
				writeError(w, http.StatusBadRequest, "FILE_COUNT_EXCEEDED",
					fmt.Sprintf("field %s already has max_count=%d files", fieldName, field.Storage.MaxCount))
				return
			}
			keys = append(keys, key)
			if err := store.UpdateFields(ctx, workspaceID, id, map[string]any{fieldName: keys}); err != nil {
				writeError(w, http.StatusInternalServerError, "UPDATE_FAILED",
					err.Error())
				return
			}
		} else {
			if err := store.UpdateFields(ctx, workspaceID, id, map[string]any{fieldName: key}); err != nil {
				writeError(w, http.StatusInternalServerError, "UPDATE_FAILED",
					err.Error())
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"key":  key,
			"name": header.Filename,
			"size": len(data),
		})
	}
}

// HandleFileDownload returns a GET /{module}/{entity}/{id}/{field} handler
// (todo 7.17.1). It streams the stored object back with a content type
// derived from the object key. Permission = view on the entity, unless the
// field's StorageSpec declares `visibility: public` (anonymous read allowed,
// todo 7.17.2). `visibility: signed` requires a valid link token via
// ?link_token= (issued by HandleLinkIssue, todo 7.17.6) — session auth is
// replaced by the unguessable token. Downloads over the effective size limit
// (min(global, per-field max_download_mb)) are rejected 413 without loading
// the object (todo 7.17.7).
func (f *HandlerFactory) HandleFileDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		workspaceID := workspaceFromContext(ctx)
		module := r.PathValue("module")
		entity := r.PathValue("entity")
		id := r.PathValue("id")
		fieldName := r.PathValue("field")

		// Resolve the field's StorageSpec to determine visibility (todo 7.17.2).
		var field *spec.Field
		if es, ok := f.entitySpec(module, entity); ok {
			for i := range es.Fields {
				if es.Fields[i].Name == fieldName {
					field = &es.Fields[i]
					break
				}
			}
		}
		visibility := "private" // default
		if field != nil && field.Storage != nil && field.Storage.Visibility != "" {
			visibility = field.Storage.Visibility
		}

		var key string
		var deleteAfterServe bool
		switch visibility {
		case "public":
			// Anonymous read allowed — no permission check.
		case "signed":
			// Link-token access (todo 7.17.6): the token IS the credential.
			if f.linkStore == nil {
				writeError(w, http.StatusNotImplemented, "LINK_STORE_UNAVAILABLE",
					"link store not configured")
				return
			}
			token := r.URL.Query().Get("link_token")
			if token == "" {
				writeError(w, http.StatusUnauthorized, "LINK_REQUIRED",
					"visibility: signed requires ?link_token= (obtain via POST .../link)")
				return
			}
			row, deleteNow, err := f.linkStore.Consume(ctx, token)
			if err != nil {
				writeError(w, http.StatusGone, "LINK_INVALID",
					"link invalid, expired, or exhausted")
				return
			}
			// Tenant isolation: the link must belong to this workspace.
			if !strings.HasPrefix(row.Path, workspaceID+"/") {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "file not found")
				return
			}
			key = row.Path
			deleteAfterServe = deleteNow
		default: // "private" or unset
			if !f.can(ctx, module, entity, "view") {
				writeError(w, http.StatusForbidden, "FORBIDDEN",
					"missing permission: "+module+"."+entity+".view")
				return
			}
		}

		store, err := f.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"entity not found: "+err.Error())
			return
		}

		if key == "" {
			rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
			if err != nil {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"record not found: "+err.Error())
				return
			}

			// Resolve the object key. A multi-file field (max_count > 1) stores
			// an array of keys; the client selects one via ?index=N (default 0).
			switch v := rec.Data[fieldName].(type) {
			case string:
				key = v
			case []any:
				idx := 0
				if s := r.URL.Query().Get("index"); s != "" {
					if n, err := strconv.Atoi(s); err == nil {
						idx = n
					}
				}
				if idx >= 0 && idx < len(v) {
					if s, ok := v[idx].(string); ok {
						key = s
					}
				}
			}
			if key == "" {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"no file on field "+fieldName)
				return
			}
		}

		caps, err := f.resolveStorage()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "STORAGE_UNAVAILABLE",
				"storage not configured: "+err.Error())
			return
		}

		// Download size limit (todo 7.17.7): check the object size BEFORE
		// loading it — an over-limit object never enters memory.
		if caps.st != nil {
			size, err := caps.st.Stat(ctx, key)
			if err != nil {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"file not found: "+err.Error())
				return
			}
			if limitMB := f.effectiveDownloadLimitMB(field); size > int64(limitMB)*1024*1024 {
				writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
					fmt.Sprintf("file exceeds download limit max_download_mb=%d", limitMB))
				return
			}
		}

		data, err := caps.store.Download(ctx, key)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"file not found: "+err.Error())
			return
		}

		w.Header().Set("Content-Type", contentTypeFor(key))
		w.Header().Set("Content-Disposition",
			`inline; filename="`+filepath.Base(key)+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

		// One-time download (todo 7.17.6): the link budget was exhausted by
		// this download — remove the object after serving.
		if deleteAfterServe && caps.del != nil {
			_ = caps.del.Delete(ctx, key)
		}
	}
}

// can reports whether the request identity holds {module}.{entity}.{action}.
func (f *HandlerFactory) can(ctx context.Context, module, entity, action string) bool {
	identity := IdentityFromContext(ctx)
	if identity == nil {
		return false
	}
	return identity.HasPermission(module + "." + entity + "." + action)
}

// entitySpec resolves the entity spec via the wired spec lookup.
func (f *HandlerFactory) entitySpec(module, entity string) (*spec.EntitySpec, bool) {
	if f.specLookup == nil {
		return nil, false
	}
	return f.specLookup(module, entity)
}

// sanitizeFilename keeps only safe filename characters (no path traversal).
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, name)
	if name == "" || name == "." {
		return "file"
	}
	return name
}

// allowedFileType matches an allowed_types entry against the MIME type and
// file extension. Entries may be extensions (".pdf") or MIME types with
// optional wildcard ("image/*").
func allowedFileType(allowed []string, contentType, filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, ".") {
			if ext == a {
				return true
			}
			continue
		}
		if a == contentType {
			return true
		}
		if strings.HasSuffix(a, "/*") && strings.HasPrefix(contentType, strings.TrimSuffix(a, "*")) {
			return true
		}
	}
	return false
}

// ─── Link routes (todo 7.17.6) ───

// consumeURLFor builds the workspace-scoped consume URL for an app-token
// link, mirroring the surface the link was issued from (/_ui or /api/v1).
func consumeURLFor(r *http.Request, token string) string {
	p := r.URL.Path
	if i := strings.Index(p, "/_ui/entity/"); i >= 0 {
		return p[:i] + "/_ui/storage/link/" + token
	}
	if i := strings.Index(p, "/api/v1/"); i >= 0 {
		return p[:i] + "/api/v1/storage/link/" + token
	}
	return "/storage/link/" + token
}

// HandleLinkIssue returns a POST /{module}/{entity}/{id}/{field}/link handler
// (todo 7.17.6). It resolves the record's object key, enforces the download
// size limit, then issues either a presigned URL (`visibility: signed` with a
// Linker-capable backend) or an app-token link consumed via
// GET .../storage/link/{token}. One-time fields (one_time: true) get a
// single-download link that deletes the object after serving.
func (f *HandlerFactory) HandleLinkIssue() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		workspaceID := workspaceFromContext(ctx)
		module := r.PathValue("module")
		entity := r.PathValue("entity")
		id := r.PathValue("id")
		fieldName := r.PathValue("field")

		var field *spec.Field
		if es, ok := f.entitySpec(module, entity); ok {
			for i := range es.Fields {
				if es.Fields[i].Name == fieldName {
					field = &es.Fields[i]
					break
				}
			}
		}
		if field == nil || field.Type != spec.FieldFile {
			writeError(w, http.StatusBadRequest, "INVALID_FIELD",
				"field is not a file field: "+fieldName)
			return
		}
		visibility := "private"
		if field.Storage != nil && field.Storage.Visibility != "" {
			visibility = field.Storage.Visibility
		}

		// Issue-link requires view permission on private fields (public
		// fields are anonymous-readable; signed fields gate on the token
		// itself, but issuing still requires view).
		if visibility != "public" && !f.can(ctx, module, entity, "view") {
			writeError(w, http.StatusForbidden, "FORBIDDEN",
				"missing permission: "+module+"."+entity+".view")
			return
		}

		store, err := f.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"entity not found: "+err.Error())
			return
		}
		rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"record not found: "+err.Error())
			return
		}
		key := fileFieldValue(rec.Data[fieldName])
		if key == "" {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"no file on field "+fieldName)
			return
		}

		caps, err := f.resolveStorage()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "STORAGE_UNAVAILABLE",
				"storage not configured: "+err.Error())
			return
		}

		// Size limit (todo 7.17.7): reject before handing out any URL —
		// a presigned link bypasses the app, so this is the only gate.
		if caps.st != nil {
			size, err := caps.st.Stat(ctx, key)
			if err != nil {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"file not found: "+err.Error())
				return
			}
			if limitMB := f.effectiveDownloadLimitMB(field); size > int64(limitMB)*1024*1024 {
				writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
					fmt.Sprintf("file exceeds download limit max_download_mb=%d", limitMB))
				return
			}
		}

		ttl := parseLinkTTL(field)
		oneTime := field.Storage != nil && field.Storage.OneTime

		// Presigned path: `visibility: signed` + Linker-capable backend.
		if visibility == "signed" && caps.link != nil {
			url, err := caps.link.Link(ctx, key, ttl)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "LINK_FAILED",
					err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"url":        url,
				"mode":       linkModePresigned,
				"expires_at": time.Now().UTC().Add(ttl).Format(time.RFC3339),
			})
			return
		}

		// App-token path. Requires the link store.
		if f.linkStore == nil {
			writeError(w, http.StatusNotImplemented, "LINK_STORE_UNAVAILABLE",
				"link store not configured")
			return
		}
		// TTL field (delete_if_untouched): the sweeper removes the object
		// when the link expires without a download (todo 7.17.6).
		deleteIfUntouched := field.Storage != nil && field.Storage.TTL != ""
		maxDownloads := 0 // unlimited
		if oneTime {
			maxDownloads = 1
		}
		row, err := f.linkStore.Issue(ctx, workspaceID, key, ttl,
			maxDownloads, oneTime, deleteIfUntouched)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "LINK_FAILED",
				err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"url":           consumeURLFor(r, row.Token),
			"mode":          linkModeToken,
			"expires_at":    row.ExpiresAt,
			"max_downloads": maxDownloads,
		})
	}
}

// HandleLinkConsume returns a GET .../storage/link/{token} handler (todo
// 7.17.6). The token is the credential — no session required. Consumption
// is atomic (count/budget/expiry enforced in one UPDATE); an exhausted
// one-time link removes the object after serving.
func (f *HandlerFactory) HandleLinkConsume() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if f.linkStore == nil {
			writeError(w, http.StatusNotImplemented, "LINK_STORE_UNAVAILABLE",
				"link store not configured")
			return
		}
		token := r.PathValue("token")
		if token == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing link token")
			return
		}
		row, deleteNow, err := f.linkStore.Consume(ctx, token)
		if err != nil {
			writeError(w, http.StatusGone, "LINK_INVALID",
				"link invalid, expired, or exhausted")
			return
		}

		caps, err := f.resolveStorage()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "STORAGE_UNAVAILABLE",
				"storage not configured: "+err.Error())
			return
		}
		data, err := caps.store.Download(ctx, row.Path)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"file not found: "+err.Error())
			return
		}

		w.Header().Set("Content-Type", contentTypeFor(row.Path))
		w.Header().Set("Content-Disposition",
			`attachment; filename="`+filepath.Base(row.Path)+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)

		// One-time download exhausted — remove the object (delete-on-download).
		if deleteNow && caps.del != nil {
			_ = caps.del.Delete(ctx, row.Path)
		}
	}
}

// ─── Chunked upload routes (todo 7.17.5) ───

// HandleChunkInit returns a POST /{module}/{entity}/{id}/{field}/upload/init
// handler (todo 7.17.5). Body: {"filename": "..."} → {"upload_id",
// "chunk_size_mb"}. The object key is computed up front so backends that
// need the final path at init (S3 multipart) can bind it.
func (f *HandlerFactory) HandleChunkInit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		workspaceID := workspaceFromContext(ctx)
		module := r.PathValue("module")
		entity := r.PathValue("entity")
		id := r.PathValue("id")
		fieldName := r.PathValue("field")

		if !f.can(ctx, module, entity, "update") {
			writeError(w, http.StatusForbidden, "FORBIDDEN",
				"missing permission: "+module+"."+entity+".update")
			return
		}
		var field *spec.Field
		if es, ok := f.entitySpec(module, entity); ok {
			for i := range es.Fields {
				if es.Fields[i].Name == fieldName {
					field = &es.Fields[i]
					break
				}
			}
		}
		if field == nil || field.Type != spec.FieldFile {
			writeError(w, http.StatusBadRequest, "INVALID_FIELD",
				"field is not a file field: "+fieldName)
			return
		}

		var body struct {
			Filename string `json:"filename"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST",
				"invalid JSON body: "+err.Error())
			return
		}
		// allowed_types enforcement at init — fail fast before any bytes
		// are transferred.
		if st := field.Storage; st != nil && len(st.AllowedTypes) > 0 {
			ct := body.Filename // no content-type available yet; extension match
			if !allowedFileType(st.AllowedTypes, "", ct) {
				writeError(w, http.StatusBadRequest, "FILE_TYPE_NOT_ALLOWED",
					"file type not allowed for field "+fieldName)
				return
			}
		}

		caps, err := f.resolveStorage()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "STORAGE_UNAVAILABLE",
				"storage not configured: "+err.Error())
			return
		}
		if caps.chunk == nil {
			writeError(w, http.StatusNotImplemented, "CHUNKING_NOT_SUPPORTED",
				"storage backend does not support chunked upload")
			return
		}

		// Same key layout as HandleFileUpload.
		key := fmt.Sprintf("%s/%s/%s/%s/%s/%s-%s",
			workspaceID, module, entity, id, fieldName,
			db.NewUUIDv7(), sanitizeFilename(body.Filename))
		uploadID, err := caps.chunk.InitChunkUpload(ctx, key)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "CHUNK_INIT_FAILED",
				err.Error())
			return
		}
		chunkSize := 8
		if field.Storage != nil && field.Storage.ChunkSizeMB > 0 {
			chunkSize = field.Storage.ChunkSizeMB
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"upload_id":     uploadID,
			"key":           key,
			"chunk_size_mb": chunkSize,
			"max_size_mb":   f.effectiveUploadLimitMB(field),
		})
	}
}

// HandleChunkPart returns a POST .../upload/{uid}/part/{part} handler (todo
// 7.17.5). The body is the raw part bytes (application/octet-stream).
func (f *HandlerFactory) HandleChunkPart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		module := r.PathValue("module")
		entity := r.PathValue("entity")

		if !f.can(ctx, module, entity, "update") {
			writeError(w, http.StatusForbidden, "FORBIDDEN",
				"missing permission: "+module+"."+entity+".update")
			return
		}
		uploadID := r.PathValue("uid")
		partNo, err := strconv.Atoi(r.PathValue("part"))
		if err != nil || partNo < 0 || partNo > 10000 {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid part number")
			return
		}

		caps, err := f.resolveStorage()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "STORAGE_UNAVAILABLE",
				"storage not configured: "+err.Error())
			return
		}
		if caps.chunk == nil {
			writeError(w, http.StatusNotImplemented, "CHUNKING_NOT_SUPPORTED",
				"storage backend does not support chunked upload")
			return
		}
		data, err := io.ReadAll(io.LimitReader(r.Body, maxChunkPartBytes+1))
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST",
				"read part body: "+err.Error())
			return
		}
		if len(data) > maxChunkPartBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "PART_TOO_LARGE",
				fmt.Sprintf("part exceeds %d bytes", maxChunkPartBytes))
			return
		}
		if err := caps.chunk.PutChunk(ctx, uploadID, partNo, data); err != nil {
			writeError(w, http.StatusBadRequest, "CHUNK_PART_FAILED",
				err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"part":     partNo,
			"size":     len(data),
			"uploaded": true,
		})
	}
}

// HandleChunkComplete returns a POST .../upload/{uid}/complete handler (todo
// 7.17.5). It assembles the object, enforces the effective size limit (the
// object is deleted when over-limit), attaches the key to the record's file
// field, and returns {"key", "size"}.
func (f *HandlerFactory) HandleChunkComplete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		workspaceID := workspaceFromContext(ctx)
		module := r.PathValue("module")
		entity := r.PathValue("entity")
		id := r.PathValue("id")
		fieldName := r.PathValue("field")

		if !f.can(ctx, module, entity, "update") {
			writeError(w, http.StatusForbidden, "FORBIDDEN",
				"missing permission: "+module+"."+entity+".update")
			return
		}
		var field *spec.Field
		if es, ok := f.entitySpec(module, entity); ok {
			for i := range es.Fields {
				if es.Fields[i].Name == fieldName {
					field = &es.Fields[i]
					break
				}
			}
		}
		if field == nil || field.Type != spec.FieldFile {
			writeError(w, http.StatusBadRequest, "INVALID_FIELD",
				"field is not a file field: "+fieldName)
			return
		}

		caps, err := f.resolveStorage()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "STORAGE_UNAVAILABLE",
				"storage not configured: "+err.Error())
			return
		}
		if caps.chunk == nil {
			writeError(w, http.StatusNotImplemented, "CHUNKING_NOT_SUPPORTED",
				"storage backend does not support chunked upload")
			return
		}
		uploadID := r.PathValue("uid")
		key, err := caps.chunk.CompleteChunkUpload(ctx, uploadID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "CHUNK_COMPLETE_FAILED",
				err.Error())
			return
		}

		// Size limit on the assembled object (todo 7.17.7) — the object is
		// removed when over-limit so partial abuse doesn't persist.
		var size int64
		if caps.st != nil {
			size, err = caps.st.Stat(ctx, key)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "STORAGE_STAT_FAILED",
					err.Error())
				return
			}
			if limitMB := f.effectiveUploadLimitMB(field); size > int64(limitMB)*1024*1024 {
				if caps.del != nil {
					_ = caps.del.Delete(ctx, key)
				}
				writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
					fmt.Sprintf("assembled file exceeds max_size_mb=%d", limitMB))
				return
			}
		}

		// Attach the object key to the record's file field (same max_count
		// semantics as HandleFileUpload, todo 7.17.2).
		store, err := f.registry.GetEntityStore(module, entity)
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"entity not found: "+err.Error())
			return
		}
		if field.Storage != nil && field.Storage.MaxCount > 1 {
			rec, getErr := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
			if getErr != nil {
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"record not found: "+getErr.Error())
				return
			}
			var keys []string
			if existing, ok := rec.Data[fieldName].([]any); ok {
				for _, k := range existing {
					if s, ok := k.(string); ok {
						keys = append(keys, s)
					}
				}
			} else if s, ok := rec.Data[fieldName].(string); ok && s != "" {
				keys = append(keys, s)
			}
			if len(keys) >= field.Storage.MaxCount {
				if caps.del != nil {
					_ = caps.del.Delete(ctx, key)
				}
				writeError(w, http.StatusBadRequest, "FILE_COUNT_EXCEEDED",
					fmt.Sprintf("field %s already has max_count=%d files", fieldName, field.Storage.MaxCount))
				return
			}
			keys = append(keys, key)
			if err := store.UpdateFields(ctx, workspaceID, id, map[string]any{fieldName: keys}); err != nil {
				writeError(w, http.StatusInternalServerError, "UPDATE_FAILED",
					err.Error())
				return
			}
		} else {
			if err := store.UpdateFields(ctx, workspaceID, id, map[string]any{fieldName: key}); err != nil {
				writeError(w, http.StatusInternalServerError, "UPDATE_FAILED",
					err.Error())
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"key":  key,
			"size": size,
		})
	}
}

// fileFieldValue extracts the single object key from a file field value
// (string, or the first element of a multi-file array).
func fileFieldValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		for _, k := range t {
			if s, ok := k.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// contentTypeFor maps a stored object key's extension to a content type.
func contentTypeFor(key string) string {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
