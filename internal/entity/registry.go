// Package entity provides the entity registry — the bridge between
// YAML manifest loading and the database storage layer.
//
// The registry is the runtime "single source of truth" for all entities:
//  1. Load manifests from disk (via manifest.Loader)
//  2. Register entities (filter kind: Entity, parse specs, validate)
//  3. Sync schema to database (via db.MigrationRunner)
//  4. Provide EntityStore instances (via db.EntityStore factory)
package entity

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/primadi/forma/internal/manifest"
	"github.com/primadi/forma/internal/permission"
	"github.com/primadi/forma/pkg/spec"
	db "github.com/primadi/forma/renderers/jsonbpersist"
)

// Registry is the central entity registry for the Forma runtime.
// It manages the lifecycle of entity definitions from YAML to database.
type Registry struct {
	mu           sync.RWMutex
	db           db.DB
	driver       db.DriverType
	loader       *manifest.Loader
	specs        map[string]*SpecInfo       // key = "module/name"
	stores       map[string]*db.EntityStore // key = "module/name"
	synced       bool
	permRegistry *permission.Registry
}

// SpecInfo holds a parsed and validated entity spec along with its source.
type SpecInfo struct {
	Metadata   spec.Metadata
	EntitySpec *spec.EntitySpec
	Source     string        // file path + document index
	TableInfo  *db.TableInfo // cached DDL result (populated after SyncSchema)
}

// EntityInfo is a lightweight summary of a registered entity.
type EntityInfo struct {
	Name           string `json:"name"`
	Module         string `json:"module"`
	Kind           string `json:"kind"` // "Entity"
	Characteristic string `json:"characteristic,omitempty"`
	TableName      string `json:"table_name,omitempty"`
	FieldCount     int    `json:"field_count"`
	Source         string `json:"source"`
	Description    string `json:"description,omitempty"`
}

// NewRegistry creates a new EntityRegistry and initialises the manifest loader.
func NewRegistry(database db.DB, driver db.DriverType, specBasePath string) *Registry {
	return &Registry{
		db:           database,
		driver:       driver,
		loader:       manifest.NewLoader(specBasePath),
		specs:        make(map[string]*SpecInfo),
		stores:       make(map[string]*db.EntityStore),
		permRegistry: permission.NewRegistry(),
	}
}

// entityKey returns the registry key for a module/entity pair.
func entityKey(module, name string) string { return module + "/" + name }

// LoadEntities discovers, parses, and registers all Entity manifests
// found under the spec base path. It collects all parse/validation errors
// but does NOT stop after the first error (best-effort).
func (r *Registry) LoadEntities() []error {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.loader.LoadAll()
	if err != nil {
		return []error{fmt.Errorf("manifest load: %w", err)}
	}

	var allErrors []error

	// Collect loader-level errors
	for _, pe := range result.Errors {
		allErrors = append(allErrors, &pe)
	}

	// Filter and register only Entity (or deprecated Document) manifests
	for _, raw := range result.Manifests {
		if !spec.IsEntityKind(spec.Kind(raw.Kind)) {
			continue
		}

		key := entityKey(raw.Metadata.Module, raw.Metadata.Name)

		// Parse spec
		entitySpec, err := manifest.RawSpecToEntitySpec(raw.Spec.(map[string]any))
		if err != nil {
			allErrors = append(allErrors, fmt.Errorf("%s: parse spec: %w", raw.Source, err))
			continue
		}

		// Validate
		if err := spec.ValidateEntitySpec(entitySpec); err != nil {
			allErrors = append(allErrors, fmt.Errorf("%s: validate: %w", raw.Source, err))
			continue
		}

		// Register
		r.specs[key] = &SpecInfo{
			Metadata: spec.Metadata{
				Name:        raw.Metadata.Name,
				Module:      raw.Metadata.Module,
				Description: raw.Metadata.Description,
				Labels:      raw.Metadata.Labels,
				Annotations: raw.Metadata.Annotations,
			},
			EntitySpec: entitySpec,
			Source:     raw.Source,
		}

		// Register permissions & uses in the permission registry
		module := raw.Metadata.Module
		entityName := raw.Metadata.Name
		for _, action := range entitySpec.Actions {
			usesEntry := permission.BuildUsesEntry(module, entityName, action.Name, action.Uses)
			r.permRegistry.RegisterAction(
				module, entityName, action.Name,
				action.RequiredPermission,
				usesEntry,
				raw.Source,
				action.Audit,
			)
		}

		// Also register standard CRUD permissions if the entity is exposed
		// Standard CRUD perms: {module}.{plural}.{list|view|create|update|delete}
		if len(entitySpec.Expose) > 0 {
			plural := entitySpec.Plural
			if plural == "" {
				plural = entityName + "s"
			}

			// Register each standard CRUD action in the permission registry
			// so PermissionExists() and ctx.auth.has() work for these.
			standardActions := []struct {
				name       string
				permission string
			}{
				{"list", module + "." + plural + ".list"},
				{"view", module + "." + plural + ".view"},
				{"create", module + "." + plural + ".create"},
				{"update", module + "." + plural + ".update"},
				{"delete", module + "." + plural + ".delete"},
			}
			for _, sa := range standardActions {
				r.permRegistry.RegisterAction(
					module, entityName, sa.name,
					sa.permission,
					&permission.UsesEntry{}, // standard CRUD has no uses
					raw.Source,
					false,
				)
			}

			// Set module description from entity metadata
			r.permRegistry.SetModuleDescription(module, raw.Metadata.Description)
		}
	}

	return allErrors
}

