// Command `formspec seed` — run YAML seeders for dev/testing data
// (docs/cli-tools/02-formspec-cli.md §6). The `formspec/seed` official module
// does not exist yet, so this verb defines a minimal declarative seed format
// (kind: Seed) and inserts records through the same EntityStore the engine
// uses, so natural-key generation, field defaults, and validation all apply.
//
//	formspec seed [--spec <path>] [--dsn <dsn>] [--module <module>]
//
// Seed manifest format:
//
//	apiVersion: formspec.dev/v1
//	kind: Seed
//	metadata:
//	  name: demo-data
//	  module: billing
//	spec:
//	  entities:
//	    - entity: customer
//	      records:
//	        - { code: C-001, name: "PT Maju" }
//
// Idempotent: a record whose natural key already exists is skipped with a
// warning, not an error.
//
// `kind: Seed` is a registered kind (pkg/spec/seed.go), so `formspec validate`
// accepts and shapes-checks these files like any other manifest.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	formspec "github.com/primadi/formspec/resource"
)

// userEntity is the one entity whose records cannot be inserted raw: the engine
// hashes `password` into `password_hash` in a `before create` hook that runs on
// the action/API path, NOT in EntityStore.Insert — which is the path seeding
// uses. Writing the raw record would store a plaintext `password` (harmless but
// useless: `password_hash` is required and empty) and login would always fail
// with no error at seed time. So users are routed through CreateUser, the same
// canonical path the register/login code uses.
const userEntity = "user"

