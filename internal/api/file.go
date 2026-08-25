package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

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

		// StorageSpec enforcement — server is the authority.
		if st := field.Storage; st != nil {
			if st.MaxSizeMB > 0 && len(data) > st.MaxSizeMB*1024*1024 {
				writeError(w, http.StatusBadRequest, "FILE_TOO_LARGE",
					fmt.Sprintf("file exceeds max_size_mb=%d", st.MaxSizeMB))
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
// todo 7.17.2). `visibility: signed` requires URL-signing infra (deferred).
func (f *HandlerFactory) HandleFileDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		workspaceID := workspaceFromContext(ctx)
		module := r.PathValue("module")
		entity := r.PathValue("entity")
		id := r.PathValue("id")
		fieldName := r.PathValue("field")

		// Resolve the field's StorageSpec to determine visibility (todo 7.17.2).
		visibility := "private" // default
		if es, ok := f.entitySpec(module, entity); ok {
			for i := range es.Fields {
				if es.Fields[i].Name == fieldName && es.Fields[i].Storage != nil {
					if v := es.Fields[i].Storage.Visibility; v != "" {
						visibility = v
					}
					break
				}
			}
		}

		switch visibility {
		case "public":
			// Anonymous read allowed — no permission check.
		case "signed":
			// Signed URLs require URL-signing infra (deferred, todo 7.17.2).
			writeError(w, http.StatusNotImplemented, "SIGNED_URL_NOT_IMPLEMENTED",
				"visibility: signed is not yet implemented")
			return
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

		rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: workspaceID, ID: id})
		if err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"record not found: "+err.Error())
			return
		}

		// Resolve the object key. A multi-file field (max_count > 1) stores an
		// array of keys; the client selects one via ?index=N (default 0).
		key := ""
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

		storage, err := f.storage()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "STORAGE_UNAVAILABLE",
				"storage not configured: "+err.Error())
			return
		}
		data, err := storage.Download(ctx, key)
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