// RegisterArtifactManifest registers a single manifest from an artifact
// (non-filesystem source) into the registry. This is the method called by
// the Resource Plane deployer when loading artifacts from the Control Plane.
//
// Unlike LoadEntities, this does NOT read from the filesystem. It accepts
// a pre-parsed raw manifest and its corresponding entity spec.
func (r *Registry) RegisterArtifactManifest(raw manifest.RawManifest, entitySpec *spec.EntitySpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !spec.IsEntityKind(spec.Kind(raw.Kind)) {
		return nil // silently skip non-Entity kinds
	}

	key := entityKey(raw.Metadata.Module, raw.Metadata.Name)

	// Validate
	if err := spec.ValidateEntitySpec(entitySpec); err != nil {
		return fmt.Errorf("%s: validate: %w", raw.Source, err)
	}

	// Register
	r.specs[key] = &SpecInfo{
		Metadata: spec.Metadata{
			Name:        raw.Metadata.Name,
			Module:      raw.Metadata.Module,
			Description: raw.Metadata.Description,
			Labels:      raw.Metadata.Labels,
			Annotations: raw.Metadata.Annotations,
		},
		EntitySpec: entitySpec,
		Source:     raw.Source,
	}

	// Register permissions
	module := raw.Metadata.Module
	entityName := raw.Metadata.Name
	for _, action := range entitySpec.Actions {
		usesEntry := permission.BuildUsesEntry(module, entityName, action.Name, action.Uses)
		r.permRegistry.RegisterAction(
			module, entityName, action.Name,
			action.RequiredPermission,
			usesEntry,
			raw.Source,
			action.Audit,
		)
	}

	// Register standard CRUD permissions if exposed
	if len(entitySpec.Expose) > 0 {
		plural := entitySpec.Plural
		if plural == "" {
			plural = entityName + "s"
		}

		standardActions := []struct {
			name       string
			permission string
		}{
			{"list", module + "." + plural + ".list"},
			{"view", module + "." + plural + ".view"},
			{"create", module + "." + plural + ".create"},
			{"update", module + "." + plural + ".update"},
			{"delete", module + "." + plural + ".delete"},
		}
		for _, sa := range standardActions {
			r.permRegistry.RegisterAction(
				module, entityName, sa.name,
				sa.permission,
				&permission.UsesEntry{},
				raw.Source,
				false,
			)
		}

		r.permRegistry.SetModuleDescription(module, raw.Metadata.Description)
	}

	return nil
}