func runSeed(args []string) {
	specPath := "spec"
	dsn := "sqlite:.formspec/data.db"
	moduleFilter := ""
	// Seeded rows land in a workspace (tenant scope). Default stays "demo" for
	// backward compatibility, but a named deployment (kafe) must pass its own
	// slug or the rows exist in a tenant the App never reads.
	workspaceID := "demo"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--spec", "-spec":
			if i+1 < len(args) {
				specPath = args[i+1]
				i++
			}
		case "--dsn", "-dsn":
			if i+1 < len(args) {
				dsn = args[i+1]
				i++
			}
		case "--module", "-module":
			if i+1 < len(args) {
				moduleFilter = args[i+1]
				i++
			}
		case "--workspace", "-workspace":
			if i+1 < len(args) {
				workspaceID = args[i+1]
				i++
			}
		case "--help", "-h":
			_, _ = fmt.Fprintf(os.Stderr, "Usage: formspec seed [--spec <path>] [--dsn <dsn>] [--module <module>] [--workspace <slug>]\n")
			os.Exit(0)
		default:
			_, _ = fmt.Fprintf(os.Stderr, "formspec seed: unknown flag %q\n", args[i])
			os.Exit(2)
		}
	}

	loader := manifest.NewLoader(specPath)
	res, err := loader.LoadAll()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: load manifests: %v\n", err)
		os.Exit(1)
	}

	// Anchor a relative SQLite path to the project root derived from the spec
	// path, like every other lifecycle verb (dev/backup/repl/archive). Without
	// this, `formspec seed` run from a different CWD writes a DIFFERENT database
	// than the one the server reads — a seed that reports success while the app
	// stays empty (plan dsn-spec-anchored.md).
	dsn = resolveDSN(dsn, specPath)

	database, err := db.Open(dsn)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: open database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}

	reg := entity.NewRegistry(database, driver, specPath)
	for _, loadErr := range reg.LoadEntities() {
		_, _ = fmt.Fprintf(os.Stderr, "formspec seed: load warning: %v\n", loadErr)
	}
	// The framework-owned `formspec.core` entities (role, user, workspace, ...)
	// live in an embedded module, not in the user's spec tree. Without this,
	// seeding a role or a user fails with "entity formspec.core.role not found"
	// — and those are the records an app most needs seeded, because a role with
	// no grants makes every permission test meaningless. The runtime registers
	// this module on boot (resource.App.New); the CLI must do the same or the
	// seed path and the server disagree about which entities exist.
	if err := auth.RegisterCoreEntities(reg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "formspec seed: core module: %v\n", err)
	}
	if _, err := reg.SyncSchema(context.Background()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: sync schema: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	uploader := newSeedAssetUploader(res, database, driver, workspaceID, specPath, dsn)
	inserted, updated, skipped, failed := seedAllWith(ctx, res, reg, moduleFilter, workspaceID, uploader)

	fmt.Printf("Seed complete: %d inserted, %d updated, %d skipped, %d failed.\n",
		inserted, updated, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// newSeedAssetUploader wires the `$asset` uploader for a seed run.
//
// Storage is resolved from the SAME datastore registry the server builds, so
// uploaded objects land wherever the app reads them from — filesystem in dev,
// garage/minio/s3 when a `kind: Datastore` declares one. That is the whole
// difference from the `cp` step this replaces
// (docs_internal/plan/seed-assets-and-reconcile.md §2, keputusan D3).
//
// Resolution stays LAZY (inside the returned closure): a seed with no `$asset`
// must not require a usable object store.
func newSeedAssetUploader(res *manifest.LoadResult, database db.DB, driver db.DriverType, workspaceID, specPath, dsn string) *assetUploader {
	// StateDirFor anchors to the project root for BOTH DSN kinds: a SQLite DSN
	// already arrives absolute (resolveDSN), while a non-SQLite one would
	// otherwise yield a CWD-relative ".formspec" (gap 10.17).
	stateDir := formspec.StateDirFor(dsn, specPath)
	dsReg, err := formspec.NewDatastoreRegistryFromManifests(res.Manifests, database, stateDir)
	if err != nil {
		// A malformed Datastore manifest must be reported, not swallowed. The
		// seed continues with the registry it could build, and the message names
		// the manifest — the same posture the boot path takes for load warnings.
		_, _ = fmt.Fprintf(os.Stderr, "formspec seed: datastore registry: %v\n", err)
	}
	return newAssetUploader(res, workspaceID, func() (api.Storage, error) {
		return formspec.ResolveStorage(dsReg, stateDir)
	}, api.DefaultUploadLimitMB)
}

// seedAll runs every kind: Seed manifest against the registry, returning
// insert/skip/fail counts. Extracted for testability.
func seedAll(ctx context.Context, res *manifest.LoadResult, reg *entity.Registry, moduleFilter, workspaceID string) (inserted, skipped, failed int) {
	inserted, _, skipped, failed = seedAllWith(ctx, res, reg, moduleFilter, workspaceID, nil)
	return inserted, skipped, failed
}

// seedAllWith is the full form: it additionally reports how many existing
// records were reconciled (field values brought back in line with the seed) and
// uploads `$asset` values through the storage service.
//
// `uploader` may be nil — then `$asset` markers are an error rather than a
// silent no-op, because a manifest that asks for an asset and gets nothing is
// exactly the "looks seeded and is quietly broken" failure this codebase
// rejects everywhere else.
func seedAllWith(ctx context.Context, res *manifest.LoadResult, reg *entity.Registry, moduleFilter, workspaceID string, uploader *assetUploader) (inserted, updated, skipped, failed int) {
	// Seed manifests are collected first, then run in YAML order.
	//
	// Block ORDER in the file does not need to match dependency order: `$ref`
	// values are resolved by lookup against the database plus everything this
	// run has already inserted, so a block may point at a row that a later
	// block creates *on a previous run* — but a reference to a row that does
	// not exist anywhere yet DOES fail, with a message naming the missing
	// field=value and telling the author to seed that entity first. Silently
	// inserting a null foreign key instead would produce an order that looks
	// seeded and is quietly broken.
	var deferredBlocks []seedBlock
	for _, m := range res.Manifests {
		if m.Kind != "Seed" {
			continue
		}
		if moduleFilter != "" && m.Metadata.Module != moduleFilter {
			continue
		}
		var seed spec.SeedSpec
		if err := reparseSpec(m.Spec, &seed); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s: invalid seed spec: %v\n", m.Source, err)
			failed++
			continue
		}
		deferredBlocks = append(deferredBlocks, seedBlock{
			source: m.Source,
			module: m.Metadata.Module,
			spec:   seed,
		})
	}

	// ONE resolver for the whole run: a `$ref` resolved while seeding one block
	// must be reusable by later blocks, and a record inserted by this run must
	// be findable by the next lookup.
	refs := newRefResolver(reg, workspaceID)

	// Iterate to a fixed point instead of trusting YAML order. A record whose
	// `$ref` cannot be resolved yet is deferred, not failed: the block that
	// creates its target may come later in the file, or in another manifest.
	// Only when a full sweep inserts nothing AND unresolved work remains is the
	// remaining set reported as an error.
	//
	// This is why resolution is deferred rather than immediate: an immediate
	// failure would make the author responsible for ordering blocks by their
	// dependencies, and the error would point at the second block while the fix
	// belongs to the first line of the file.
	type pendingRecord struct {
		entity string
		rec    map[string]any
	}
	pendingRecords := make([]pendingRecord, 0, 16)
	for _, blk := range deferredBlocks {
		for _, se := range blk.spec.Entities {
			for _, rec := range se.Records {
				pendingRecords = append(pendingRecords, pendingRecord{entity: blk.module + "." + se.Entity, rec: rec})
			}
		}
	}

	for {
		var next []pendingRecord
		progress := false
		for _, p := range pendingRecords {
			// Split at the LAST dot: a module name may itself contain dots
			// (`formspec.core`), so cutting at the first one yields
			// module="formspec", entity="core.role" — which fails with a
			// confusing "entity formspec/core.role not found".
			module, entityName, _ := splitRefLastDot(p.entity)
			store, err := reg.GetEntityStore(module, entityName)
			if err != nil {
				// Not a retry-able condition: no amount of extra passes will
				// register the entity. Report it with the reason instead of
				// deferring, or the author sees "could not be inserted".
				_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s: %v\n", p.entity, err)
				failed++
				continue
			}
			info, ok := reg.GetEntity(module, entityName)
			if !ok || info.EntitySpec == nil {
				_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s: entity not registered\n", p.entity)
				failed++
				continue
			}
			rec, refErr := refs.resolve(p.rec)
			if refErr != nil {
				// Unresolved this sweep — retry after other records land.
				next = append(next, p)
				continue
			}
			// `$asset` values become concrete object keys only after the record
			// exists (the canonical key embeds its ID). Split the record here:
			// what is inserted now, and what is uploaded next.
			var assetPending []pendingAsset
			if uploader != nil {
				var assetErr error
				rec, assetPending, assetErr = uploader.resolveAssets(module, info.EntitySpec, rec)
				if assetErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s: %v\n", p.entity, assetErr)
					failed++
					progress = true
					continue
				}
			} else if hasAssetMarker(rec) {
				_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s: record uses `%s` but asset upload is not configured\n", p.entity, assetMarker)
				failed++
				progress = true
				continue
			}
			// Idempotency key: the natural key when the entity declares one,
			// otherwise its single `unique` field, otherwise a composite
			// `invariants: [{unique: [...]}]` tuple. Roles and users rely on the
			// middle case; per-branch prices (`menu-item-price`, unique per
			// branch+menu) rely on the last. Without this a second run tries to
			// insert duplicates, hits the unique index, and reports failure for a
			// seed that is already in the state the author asked for.
			if idField, idValue, ok := seedIdentityKey(info.EntitySpec, rec); ok {
				existing, findErr := findByIdentity(ctx, store, workspaceID, idField, idValue)
				if findErr == nil && existing != nil {
					// Users are skipped, never reconciled. The user entity's
					// `password` is a WRITE-ONLY input: the entity's
					// before create/update hook turns it into `password_hash`.
					// Reconcile writes through UpdateFields, which runs no hooks
					// — so "fixing" a user would store the plaintext password in
					// the record (measured: `password: 'kafe123'` written next to
					// a valid hash, i.e. a credential leak). The canonical write
					// path for users is CreateUser, and an existing account's
					// credentials are not seed data to be brought back in line.
					if entityName == userEntity {
						_, _ = fmt.Fprintf(os.Stderr, "formspec seed: skip %s %s=%v (already exists)\n", p.entity, idField, idValue)
						skipped++
						progress = true
						continue
					}
					// Idempotence is not the same as "leave it alone". A seed
					// that was fixed (a photo that used to be empty, a price that
					// changed) must actually reach an existing database, or the
					// file looks right while the app stays wrong. Reconcile the
					// differing fields and say so.
					changed, updCnt, recErr := reconcileRecord(ctx, store, uploader, info.EntitySpec, existing, rec, module, entityName, workspaceID, assetPending)
					if recErr != nil {
						_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s %s=%v: %v\n", p.entity, idField, idValue, recErr)
						failed++
						progress = true
						continue
					}
					if updCnt > 0 {
						_, _ = fmt.Fprintf(os.Stderr, "formspec seed: update %s %s=%v (%s)\n", p.entity, idField, idValue, changed)
						updated++
					} else {
						_, _ = fmt.Fprintf(os.Stderr, "formspec seed: skip %s %s=%v (already exists)\n", p.entity, idField, idValue)
						skipped++
					}
					progress = true
					continue
				}
			}
			// Users go through CreateUser: it hashes `password` into
			// `password_hash` (entity hooks do not run on the seed path), so
			// seeded accounts can actually log in.
			if entityName == userEntity {
				if err := insertUser(ctx, store, workspaceID, rec); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s insert %v: %v\n", p.entity, rec, err)
					failed++
					progress = true
					continue
				}
				inserted++
				progress = true
				continue
			}
			id, err := store.Insert(ctx, db.InsertParams{
				WorkspaceID: workspaceID,
				CreatedBy:   "seed",
				Data:        rec,
			})
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s insert %v: %v\n", p.entity, rec, err)
				failed++
				progress = true
				continue
			}
			// Upload the assets and point the row at them. A failure here leaves
			// a row with an empty file field — benign, and healed by re-running
			// the seed — so it is reported rather than rolled back.
			if len(assetPending) > 0 {
				keyUpdates, upErr := uploader.upload(ctx, module, entityName, id, assetPending)
				if upErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s (%s): %v\n", p.entity, id, upErr)
					failed++
					progress = true
					continue
				}
				if err := store.UpdateFields(ctx, workspaceID, id, keyUpdates); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s (%s): attach assets: %v\n", p.entity, id, err)
					failed++
					progress = true
					continue
				}
			}
			inserted++
			progress = true
		}
		if len(next) == 0 {
			break
		}
		if !progress {
			// Nothing moved: the remainder cannot be resolved by more passes.
			// Report the real cause rather than looping.
			for _, p := range next {
				if _, refErr := refs.resolve(p.rec); refErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s: %v\n", p.entity, refErr)
				} else {
					_, _ = fmt.Fprintf(os.Stderr, "formspec seed: %s: could not be inserted\n", p.entity)
				}
				failed++
			}
			break
		}
		pendingRecords = next
	}
	return inserted, updated, skipped, failed
}

