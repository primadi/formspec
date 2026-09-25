package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"
)

// assetSeedSpec writes a module with one entity carrying a file field, one
// asset file, and a seed that references it with `$asset`.
//
// The module lives at <dir>/modules/alpha/ so that the uploader's
// module-directory discovery (walk up from the manifest Source path, find the
// module.yaml whose metadata.name matches) is exercised the same way a real
// spec tree exercises it.
func assetSeedSpec(t *testing.T, dir string) {
	t.Helper()
	modDir := filepath.Join(dir, "modules", "alpha")
	if err := os.MkdirAll(filepath.Join(modDir, "master", "product"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(modDir, "module.yaml"), `apiVersion: formspec.dev/v1
kind: Module
metadata: { name: alpha, description: "test module" }
spec: { version: 1.0.0 }
`)
	write(t, filepath.Join(modDir, "master", "product", "entity.yaml"), `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: product, module: alpha }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string, natural_key: true }
    - { name: name, type: string }
    - name: photo
      type: file
      storage: { allowed_types: [jpg, png], max_size_mb: 2, max_count: 1, visibility: public }
`)
	// Minimal but real JPEG bytes (SOI + APP0 + EOI); the seed path does not
	// parse images, it only reads and uploads them.
	if err := os.MkdirAll(filepath.Join(modDir, "assets", "products"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(modDir, "assets", "products", "widget.jpg"), jpegBytes)
	write(t, filepath.Join(dir, "seed.yaml"), `apiVersion: formspec.dev/v1
kind: Seed
metadata: { name: demo, module: alpha }
spec:
  entities:
    - entity: product
      records:
        - code: P-1
          name: "Widget"
          photo: { $asset: "products/widget.jpg" }
`)
}

var jpegBytes = string([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9})

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedHarness builds a registry + loader + uploader backed by a temp store.
func seedHarness(t *testing.T, dir, storageRoot string) (*manifest.LoadResult, *entity.Registry, *assetUploader) {
	t.Helper()
	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	for _, loadErr := range reg.LoadEntities() {
		t.Fatalf("load entity: %v", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}
	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}
	store, err := memory.NewStorage(storageRoot)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	return res, reg, newAssetUploader(res, "demo", func() (api.Storage, error) {
		return store, nil
	}, 100)
}

// TestSeedAssetUploadsThroughStorage pins the core of the `$asset` contract: the
// seed writes the CANONICAL object key, the bytes are in the storage service,
// and the key is the same shape the HTTP upload route produces — so the
// download route and `visibility` work with no special case.
func TestSeedAssetUploadsThroughStorage(t *testing.T) {
	dir := t.TempDir()
	assetSeedSpec(t, dir)
	storageRoot := filepath.Join(t.TempDir(), "storage")

	res, reg, up := seedHarness(t, dir, storageRoot)

	inserted, updated, skipped, failed := seedAllWith(context.Background(), res, reg, "", "demo", up)
	if failed != 0 {
		t.Fatalf("expected 0 failed, got %d (inserted=%d updated=%d skipped=%d)", failed, inserted, updated, skipped)
	}
	if inserted != 1 {
		t.Fatalf("expected 1 inserted, got %d", inserted)
	}

	store, err := reg.GetEntityStore("alpha", "product")
	if err != nil {
		t.Fatal(err)
	}
	rec, err := store.FindByField(context.Background(), "demo", "code", "P-1")
	if err != nil || rec == nil {
		t.Fatalf("product not seeded: %v", err)
	}
	key, _ := rec.Data["photo"].(string)
	if key == "" {
		t.Fatal("photo is empty — `$asset` did not attach an object key")
	}
	// {workspace}/{module}/{entity}/{id}/{field}/{uuid}-{name}
	wantPrefix := "demo/alpha/product/" + rec.ID + "/photo/"
	if !strings.HasPrefix(key, wantPrefix) {
		t.Errorf("object key %q does not start with %q — it must use the same canonical shape as the HTTP upload route", key, wantPrefix)
	}
	if !strings.HasSuffix(key, "-widget.jpg") {
		t.Errorf("object key %q must keep the source filename for a readable download", key)
	}

	// The bytes must actually be in the object store (this is what replaced the
	// `cp` step: the object is written through the storage service).
	got, err := os.ReadFile(filepath.Join(storageRoot, filepath.FromSlash(key)))
	if err != nil {
		t.Fatalf("object not found in storage: %v", err)
	}
	if string(got) != jpegBytes {
		t.Errorf("stored bytes differ from the source asset (%d vs %d bytes)", len(got), len(jpegBytes))
	}
}

// TestSeedReconcilesExistingRecord pins the second half of the change: a
// re-run does not duplicate, but it DOES bring a drifted record back in line.
//
// Before this, an existing record was skipped outright, so a seed whose value
// had been fixed (a photo that used to be empty, a corrected price) never
// reached an existing database — the file looked right while the app stayed
// wrong.
func TestSeedReconcilesExistingRecord(t *testing.T) {
	dir := t.TempDir()
	assetSeedSpec(t, dir)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	res, reg, up := seedHarness(t, dir, storageRoot)
	ctx := context.Background()

	if _, _, _, failed := seedAllWith(ctx, res, reg, "", "demo", up); failed != 0 {
		t.Fatalf("first run failed=%d", failed)
	}

	store, err := reg.GetEntityStore("alpha", "product")
	if err != nil {
		t.Fatal(err)
	}
	rec, _ := store.FindByField(ctx, "demo", "code", "P-1")
	if rec == nil {
		t.Fatal("seeded record missing")
	}

	// Simulate drift: an empty photo (the real kafe case — rows created by hand
	// through the UI) and a renamed product.
	if err := store.UpdateFields(ctx, "demo", rec.ID, map[string]any{"photo": nil, "name": "Nama Lama"}); err != nil {
		t.Fatal(err)
	}

	inserted, updated, skipped, failed := seedAllWith(ctx, res, reg, "", "demo", up)
	if failed != 0 {
		t.Fatalf("second run failed=%d", failed)
	}
	if inserted != 0 {
		t.Errorf("re-run inserted %d records — reconcile must not duplicate", inserted)
	}
	if updated != 1 {
		t.Errorf("re-run reported updated=%d, want 1 (photo was empty, name drifted)", updated)
	}
	if skipped != 0 {
		t.Errorf("re-run reported skipped=%d — a drifted record must not be reported as untouched", skipped)
	}

	rec, _ = store.FindByField(ctx, "demo", "code", "P-1")
	key, _ := rec.Data["photo"].(string)
	if key == "" {
		t.Error("photo is still empty after reconcile — an empty file field must be filled from the asset")
	}
	if rec.Data["name"] != "Widget" {
		t.Errorf("name = %v, want Widget (drift must be written back)", rec.Data["name"])
	}
}

// TestSeedReconcileIsIdempotent pins that "reconcile" does not mean "rewrite on
// every run": a second pass over an in-sync record reports 0 updated.
func TestSeedReconcileIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	assetSeedSpec(t, dir)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	res, reg, up := seedHarness(t, dir, storageRoot)
	ctx := context.Background()

	seedAllWith(ctx, res, reg, "", "demo", up)
	seedAllWith(ctx, res, reg, "", "demo", up)

	inserted, updated, skipped, failed := seedAllWith(ctx, res, reg, "", "demo", up)
	if inserted != 0 || updated != 0 || failed != 0 || skipped != 1 {
		t.Fatalf("third run = %d inserted / %d updated / %d skipped / %d failed, want 0/0/1/0",
			inserted, updated, skipped, failed)
	}
}

// TestSeedRestoresMissingObjects pins the recovery property that made
// `make seed-kafe-assets` unnecessary: if the object store is wiped, the next
// seed run re-uploads the assets instead of leaving a record pointing at a
// missing object.
func TestSeedRestoresMissingObjects(t *testing.T) {
	dir := t.TempDir()
	assetSeedSpec(t, dir)
	storageRoot := filepath.Join(t.TempDir(), "storage")
	res, reg, up := seedHarness(t, dir, storageRoot)
	ctx := context.Background()

	seedAllWith(ctx, res, reg, "", "demo", up)
	store, _ := reg.GetEntityStore("alpha", "product")
	rec, _ := store.FindByField(ctx, "demo", "code", "P-1")
	key, _ := rec.Data["photo"].(string)

	// Wipe the object (the `rm -rf .formspec/` case), keeping the DB row.
	if err := os.RemoveAll(storageRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(storageRoot, filepath.FromSlash(key))); !os.IsNotExist(err) {
		t.Fatalf("object should be gone, stat err = %v", err)
	}

	_, updated, _, failed := seedAllWith(ctx, res, reg, "", "demo", up)
	if failed != 0 {
		t.Fatalf("recovery run failed=%d", failed)
	}
	if updated != 1 {
		t.Fatalf("recovery run updated=%d, want 1 (the missing object must be re-uploaded)", updated)
	}
	// The re-upload mints a NEW key (the canonical key embeds a fresh UUID), so
	// the assertion is on the key the record now points at — not the old one.
	rec, _ = store.FindByField(ctx, "demo", "code", "P-1")
	newKey, _ := rec.Data["photo"].(string)
	if newKey == "" {
		t.Fatal("photo is empty after recovery")
	}
	if _, err := os.Stat(filepath.Join(storageRoot, filepath.FromSlash(newKey))); err != nil {
		t.Fatalf("object was not restored at the recorded key %s: %v", newKey, err)
	}
}

// TestSeedReconcileSkipsMaskedFields pins a credential-safety rule found by
// running the seed against the kafe dev database.
//
// The user entity declares `password` with `masked: true`: it is a WRITE-ONLY
// input that the entity's before create/update hook turns into `password_hash`.
// Reconcile writes through UpdateFields, which runs no hooks — so copying the
// seed's plaintext `password` into an existing record stored the credential in
// the clear right next to a valid hash (measured on the kafe DB:
// `password: 'kafe123'`). Write-only fields must never be reconciled.
func TestSeedReconcileSkipsMaskedFields(t *testing.T) {
	dir := t.TempDir()
	modDir := filepath.Join(dir, "modules", "alpha")
	if err := os.MkdirAll(filepath.Join(modDir, "master", "account"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(modDir, "module.yaml"), `apiVersion: formspec.dev/v1
kind: Module
metadata: { name: alpha }
spec: { version: 1.0.0 }
`)
	write(t, filepath.Join(modDir, "master", "account", "entity.yaml"), `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: account, module: alpha }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: username, type: string, unique: true }
    - { name: password, type: string, masked: true }
    - { name: password_hash, type: string }
`)
	write(t, filepath.Join(dir, "seed.yaml"), `apiVersion: formspec.dev/v1
kind: Seed
metadata: { name: demo, module: alpha }
spec:
  entities:
    - entity: account
      records:
        - { username: siti, password: "rahasia", password_hash: "$2a$10$hashasli" }
`)

	database, err := db.Open("sqlite::memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	reg := entity.NewRegistry(database, db.DriverSQLite, dir)
	for _, loadErr := range reg.LoadEntities() {
		t.Fatalf("load entity: %v", loadErr)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		t.Fatalf("sync schema: %v", err)
	}
	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	store, err := reg.GetEntityStore("alpha", "account")
	if err != nil {
		t.Fatal(err)
	}
	// The record is created OUTSIDE the seed, with no `password` key at all —
	// the shape a real account has (the plaintext never reaches storage; only
	// `password_hash` does). Then the seed runs against it.
	if _, err := store.Insert(ctx, db.InsertParams{
		WorkspaceID: "demo",
		CreatedBy:   "test",
		Data: map[string]any{
			"username":      "siti",
			"password_hash": "$2a$10$hashasli",
		},
	}); err != nil {
		t.Fatal(err)
	}

	if _, _, _, failed := seedAllWith(ctx, res, reg, "", "demo", nil); failed != 0 {
		t.Fatalf("seed run failed=%d", failed)
	}

	rec, _ := store.FindByField(ctx, "demo", "username", "siti")
	if rec == nil {
		t.Fatal("account missing")
	}
	if pw, present := rec.Data["password"]; present && pw != nil && pw != "" {
		t.Errorf("stored `password` = %q — a masked (write-only) field must never be reconciled: "+
			"UpdateFields runs no hooks, so the plaintext would be persisted next to the hash", pw)
	}
	if rec.Data["password_hash"] != "$2a$10$hashasli" {
		t.Errorf("password_hash = %v, want the stored value preserved", rec.Data["password_hash"])
	}
}

// TestSeedAssetRejectedOnNonFileField pins that `$asset` is validated, not
// guessed: using it on a string field is an error, because the alternative is
// silently storing a filesystem path where an object key belongs.
func TestSeedAssetRejectedOnNonFileField(t *testing.T) {
	dir := t.TempDir()
	assetSeedSpec(t, dir)
	// Point `$asset` at a non-file field.
	write(t, filepath.Join(dir, "seed.yaml"), `apiVersion: formspec.dev/v1
kind: Seed
metadata: { name: demo, module: alpha }
spec:
  entities:
    - entity: product
      records:
        - code: P-1
          name: { $asset: "products/widget.jpg" }
`)

	res, reg, up := seedHarness(t, dir, filepath.Join(t.TempDir(), "storage"))
	_, _, _, failed := seedAllWith(context.Background(), res, reg, "", "demo", up)
	if failed != 1 {
		t.Fatalf("failed = %d, want 1 (`$asset` on a non-file field must be rejected)", failed)
	}
}

// TestSeedAssetMissingFileFails pins the failure message quality: a missing
// asset must report the directory that was searched, not just "file not found".
func TestSeedAssetMissingFileFails(t *testing.T) {
	dir := t.TempDir()
	assetSeedSpec(t, dir)
	_, reg, up := seedHarness(t, dir, filepath.Join(t.TempDir(), "storage"))
	_, _, err := up.resolveAssets("alpha", entitySpecOf(t, reg, "alpha", "product"), map[string]any{
		"code": "P-1", "photo": map[string]any{assetMarker: "products/tidak-ada.jpg"},
	})
	if err == nil {
		t.Fatal("expected an error for a missing asset")
	}
	if !strings.Contains(err.Error(), "assets") {
		t.Errorf("error %q should name the directory that was searched", err)
	}
}

func entitySpecOf(t *testing.T, reg *entity.Registry, module, name string) *spec.EntitySpec {
	t.Helper()
	info, ok := reg.GetEntity(module, name)
	if !ok || info.EntitySpec == nil {
		t.Fatalf("entity %s/%s not registered", module, name)
	}
	return info.EntitySpec
}