// SyncSchema applies all registered entity schemas to the database.
// It returns the number of migrations applied and any errors.
func (r *Registry) SyncSchema(ctx context.Context) (int, error) {
	r.mu.RLock()
	entries := make([]db.EntityMigration, 0, len(r.specs))
	for key, info := range r.specs {
		entries = append(entries, db.EntityMigration{
			Metadata:   info.Metadata,
			EntitySpec: *info.EntitySpec,
		})
		_ = key // for debugging
	}
	r.mu.RUnlock()

	if len(entries) == 0 {
		return 0, nil
	}

	runner := db.NewMigrationRunner(r.db, r.driver)
	if err := runner.EnsureSystemTables(ctx); err != nil {
		return 0, fmt.Errorf("ensure system tables: %w", err)
	}

	applied, err := runner.ApplyMigrations(ctx, entries)
	if err != nil {
		return 0, fmt.Errorf("apply migrations: %w", err)
	}

	// Cache TableInfo for each entity
	r.mu.Lock()
	for _, em := range entries {
		key := entityKey(em.Metadata.Module, em.Metadata.Name)
		if info, ok := r.specs[key]; ok {
			ti, err := db.GenerateEntityDDL(em.Metadata, &em.EntitySpec, r.driver)
			if err == nil {
				info.TableInfo = ti
			}
		}
	}
	r.synced = true
	r.mu.Unlock()

	return applied, nil
}

// GetEntityStore returns a cached EntityStore for the given entity.
// The store is lazily created on first access and cached thereafter.
func (r *Registry) GetEntityStore(module, name string) (*db.EntityStore, error) {
	key := entityKey(module, name)

	r.mu.RLock()
	store, ok := r.stores[key]
	info, specOk := r.specs[key]
	r.mu.RUnlock()

	if !specOk {
		return nil, fmt.Errorf("entity %q not found in registry", key)
	}

	if ok {
		return store, nil
	}

	// Lazy-create store
	newStore := db.NewEntityStore(r.db, r.driver, info.Metadata, info.EntitySpec)

	// Wire the cross-module table resolver (2.2.5) — resolves
	// "{module}.{entity}" references to the actual registered table name
	// using the entity spec's Plural, not naive "add s" pluralization.
	newStore.SetTargetTableResolver(func(targetModule, targetEntityName string) (string, error) {
		targetKey := entityKey(targetModule, targetEntityName)
		r.mu.RLock()
		targetInfo, ok := r.specs[targetKey]
		r.mu.RUnlock()
		if !ok {
			return "", fmt.Errorf("target entity %q not registered", targetKey)
		}
		// Use the cached TableInfo if available, otherwise compute it.
		if targetInfo.TableInfo != nil {
			return targetInfo.TableInfo.TableName, nil
		}
		ti, err := db.GenerateEntityDDL(targetInfo.Metadata, targetInfo.EntitySpec, r.driver)
		if err != nil {
			return "", err
		}
		return ti.TableName, nil
	})

	// Wire the referencing-entity resolver for referential integrity
	// enforcement on delete/cancel. Scans all registered entity specs for
	// belongs_to relation fields targeting this entity.
	newStore.SetReferencingEntityResolver(func(currentModule, currentEntity string) ([]db.ReferencingEntity, error) {
		r.mu.RLock()
		defer r.mu.RUnlock()

		var refs []db.ReferencingEntity
		for key, si := range r.specs {
			refModule, refEntity, _ := strings.Cut(key, "/")

			for _, f := range si.EntitySpec.Fields {
				if f.Relation == nil || f.Relation.Type != "belongs_to" {
					continue
				}

				// Resolve the target: "entity" (same module) or "module.entity"
				targetModule := refModule
				targetEntity := f.Relation.Resource
				if dotIdx := strings.Index(f.Relation.Resource, "."); dotIdx >= 0 {
					targetModule = f.Relation.Resource[:dotIdx]
					targetEntity = f.Relation.Resource[dotIdx+1:]
				}

				if targetModule == currentModule && targetEntity == currentEntity {
					// Found a referencing entity — resolve its table name
					var tbl string
					if si.TableInfo != nil {
						tbl = si.TableInfo.TableName
					} else {
						ti, err := db.GenerateEntityDDL(si.Metadata, si.EntitySpec, r.driver)
						if err != nil {
							return nil, fmt.Errorf("compute table for %s: %w", key, err)
						}
						tbl = ti.TableName
					}
					refs = append(refs, db.ReferencingEntity{
						Module:    refModule,
						Entity:    refEntity,
						FieldName: f.Name,
						TableName: tbl,
					})
				}
			}
		}
		return refs, nil
	})

	r.mu.Lock()
	r.stores[key] = newStore
	r.mu.Unlock()

	return newStore, nil
}