// reconcilableStatus reports whether a record's lifecycle state allows a seed to
// update it in place. A document that has been submitted (or cancelled) is no
// longer "seed data": its content is a business fact, and UpdateFields bypasses
// the lifecycle guard that exists precisely to prevent edits after submit.
func reconcilableStatus(rec *db.EntityRecord) bool {
	status := spec.DocStatus(rec.DocStatus)
	return status == "" || status == spec.DocStatusDraft
}

// reconcileRecord brings an existing record in line with the seed, writes the
// difference, and returns the changed field names plus how many were written.
//
// Only declared seed fields are considered: a seed declares the values it names,
// not a full replace of the row. Fields the record carries from elsewhere
// (computed, hook-written, or entered in the UI) stay untouched.
//
// File fields get the one treatment a value comparison cannot give them: every
// upload writes a NEW unique key, so "the same asset" can never be recognised
// from the stored value. What can be checked is whether the recorded object is
// still there — so a wiped state dir (or a recreated bucket) is healed by the
// next seed run, which is what replaced the old `seed-kafe-assets` copy step.
func reconcileRecord(ctx context.Context, store *db.EntityStore, uploader *assetUploader, es *spec.EntitySpec, existing *db.EntityRecord, rec map[string]any, module, entity, workspaceID string, pending []pendingAsset) (changed []string, count int, err error) {
	if !reconcilableStatus(existing) {
		return nil, 0, nil
	}
	updates := map[string]any{}

	// File fields: re-upload when the stored value is empty, when the object it
	// names is gone from storage, or when the object's BYTES differ from the
	// asset the seed now names.
	//
	// The byte comparison is what makes a corrected asset actually land. A
	// value comparison cannot work (every upload mints a new key, so the stored
	// value never equals an asset path) and a mere existence check is not enough
	// either — replacing `es-jeruk.jpg` with the right photo left the old object
	// untouched and the app kept showing the wrong picture. Seeds are small
	// files; reading them back to compare is cheap and is the only way to tell
	// "already uploaded" from "uploaded, then fixed".
	if uploader != nil && len(pending) > 0 {
		storage, storeErr := uploader.storage()
		if storeErr != nil {
			return nil, 0, fmt.Errorf("storage service: %w", storeErr)
		}
		var toUpload []pendingAsset
		for _, p := range pending {
			key, _ := existing.Data[p.field].(string)
			if key != "" {
				stored, downErr := storage.Download(ctx, key)
				if downErr == nil && bytes.Equal(stored, p.data) {
					continue // object present and identical — leave the row alone
				}
			}
			toUpload = append(toUpload, p)
		}
		if len(toUpload) > 0 {
			keyUpdates, upErr := uploader.upload(ctx, module, entity, existing.ID, toUpload)
			if upErr != nil {
				return nil, 0, upErr
			}
			for k, v := range keyUpdates {
				updates[k] = v
				changed = append(changed, k)
			}
		}
	}

	// Scalar drift: a seed field whose value differs from what is stored. File
	// fields are handled above, because their stored key is never "equal" to an
	// asset path.
	for k, want := range rec {
		if _, isFile := pendingAssetField(pending, k); isFile {
			continue
		}
		if want == nil {
			continue // a cleared `$asset` placeholder carries no value of its own
		}
		// Write-only fields are never reconciled. `masked: true` marks a field
		// whose stored value is not its written value — the user entity's
		// `password` is the canonical case: the entity's before create/update
		// hook turns it into `password_hash`, and UpdateFields runs no hooks.
		// Copying the seed's plaintext `password` into the record therefore
		// stores the credential in the clear (measured: `password: 'kafe123'`
		// written alongside a valid hash). Skipping is the only safe default for
		// a field the framework declares cannot be read back.
		if f := findField(es, k); f != nil && f.Masked {
			continue
		}
		got, present := existing.Data[k]
		if present && valueEqual(got, want) {
			continue
		}
		updates[k] = want
		changed = append(changed, k)
	}
	if len(updates) == 0 {
		return nil, 0, nil
	}
	if err := store.UpdateFields(ctx, workspaceID, existing.ID, updates); err != nil {
		return nil, 0, fmt.Errorf("update fields: %w", err)
	}
	sort.Strings(changed)
	return changed, len(updates), nil
}

