// Seed assets — `$asset` values uploaded through the storage service.
//
// Why this file exists: a `file` field stores an OBJECT KEY, and the canonical
// key embeds the record ID, which does not exist before insert. Until now the
// kafe seed worked around that with `make seed-kafe-assets`, which `cp`-ed the
// sample photos straight into `.formspec/storage/seed/menu/`. That workaround
// had three problems, all of them visible to a user:
//
//   - it bypassed the storage service entirely, so it had no production
//     counterpart (prod runs garage/minio/s3, not a local directory) — the
//     "seeded menu with pictures" flow only ever worked in dev;
//   - it used a fabricated key (`seed/menu/<file>`) instead of the canonical
//     `{workspace}/{module}/{entity}/{id}/{field}/{uuid}-{name}`, so nothing
//     could verify that a file field pointed at a real object;
//   - `rm -rf .formspec/` silently broke every menu photo until someone
//     remembered to re-run the copy.
//
// The fix is to upload AFTER the row exists — at which point the record ID is
// known — through the SAME storage service and the SAME key shape the HTTP
// upload route uses (internal/api/file.go). See
// docs_internal/plan/seed-assets-and-reconcile.md §2.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// assetMarker is the single key of an asset reference in a seed record:
//
//	photo: { $asset: "menu/sate-ayam.jpg" }
//
// It is deliberately an explicit marker, like `$ref`, rather than "anything
// that looks like a path". A `file` field's value IS an object key, and a key
// can legitimately be written literally in a seed
// (resource/storage_reload_e2e_test.go does exactly that). Guessing from the
// value would make "this is an asset to upload" and "this is already a key"
// indistinguishable, and a wrong guess is silent: it either uploads something
// meant as a literal key or stores a path that will never resolve.
const assetMarker = "$asset"

// storageResolver returns the object store to upload into. It is a function so
// resolution is lazy: a seed with no `$asset` never resolves storage, and
// therefore never fails on a deployment where none is configured.
type storageResolver func() (api.Storage, error)

// assetUploader resolves `$asset` markers to files and uploads them.
//
// Module directories are discovered by walking up from the manifest Source
// paths the loader recorded, looking for the `module.yaml` whose metadata.name
// matches. That follows the manifest rather than a hardcoded layout, matching
// the loader's own zero-folder-assumption contract
// (docs/spec/platform/08-project-layout.md §1.2), and it keeps working when a
// module is vendored to a different location.
type assetUploader struct {
	workspaceID string
	storage     storageResolver
	globalLimit int

	mu        sync.Mutex
	roots     []string          // distinct manifest directories, longest first
	moduleDir map[string]string // module name → assets dir ("" = not found)
}

func newAssetUploader(res *manifest.LoadResult, workspaceID string, storage storageResolver, globalLimit int) *assetUploader {
	u := &assetUploader{
		workspaceID: workspaceID,
		storage:     storage,
		globalLimit: globalLimit,
		moduleDir:   map[string]string{},
	}
	seen := map[string]bool{}
	var roots []string
	for _, m := range res.Manifests {
		// Source is "<path>#<docIndex>" (internal/manifest/loader.go).
		path, _, _ := strings.Cut(m.Source, "#")
		if path == "" {
			continue
		}
		// Every manifest directory is a candidate module root, plus each
		// ancestor — module.yaml sits above the characteristic subfolders
		// (`<module>/master/menu-item/entity.yaml`).
		for dir := filepath.Dir(path); dir != "" && dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
			if seen[dir] {
				break // ancestors already recorded
			}
			seen[dir] = true
			roots = append(roots, dir)
		}
	}
	// Longest path first: a module nested inside another module must win.
	sortByLengthDesc(roots)
	u.roots = roots
	return u
}

func sortByLengthDesc(paths []string) {
	for i := 1; i < len(paths); i++ {
		for j := i; j > 0 && len(paths[j]) > len(paths[j-1]); j-- {
			paths[j], paths[j-1] = paths[j-1], paths[j]
		}
	}
}

// moduleAssetsDir returns `<module-dir>/assets` for a module, or "" when no
// module directory could be identified. A miss is cached: re-checking every
// module for every record of a large seed is pure repetition.
func (u *assetUploader) moduleAssetsDir(module string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if dir, ok := u.moduleDir[module]; ok {
		return dir
	}
	dir := ""
	// roots are longest-first, so the deepest matching module directory wins.
	for _, root := range u.roots {
		if isModuleDir(root, module) {
			dir = filepath.Join(root, "assets")
			break
		}
	}
	u.moduleDir[module] = dir
	return dir
}

// isModuleDir reports whether dir holds a `module.yaml` for the named module.
// The manifest's declared name decides — not the directory name — so a vendored
// or renamed directory still resolves.
func isModuleDir(dir, module string) bool {
	for _, name := range []string{"module.yaml", "module.yml"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		docs, errs := manifest.NewLoader(".").ParseBytes(data, name)
		if len(errs) > 0 || len(docs) == 0 {
			continue
		}
		if string(spec.Kind(docs[0].Kind)) != string(spec.KindModule) {
			continue
		}
		if docs[0].Metadata.Name == module {
			return true
		}
	}
	return false
}