// GetEntity returns the registered spec info for a module/name pair.
func (r *Registry) GetEntity(module, name string) (*SpecInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.specs[entityKey(module, name)]
	return info, ok
}

// GetActionSpec returns a named action's spec for a module/entity pair, e.g.
// for cross-resource resource.call() dispatch or route generation.
func (r *Registry) GetActionSpec(module, name, actionName string) (*spec.Action, bool) {
	info, ok := r.GetEntity(module, name)
	if !ok || info.EntitySpec == nil {
		return nil, false
	}
	for i := range info.EntitySpec.Actions {
		if info.EntitySpec.Actions[i].Name == actionName {
			return &info.EntitySpec.Actions[i], true
		}
	}
	return nil, false
}

// GenerateNaturalKey generates a formatted natural key for the given field on
// module/name, per its natural_key_rule (strategy/format/prefix/reset). It is
// the runtime backing for a script's ctx.next_key(field) call — automatic
// natural-key generation on Insert() is handled separately, inline in
// db.EntityStore (natural keys need to be present before required-field
// validation runs on create).
//
// Known gap: rule.ScopeField (branch-scoped numbering, see db.EntityStore.
// generateNaturalKeys) is not wired here — this path has no resource data in
// scope to resolve the scope field's value from, only workspaceID/module/name.
// ctx.next_key() calls therefore always use the workspace-wide scope, same as
// before ScopeField was introduced.
func (r *Registry) GenerateNaturalKey(ctx context.Context, workspaceID, module, name, fieldName string) (string, error) {
	info, ok := r.GetEntity(module, name)
	if !ok || info.EntitySpec == nil {
		return "", fmt.Errorf("entity %s/%s not found", module, name)
	}

	var rule *spec.NaturalKeyRuleDecl
	for _, f := range info.EntitySpec.Fields {
		if f.Name == fieldName && f.NaturalKey && f.NaturalKeyRule != nil {
			rule = f.NaturalKeyRule
			break
		}
	}
	if rule == nil {
		return "", fmt.Errorf("field %q on %s/%s has no natural_key_rule", fieldName, module, name)
	}

	prefix := ""
	if rule.Prefix != nil {
		if rule.Prefix.Value != "" {
			prefix = rule.Prefix.Value
		} else if rule.Prefix.Default != "" {
			prefix = rule.Prefix.Default
		}
	}

	counter := db.NewNaturalKeyCounter(r.db, r.driver)
	return counter.GenerateNaturalKey(ctx, workspaceID, name, fieldName, "", rule.Reset, rule.Format, prefix)
}

// ListEntities returns a sorted summary of all registered entities.
func (r *Registry) ListEntities() []EntityInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]EntityInfo, 0, len(r.specs))
	for _, info := range r.specs {
		char := string(info.EntitySpec.Characteristic)

		tableName := ""
		if info.TableInfo != nil {
			tableName = info.TableInfo.TableName
		}

		result = append(result, EntityInfo{
			Name:           info.Metadata.Name,
			Module:         info.Metadata.Module,
			Kind:           "Entity",
			Characteristic: char,
			TableName:      tableName,
			FieldCount:     len(info.EntitySpec.Fields),
			Source:         info.Source,
			Description:    info.Metadata.Description,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Module != result[j].Module {
			return result[i].Module < result[j].Module
		}
		return result[i].Name < result[j].Name
	})

	return result
}

// GetEntitiesByCharacteristic filters registered entities by characteristic.
func (r *Registry) GetEntitiesByCharacteristic(char spec.Characteristic) []EntityInfo {
	all := r.ListEntities()
	var filtered []EntityInfo
	for _, e := range all {
		if e.Characteristic == string(char) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// Count returns the total number of registered entities.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.specs)
}

// IsSynced returns true if SyncSchema has been called.
func (r *Registry) IsSynced() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.synced
}

// GetPermissionRegistry returns the permission registry for this entity registry.
func (r *Registry) GetPermissionRegistry() *permission.Registry {
	return r.permRegistry
}

// GetModuleFootprint returns the permission footprint for a module.
func (r *Registry) GetModuleFootprint(module string) (*permission.ModuleFootprint, bool) {
	return r.permRegistry.GetModuleFootprint(module)
}