// pendingAssetField reports whether a pending asset targets the named field.
func pendingAssetField(pending []pendingAsset, field string) (pendingAsset, bool) {
	for _, p := range pending {
		if p.field == field {
			return p, true
		}
	}
	return pendingAsset{}, false
}

// valueEqual compares a stored JSON value with a seed value. Numbers arrive as
// float64 from JSON and as int/float from YAML, so numerics are compared
// numerically; everything else uses a normalised deep comparison.
func valueEqual(a, b any) bool {
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		return af == bf
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	default:
		return 0, false
	}
}

// hasAssetMarker reports whether any value in a record is a `$asset` marker.
func hasAssetMarker(rec map[string]any) bool {
	for _, v := range rec {
		if m, ok := v.(map[string]any); ok {
			if _, found := m[assetMarker]; found {
				return true
			}
		}
	}
	return false
}

// findByIdentity returns the record matching a seed identity, or nil when none
// exists. It mirrors naturalKeyExists (same single-field / composite-key rules)
// but returns the record, which reconcile needs.
func findByIdentity(ctx context.Context, store *db.EntityStore, workspaceID, field string, value any) (*db.EntityRecord, error) {
	if value == nil {
		return nil, nil
	}
	if !strings.Contains(field, "+") {
		rec, err := store.FindByField(ctx, workspaceID, field, value)
		if err != nil {
			return nil, nil // not found is not an error for a find
		}
		return rec, nil
	}
	parts := strings.Split(field, "+")
	vals := strings.Split(fmt.Sprintf("%v", value), "\x1f")
	if len(parts) != len(vals) {
		return nil, nil
	}
	match := map[string]any{}
	for i, name := range parts {
		match[name] = vals[i]
	}
	rec, err := store.FindByFields(ctx, workspaceID, match)
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// seedBlock is one loaded kind: Seed manifest held for the two-pass run.
type seedBlock struct {
	source string
	module string
	spec   spec.SeedSpec
}

// refResolver resolves `$ref` values inside seed records.
//
// A relationship in this framework is stored as the target record's ID (a
// generated UUID), which a YAML author cannot know. Without this, seeding a
// purchase order — or a menu price, or a recipe line — is impossible to
// express, and those are exactly the records that make an app's seed useful.
//
// Syntax: `$ref: <module>.<entity>:<field>=<value>`. The target is looked up by
// the named field (a natural key, a code, a name — whatever the author trusts
// to be unique), and the value is replaced by the resolved record ID.
//
//	menu_category_id: {$ref: "cafe-master.menu-category:name=Makanan"}
//
// Resolution order: records already inserted in this run (via the natural-key
// index), then a database lookup. The DB lookup is what makes a partial seed
// work — re-running after adding one block finds the rows inserted by the
// earlier run instead of failing.
type refResolver struct {
	reg         *entity.Registry
	workspaceID string
	byRef       map[string]string // "module.entity:field=value" → record ID
}

func newRefResolver(reg *entity.Registry, workspaceID string) *refResolver {
	return &refResolver{reg: reg, workspaceID: workspaceID, byRef: map[string]string{}}
}

// resolve walks a record and replaces every `$ref` with the target record ID.
func (r *refResolver) resolve(rec map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(rec))
	for k, v := range rec {
		rv, err := r.resolveValue(v)
		if err != nil {
			return rec, fmt.Errorf("field %q: %w", k, err)
		}
		out[k] = rv
	}
	return out, nil
}