// readAsset loads an asset file, reporting the directory it looked in. A bare
// "file not found" would leave the author guessing which root was used.
func (u *assetUploader) readAsset(module, relPath, field string) ([]byte, error) {
	base := u.moduleAssetsDir(module)
	if base == "" {
		return nil, fmt.Errorf("field %q: module %q: no module.yaml found under the spec root, so `assets/%s` cannot be located",
			field, module, relPath)
	}
	full := filepath.Join(base, filepath.FromSlash(relPath))
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("field %q: asset %q: %w (looked in %s)", field, relPath, err, base)
	}
	return data, nil
}

// pendingAsset is one `$asset` value waiting for the record to exist.
type pendingAsset struct {
	field   string
	relPath string // as written in the manifest (for messages)
	data    []byte
}

// resolveAssets walks a record, replaces every `$asset` marker with a cleared
// field, and returns the assets to upload once the record has an ID.
//
// The two-step shape is the whole point: the canonical key embeds the record
// ID, which is minted by the INSERT. Uploading before it would produce keys
// pointing at a row that a validation failure may never have created.
func (u *assetUploader) resolveAssets(module string, es *spec.EntitySpec, rec map[string]any) (map[string]any, []pendingAsset, error) {
	out := make(map[string]any, len(rec))
	var pending []pendingAsset
	for k, v := range rec {
		m, ok := v.(map[string]any)
		if !ok {
			out[k] = v
			continue
		}
		raw, isAsset := m[assetMarker]
		if !isAsset || len(m) != 1 {
			// Not a marker: recurse, because a child payload can carry its own
			// file field.
			inner, more, err := u.resolveAssets(module, es, m)
			if err != nil {
				return rec, nil, err
			}
			pending = append(pending, more...)
			out[k] = inner
			continue
		}
		relPath, ok := raw.(string)
		if !ok {
			return rec, nil, fmt.Errorf("field %q: `%s` must be a string path, got %T", k, assetMarker, raw)
		}
		field := findField(es, k)
		if field == nil {
			return rec, nil, fmt.Errorf("field %q: unknown field — `%s` needs a declared field to validate against", k, assetMarker)
		}
		if field.Type != spec.FieldFile && field.Type != spec.FieldAttachment {
			return rec, nil, fmt.Errorf("field %q: `%s` is only valid on a `file` field, this one is %q", k, assetMarker, field.Type)
		}
		data, err := u.readAsset(module, relPath, k)
		if err != nil {
			return rec, nil, err
		}
		// Same enforcement as the upload route, so a seed cannot write an
		// object the API would have rejected.
		if st := field.Storage; st != nil {
			if limit := api.MinUploadLimitMB(u.globalLimit, field); int64(len(data)) > int64(limit)*1024*1024 {
				return rec, nil, fmt.Errorf("field %q: asset %q is %d bytes, over max_size_mb=%d",
					k, relPath, len(data), limit)
			}
			if len(st.AllowedTypes) > 0 && !api.AllowedFileType(st.AllowedTypes, "", filepath.Base(relPath)) {
				return rec, nil, fmt.Errorf("field %q: asset %q does not match allowed_types %v",
					k, relPath, st.AllowedTypes)
			}
		}
		out[k] = nil // cleared; the key is written after the upload
		pending = append(pending, pendingAsset{field: k, relPath: relPath, data: data})
	}
	return out, pending, nil
}

// upload writes one record's pending assets and returns the field updates that
// point the record at them. Storage is resolved here (not in resolveAssets) so
// a seed whose assets all fail validation never opens a connection.
func (u *assetUploader) upload(ctx context.Context, module, entity, id string, pending []pendingAsset) (map[string]any, error) {
	updates := map[string]any{}
	if len(pending) == 0 {
		return updates, nil
	}
	storage, err := u.storage()
	if err != nil {
		return nil, fmt.Errorf("storage service: %w", err)
	}
	for _, p := range pending {
		key := api.ObjectKey(u.workspaceID, module, entity, id, p.field, filepath.Base(p.relPath))
		if err := storage.Upload(ctx, key, p.data); err != nil {
			return updates, fmt.Errorf("field %q: upload %q: %w", p.field, p.relPath, err)
		}
		updates[p.field] = key
	}
	return updates, nil
}

// assetSource returns the asset path declared for a file field, if any. It is
// how reconcile learns that a record's photo has a source file to restore — a
// hand-written literal key does not.
func assetSource(rec map[string]any, field string) (string, bool) {
	m, ok := rec[field].(map[string]any)
	if !ok {
		return "", false
	}
	raw, ok := m[assetMarker]
	if !ok || len(m) != 1 {
		return "", false
	}
	path, _ := raw.(string)
	return path, path != ""
}

// findField looks up a field by name in an entity spec.
func findField(es *spec.EntitySpec, name string) *spec.Field {
	if es == nil {
		return nil
	}
	for i := range es.Fields {
		if es.Fields[i].Name == name {
			return &es.Fields[i]
		}
	}
	return nil
}
