package formspec

import (
	"fmt"
	"path/filepath"

	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/manifest"
	specpkg "github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/memory"
)

// ─── Storage service resolution (satu tempat) ───
//
// Object store untuk field `file` diresolusi dari datastore registry: service
// yang menyajikan primitive `storage` dan memakai driver object-store
// (garage/minio/s3) menang; kalau tidak ada, fallback ke filesystem di bawah
// state dir.
//
// Kenapa diekspor: jalur boot (`App`) dan jalur CLI (`formspec seed`) harus
// memakai storage yang SAMA. Sebelumnya `formspec seed` tidak memakai storage
// sama sekali — foto seed ditanam dengan `cp` oleh target Makefile — sehingga
// "seed bergambar" hanya bisa jalan di dev (filesystem), bukan di prod
// (garage/minio/s3). Lihat docs_internal/plan/seed-assets-and-reconcile.md §1.1.

// ObjectStoreStorage is the storage contract as this package uses it.
//
// Alias (bukan interface baru) supaya `resource` dan `cmd/formspec` menyebut
// tipe yang sama tanpa salah satunya bergantung pada nama internal api.Storage.
type ObjectStoreStorage = api.Storage

// NewDatastoreRegistryFromManifests builds a datastore registry from loaded
// manifests, using the built-in in-memory backends for the non-persistent
// primitives. It is the CLI counterpart of the registry the server builds at
// boot: value-level storage lives in manifests, so a CLI that ignored them
// would write objects somewhere the server never reads.
func NewDatastoreRegistryFromManifests(manifests []manifest.RawManifest, database db.DB, stateDir string) (*DatastoreRegistry, error) {
	return buildDatastoreRegistry(manifests, database, stateDir, memory.NewPubSub())
}

// ResolveStorage returns the object store for a datastore registry, preferring
// a named object-store service and falling back to the filesystem under
// stateDir. Deterministic: service names are probed in sorted order.
func ResolveStorage(dsReg *DatastoreRegistry, stateDir string) (ObjectStoreStorage, error) {
	for _, name := range sortedServiceNames(dsReg) {
		e := dsReg.services[name]
		if e == nil || e.spec == nil {
			continue
		}
		drv := e.spec.Driver
		if drv != specpkg.DatastoreDriverGarage && drv != specpkg.DatastoreDriverMinio && drv != specpkg.DatastoreDriverS3 {
			continue
		}
		servesStorage := false
		for _, p := range e.spec.Serves {
			if p == specpkg.PrimitiveStorage {
				servesStorage = true
				break
			}
		}
		if !servesStorage {
			continue
		}
		conn, err := dsReg.Resolve("storage", name, "")
		if err != nil {
			return nil, fmt.Errorf("init storage service %q: %w", name, err)
		}
		store, ok := conn.(ObjectStoreStorage)
		if !ok {
			return nil, fmt.Errorf("storage service %q does not implement the object-store contract", name)
		}
		return store, nil
	}
	fsStore, err := memory.NewStorage(filepath.Join(stateDir, "storage"))
	if err != nil {
		return nil, fmt.Errorf("init storage: %w", err)
	}
	return fsStore, nil
}