func (r *refResolver) resolveValue(v any) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		// A single-key map with `$ref` is a reference; anything else is a
		// nested payload (money, child rows) that may itself contain refs.
		if raw, ok := t["$ref"]; ok && len(t) == 1 {
			expr, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("$ref must be a string, got %T", raw)
			}
			return r.lookup(expr)
		}
		out := make(map[string]any, len(t))
		for k, inner := range t {
			rv, err := r.resolveValue(inner)
			if err != nil {
				return nil, err
			}
			out[k] = rv
		}
		return out, nil
	case []any:
		out := make([]any, 0, len(t))
		for _, inner := range t {
			rv, err := r.resolveValue(inner)
			if err != nil {
				return nil, err
			}
			out = append(out, rv)
		}
		return out, nil
	default:
		return v, nil
	}
}

// lookup resolves "module.entity:field=value" to a record ID.
func (r *refResolver) lookup(expr string) (string, error) {
	if id, ok := r.byRef[expr]; ok {
		return id, nil
	}
	entityRef, cond, ok := strings.Cut(expr, ":")
	if !ok {
		return "", fmt.Errorf("$ref %q must be `<module>.<entity>:<field>=<value>`", expr)
	}
	module, entityName, ok := strings.Cut(entityRef, ".")
	if !ok {
		return "", fmt.Errorf("$ref %q must name an entity as <module>.<entity>", expr)
	}
	field, value, ok := strings.Cut(cond, "=")
	if !ok {
		return "", fmt.Errorf("$ref %q must name a field and value as <field>=<value>", expr)
	}
	store, err := r.reg.GetEntityStore(module, entityName)
	if err != nil {
		return "", fmt.Errorf("$ref %q: entity %s.%s: %w", expr, module, entityName, err)
	}
	rec, err := store.FindByField(context.Background(), r.workspaceID, field, value)
	if err != nil || rec == nil {
		return "", fmt.Errorf("$ref %q: no %s.%s row with %s=%q — seed that entity first (order within one run does not matter, but the target must exist)", expr, module, entityName, field, value)
	}
	r.byRef[expr] = rec.ID
	return rec.ID, nil
}

