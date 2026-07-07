package permission

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry indexes all permission declarations across loaded modules.
//
// It is the runtime source of truth for:
//   - What permissions exist (for ctx.auth.has() validation)
//   - What uses are declared (for runtime enforcement)
//   - What footprint a module has (for install-time consent)
//
// Thread-safe (sync.RWMutex). Populated during entity registration.
type Registry struct {
	mu        sync.RWMutex
	modules   map[string]*ModuleFootprint // module name → footprint
	permIndex map[string]*PermissionEntry // permission key → entry (for fast lookup)
	usesIndex map[string][]UsesEntry      // "module/entity" → uses entries
}

// NewRegistry creates an empty permission registry.
func NewRegistry() *Registry {
	return &Registry{
		modules:   make(map[string]*ModuleFootprint),
		permIndex: make(map[string]*PermissionEntry),
		usesIndex: make(map[string][]UsesEntry),
	}
}

// RegisterAction indexes one action's required_permission and uses.
//
// Parameters:
//   - module: owning module name
//   - entity: entity name (or "service" for Service kinds)
//   - action: action name
//   - perm: the required_permission string (may be empty/"public")
//   - uses: the uses declaration (may be nil)
//   - source: file path for audit trail
//   - audit: whether the action is audited
func (r *Registry) RegisterAction(module, entity, action, perm string, uses *UsesEntry, source string, audit bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize module footprint if not exists
	if _, ok := r.modules[module]; !ok {
		r.modules[module] = &ModuleFootprint{
			Module: module,
		}
	}

	// Register required_permission
	if perm != "" && perm != "public" {
		qualified := AutoPrefixPermission(perm, module)
		entry := &PermissionEntry{
			Module: module,
			Entity: entity,
			Action: action,
			Type:   PermAction,
			Key:    qualified,
			Source: source,
			Audit:  audit,
		}
		r.modules[module].Permissions = append(r.modules[module].Permissions, *entry)
		r.permIndex[qualified] = entry
	}

	// Register uses
	if uses != nil {
		r.modules[module].Uses = append(r.modules[module].Uses, *uses)
		key := module + "/" + entity
		r.usesIndex[key] = append(r.usesIndex[key], *uses)

		// Detect cross-module writes
		for _, res := range uses.Resources {
			if res.Mode == AccessWrite {
				resModule, _, _, err := ParseResourceTarget(res.Target, module)
				if err == nil && resModule != module {
					cmKey := fmt.Sprintf("%s writes to %s (via %s.%s)", module, res.Target, entity, action)
					r.modules[module].CrossModuleWrites = append(r.modules[module].CrossModuleWrites, cmKey)
				}
			}
		}
	}

	return nil
}

// GetModuleFootprint returns the aggregated footprint for a module.
func (r *Registry) GetModuleFootprint(module string) (*ModuleFootprint, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fp, ok := r.modules[module]
	return fp, ok
}

// AllFootprints returns all registered module footprints, sorted by module name.
func (r *Registry) AllFootprints() []*ModuleFootprint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ModuleFootprint, 0, len(r.modules))
	for _, fp := range r.modules {
		result = append(result, fp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Module < result[j].Module
	})
	return result
}

// ListModules returns all module names with registered permissions.
func (r *Registry) ListModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modules := make([]string, 0, len(r.modules))
	for m := range r.modules {
		modules = append(modules, m)
	}
	sort.Strings(modules)
	return modules
}

// FindPermission looks up a permission entry by its fully-qualified key.
// Returns nil if not found.
func (r *Registry) FindPermission(key string) *PermissionEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.permIndex[key]
}

// PermissionExists checks if a permission key is registered in any module.
func (r *Registry) PermissionExists(key string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.permIndex[key]
	return ok
}

// UsesFor returns all uses entries for a given module/entity key.
func (r *Registry) UsesFor(module, entity string) []UsesEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := module + "/" + entity
	entries, _ := r.usesIndex[key]
	return entries
}

// TotalPermissions returns the total count of registered permission entries.
func (r *Registry) TotalPermissions() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.permIndex)
}

// TotalModules returns the total count of registered modules.
func (r *Registry) TotalModules() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.modules)
}

// String returns a summary of all registered permissions.
func (r *Registry) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Permission Registry: %d modules, %d permissions\n",
		len(r.modules), len(r.permIndex)))
	for _, m := range r.AllFootprints() {
		b.WriteString(m.String())
	}
	return b.String()
}

// SetModuleDescription sets the human-readable description for a module.
func (r *Registry) SetModuleDescription(module, description string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if fp, ok := r.modules[module]; ok {
		fp.Description = description
	} else {
		r.modules[module] = &ModuleFootprint{
			Module:      module,
			Description: description,
		}
	}
}