// insertUser seeds a formspec.core user record through the canonical creation
// path so the plaintext `password` in the YAML becomes a bcrypt hash.
//
// The `roles`, `permissions`, and `assignments` fields are passed through
// as-is: a seeded role is only useful if the account arrives already carrying
// it, and the assignment list is what lets login answer 409 CONTEXT_REQUIRED
// with real choices (todo 3.8).
func insertUser(ctx context.Context, store *db.EntityStore, workspaceID string, rec map[string]any) error {
	u := &auth.User{
		Username:    stringFrom(rec, "username"),
		Email:       stringFrom(rec, "email"),
		DisplayName: stringFrom(rec, "display_name"),
	}
	// PasswordHash carries PLAINTEXT here — CreateUser hashes it (same
	// convention as the auth tests and the register handler).
	u.PasswordHash = stringFrom(rec, "password")
	if u.PasswordHash == "" {
		return fmt.Errorf("user %q: `password` is required (it is hashed into password_hash)", u.Username)
	}
	if b, ok := rec["email_verified"].(bool); ok {
		u.EmailVerified = b
	}
	if status := stringFrom(rec, "status"); status != "" {
		u.Status = status
	}
	if v, ok := rec["active"].(bool); ok && !v {
		u.Status = auth.UserStatusDisabled
	}
	u.Roles = stringSliceFrom(rec, "roles")
	u.Permissions = stringSliceFrom(rec, "permissions")
	for _, raw := range anySliceFrom(rec, "assignments") {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		u.Assignments = append(u.Assignments, auth.Assignment{
			Role:      stringFrom(m, "role"),
			Dimension: stringFrom(m, "dimension"),
			Value:     stringFrom(m, "value"),
		})
	}
	users := auth.NewEntityUserStore(store)
	return users.CreateUser(ctx, workspaceID, u)
}

// stringFrom reads a string field, tolerating YAML's non-string scalars (a
// natural key written as `123` still needs to become "123", not "").
func stringFrom(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// stringSliceFrom reads a string list from a record field.
func stringSliceFrom(m map[string]any, key string) []string {
	raw := anySliceFrom(m, key)
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		out = append(out, fmt.Sprintf("%v", v))
	}
	return out
}

// anySliceFrom reads a list field, tolerating a single scalar.
func anySliceFrom(m map[string]any, key string) []any {
	switch v := m[key].(type) {
	case []any:
		return v
	case nil:
		return nil
	default:
		return []any{v}
	}
}

// splitRefLastDot splits a "module.entity" reference at the LAST dot, so a
// module whose name contains dots (`formspec.core`) is not truncated into a
// bogus module plus a dotted entity name.
func splitRefLastDot(ref string) (module, entity string, ok bool) {
	i := strings.LastIndex(ref, ".")
	if i <= 0 || i == len(ref)-1 {
		return "", ref, false
	}
	return ref[:i], ref[i+1:], true
}

// seedIdentityKey returns the field and value a seed record is identified by,
// so re-running a seed skips instead of failing on a unique constraint.
//
// Preference order:
//  1. the declared natural key — that is what it is for;
//  2. the entity's single `unique` field — roles and users live here, and they
//     are the records an app re-seeds most often;
//  3. a composite `invariants: [{unique: [a, b]}]` tuple — per-branch prices
//     are unique per (branch, menu) and have no single unique field.
//
// Returns ok=false when none applies, in which case the insert is left to the
// database's own constraints and a genuine duplicate is reported.
func seedIdentityKey(es *spec.EntitySpec, rec map[string]any) (string, any, bool) {
	if es == nil {
		return "", nil, false
	}
	if nk := es.NaturalKeyField; nk != "" {
		if v, ok := rec[nk]; ok {
			return nk, v, true
		}
	}
	uniqueFields := make([]string, 0, 2)
	for i := range es.Fields {
		if es.Fields[i].Unique {
			uniqueFields = append(uniqueFields, es.Fields[i].Name)
		}
	}
	// Only when unambiguous: with two unique fields there is no way to tell
	// which one the author means, and guessing would skip the wrong row.
	if len(uniqueFields) == 1 {
		if v, ok := rec[uniqueFields[0]]; ok {
			return uniqueFields[0], v, true
		}
	}
	for _, inv := range es.Invariants {
		if len(inv.Unique) == 0 {
			continue
		}
		if field, value, ok := compositeIdentity(inv.Unique, rec); ok {
			return field, value, true
		}
	}
	// Composite unique INDEX — the other way the same rule gets written.
	// `menu-item-price` declares `indexes: [{fields: [branch_id, menu_item_id],
	// unique: true}]`, and an entity may express identity that way without ever
	// using `invariants`.
	for _, idx := range es.Indexes {
		if !idx.Unique || idx.Where != "" || len(idx.Fields) < 2 {
			continue
		}
		if field, value, ok := compositeIdentity(idx.Fields, rec); ok {
			return field, value, true
		}
	}
	return "", nil, false
}

// compositeIdentity builds the synthetic identity key for a multi-field unique
// rule, or reports false when the record does not carry the whole tuple.
func compositeIdentity(fields []string, rec map[string]any) (string, any, bool) {
	vals := make([]string, 0, len(fields))
	for _, f := range fields {
		v, ok := rec[f]
		if !ok {
			// Partial tuple — cannot use this rule as an identity.
			return "", nil, false
		}
		vals = append(vals, fmt.Sprintf("%v", v))
	}
	// Joined with a unit separator: it cannot appear in a field value, so the
	// tuple survives the round trip through naturalKeyExists.
	return strings.Join(fields, "+"), strings.Join(vals, "\x1f"), true
}

// reparseSpec re-marshals the raw spec (map[string]any) and unmarshals it into
// the typed seed spec.
func reparseSpec(raw any, out *spec.SeedSpec) error {
	specMap, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("seed spec must be a mapping")
	}
	typed, err := manifest.RawSpecTo[spec.SeedSpec](specMap)
	if err != nil {
		return err
	}
	*out = *typed
	return nil
}

// naturalKeyExists reports whether a record with the given identity already
// exists for the entity.
//
// `field` is normally a single field name. When it contains "+", it names a
// composite identity ("branch_id+menu_item_id") and `value` is the
// unit-separator-joined tuple — that is how a seed becomes idempotent for
// entities whose uniqueness is expressed as an invariant rather than a
// natural key.
func naturalKeyExists(ctx context.Context, store *db.EntityStore, workspaceID, field string, value any) (bool, error) {
	if value == nil {
		return false, nil
	}
	filters := map[string]db.FilterOp{}
	if strings.Contains(field, "+") {
		parts := strings.Split(field, "+")
		vals := strings.Split(fmt.Sprintf("%v", value), "\x1f")
		if len(parts) != len(vals) {
			return false, nil
		}
		for i, name := range parts {
			filters[name] = db.FilterOp{Op: "eq", Value: vals[i]}
		}
	} else {
		filters[field] = db.FilterOp{Op: "eq", Value: value}
	}
	res, err := store.List(ctx, db.ListParams{
		WorkspaceID: workspaceID,
		Page:        1,
		PerPage:     1,
		Filters:     filters,
	})
	if err != nil {
		return false, err
	}
	return res.Total > 0, nil
}
