package vendor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/textdiff"
)

// Shadow copy / overrides (todo 13.2, 08-project-layout.md §6.4, technical
// note §5 / D-h): full-replace copies of vendor/local manifest files,
// stored under overrides/{module}/. The fork-base checksum is recorded in
// the lock so vendor updates can warn about drift (§5.3). Only
// presentation kinds may be shadowed (§5.4 whitelist) — enforced at boot.

// OverrideWhitelist lists the kinds that may be shadow-copied (§5.4):
// presentation only. There is no standalone Menu kind (navigation lives in
// App/Module spec), so the file-level whitelist is Form + VisualSpecKind.
// Entity/Service/Workflow etc. have NO shadow-copy path — use Entity
// Extension or the Integrator pattern instead.
var OverrideWhitelist = map[string]bool{
	"Form":           true,
	"VisualSpecKind": true,
}

// OverrideEntry records one adopted shadow copy on the module's lock entry.
type OverrideEntry struct {
	// Kind/Name identify the adopted manifest.
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
	// Origin is where the upstream file lives: "vendors/{effective}" or
	// "modules/{module}" (relative to project root).
	Origin string `yaml:"origin"`
	// RelPath is the manifest path relative to the module root.
	RelPath string `yaml:"rel_path"`
	// BaseChecksum is the sha256 of the upstream file content at adopt
	// time — the "asal fork" marker for drift detection (§5.3).
	BaseChecksum string `yaml:"base_checksum"`
	// AdoptedAt timestamps the adoption.
	AdoptedAt string `yaml:"adopted_at"`
}

// OverridePath is the on-disk location of a shadow copy:
// overrides/{module}/{kind}.{name}.yaml.
func OverridePath(projectRoot, module, kind, name string) string {
	return filepath.Join(projectRoot, "overrides", module, strings.ToLower(kind)+"."+name+".yaml")
}

func fileChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// AdoptResult reports one adoption.
type AdoptResult struct {
	Module       string // effective module name
	Kind         string
	Name         string
	Source       string // upstream file path
	OverridePath string
	BaseChecksum string
}

// Adopt copies an existing manifest into overrides/ and records the fork
// base in the lock (todo 13.2.1, technical note §5.2). The upstream file is
// located by (module, kind, name) through the manifest loader — never by
// guessing paths.
func Adopt(projectRoot, specPath, module, kind, name string) (*AdoptResult, error) {
	kind = normalizeKind(kind)
	if !OverrideWhitelist[kind] {
		return nil, fmt.Errorf(
			"kind %q is not shadow-copyable — whitelist: Form, VisualSpecKind (presentation only, §5.4). "+
				"For entity fields/validation use an Entity Extension; for cross-module behavior use the Integrator pattern", kind)
	}

	upstream, relPath, origin, err := findUpstream(projectRoot, specPath, module, kind, name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(upstream)
	if err != nil {
		return nil, err
	}

	overridePath := OverridePath(projectRoot, module, kind, name)
	if err := os.MkdirAll(filepath.Dir(overridePath), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(overridePath, data, 0644); err != nil {
		return nil, err
	}

	// Record the fork base on the module's lock entry (create a stub entry
	// for local modules that aren't vendored — drift only applies to
	// vendored modules, but the record keeps adopt/diff symmetric).
	lockPath := filepath.Join(projectRoot, "formspec.lock")
	lock, err := LoadLock(lockPath)
	if err != nil {
		return nil, err
	}
	entry := lock.FindByEffectiveName(module)
	if entry == nil {
		lock.Modules = append(lock.Modules, LockEntry{
			Name:        module,
			Version:     "local",
			InstalledAt: nowStamp(),
		})
		entry = lock.FindByEffectiveName(module)
	}
	base := fileChecksum(data)
	ov := OverrideEntry{
		Kind:         kind,
		Name:         name,
		Origin:       origin,
		RelPath:      relPath,
		BaseChecksum: base,
		AdoptedAt:    nowStamp(),
	}
	replaced := false
	for i, existing := range entry.Overrides {
		if existing.Kind == kind && existing.Name == name {
			entry.Overrides[i] = ov
			replaced = true
		}
	}
	if !replaced {
		entry.Overrides = append(entry.Overrides, ov)
	}
	if err := lock.Save(lockPath); err != nil {
		return nil, err
	}

	return &AdoptResult{
		Module:       module,
		Kind:         kind,
		Name:         name,
		Source:       upstream,
		OverridePath: overridePath,
		BaseChecksum: base,
	}, nil
}

// findUpstream locates the manifest file for (module, kind, name) across
// spec/modules and vendors/*, returning its path, module-relative path, and
// origin label.
func findUpstream(projectRoot, specPath, module, kind, name string) (path, relPath, origin string, err error) {
	// Absolute bases — the loader returns absolute file paths, and
	// filepath.Rel requires both sides in the same form (the CLI defaults
	// projectRoot/specPath to "."/"spec" relative forms).
	projectRoot, _ = filepath.Abs(projectRoot)
	specPath, _ = filepath.Abs(specPath)
	loader := manifest.NewLoader(specPath)
	// All installed vendors (active or not) are adoptable.
	vendorBase := filepath.Join(projectRoot, "vendors")
	if entries, err := os.ReadDir(vendorBase); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				loader.AddRoot(filepath.Join(vendorBase, e.Name()))
			}
		}
	}
	res, err := loader.LoadAll()
	if err != nil {
		return "", "", "", err
	}
	for _, m := range res.Manifests {
		if !strings.EqualFold(m.Metadata.Module, module) ||
			!strings.EqualFold(m.Kind, kind) ||
			!strings.EqualFold(m.Metadata.Name, name) {
			continue
		}
		// Source carries a "#<doc-index>" suffix for multi-doc files.
		abs, err := filepath.Abs(strings.SplitN(m.Source, "#", 2)[0])
		if err != nil {
			continue
		}
		// Determine origin + module-relative path.
		if rel, err := filepath.Rel(vendorBase, abs); err == nil && !strings.HasPrefix(rel, "..") {
			parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
			if len(parts) == 2 {
				return abs, parts[1], "vendors/" + parts[0], nil
			}
		}
		modulesBase := filepath.Join(specPath, "modules")
		if rel, err := filepath.Rel(modulesBase, abs); err == nil && !strings.HasPrefix(rel, "..") {
			parts := strings.SplitN(filepath.ToSlash(rel), "/", 2)
			if len(parts) == 2 {
				return abs, parts[1], "modules/" + parts[0], nil
			}
		}
		// Spec-root layout (spec IS the module root).
		if rel, err := filepath.Rel(specPath, abs); err == nil && !strings.HasPrefix(rel, "..") {
			return abs, rel, "modules/" + module, nil
		}
	}
	return "", "", "", fmt.Errorf("no manifest %s/%s/%s found under %s or %s", module, kind, name, specPath, vendorBase)
}

func normalizeKind(kind string) string {
	// Accept "Form" or "form"; normalize to the canonical PascalCase kind.
	for k := range OverrideWhitelist {
		if strings.EqualFold(k, kind) {
			return k
		}
	}
	return kind
}

// OverrideDiff is one shadow copy's comparison against upstream.
type OverrideDiff struct {
	Module   string
	Kind     string
	Name     string
	Upstream string
	Unified  string // "" when identical
	Drift    bool   // upstream changed since adopt (§5.3)
}

// DiffOverride compares a shadow copy against its upstream file and reports
// drift (todo 13.2.3 + §5.3).
func DiffOverride(projectRoot, specPath, module, kind, name string) (*OverrideDiff, error) {
	kind = normalizeKind(kind)
	lock, err := LoadLock(filepath.Join(projectRoot, "formspec.lock"))
	if err != nil {
		return nil, err
	}
	entry := lock.FindByEffectiveName(module)
	if entry == nil {
		return nil, fmt.Errorf("module %q is not recorded in formspec.lock", module)
	}
	var ov *OverrideEntry
	for i := range entry.Overrides {
		if strings.EqualFold(entry.Overrides[i].Kind, kind) && strings.EqualFold(entry.Overrides[i].Name, name) {
			ov = &entry.Overrides[i]
		}
	}
	if ov == nil {
		return nil, fmt.Errorf("no shadow copy for %s/%s in module %s (formspec override adopt first)", kind, name, module)
	}

	overridePath := OverridePath(projectRoot, module, ov.Kind, ov.Name)
	upstreamPath := filepath.Join(projectRoot, filepath.FromSlash(ov.Origin), filepath.FromSlash(ov.RelPath))

	overrideData, err := os.ReadFile(overridePath)
	if err != nil {
		return nil, fmt.Errorf("read shadow copy: %w", err)
	}
	upstreamData, upstreamErr := os.ReadFile(upstreamPath)
	if upstreamErr != nil {
		return nil, fmt.Errorf("read upstream %s: %w", upstreamPath, upstreamErr)
	}

	diff := &OverrideDiff{
		Module:   module,
		Kind:     ov.Kind,
		Name:     ov.Name,
		Upstream: upstreamPath,
		Unified:  textdiff.Unified(string(upstreamData), string(overrideData)),
	}
	// Drift: upstream content changed since adopt (§5.3).
	diff.Drift = fileChecksum(upstreamData) != ov.BaseChecksum
	return diff, nil
}

// DriftWarning is one upstream-changed-since-adopt finding (§5.3).
type DriftWarning struct {
	Module string
	Kind   string
	Name   string
	Detail string
}

// CheckDrift compares every recorded override's base checksum against the
// current upstream (todo 13.2.4). Called at boot (warning to stderr) and by
// install/update.
func CheckDrift(projectRoot string) ([]DriftWarning, error) {
	lock, err := LoadLock(filepath.Join(projectRoot, "formspec.lock"))
	if err != nil {
		return nil, err
	}
	var out []DriftWarning
	for _, entry := range lock.Modules {
		for _, ov := range entry.Overrides {
			upstreamPath := filepath.Join(projectRoot, filepath.FromSlash(ov.Origin), filepath.FromSlash(ov.RelPath))
			data, err := os.ReadFile(upstreamPath)
			if err != nil {
				out = append(out, DriftWarning{
					Module: entry.EffectiveName(), Kind: ov.Kind, Name: ov.Name,
					Detail: "upstream missing: " + upstreamPath,
				})
				continue
			}
			if fileChecksum(data) != ov.BaseChecksum {
				out = append(out, DriftWarning{
					Module: entry.EffectiveName(), Kind: ov.Kind, Name: ov.Name,
					Detail: fmt.Sprintf(
						"shadow copy of %s (adopted from version %s) — upstream changed; "+
							"your copy does NOT automatically receive upstream changes → formspec override diff %s %s %s",
						ov.RelPath, entry.Version, entry.EffectiveName(), strings.ToLower(ov.Kind), ov.Name),
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ValidateOverridesDir enforces the §5.4 whitelist at boot (todo 13.2.2):
// every manifest under overrides/ must be a whitelisted presentation kind —
// a non-whitelisted kind refuses the boot, not just a warning.
func ValidateOverridesDir(projectRoot string) error {
	dir := filepath.Join(projectRoot, "overrides")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	var errs []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p, err))
			return nil
		}
		loader := manifest.NewLoader(".")
		docs, parseErrs := loader.ParseBytes(data, p)
		if len(parseErrs) > 0 {
			errs = append(errs, fmt.Sprintf("%s: %s", p, parseErrs[0].Message))
			return nil
		}
		for _, doc := range docs {
			if !OverrideWhitelist[strings.Title(strings.ToLower(doc.Kind))] &&
				!OverrideWhitelist[doc.Kind] {
				errs = append(errs, fmt.Sprintf(
					"%s: kind %q is not shadow-copyable (whitelist: Form, VisualSpecKind) — "+
						"remove the file or use Entity Extension / Integrator pattern", p, doc.Kind))
			}
		}
		return nil
	})
	if len(errs) > 0 {
		return fmt.Errorf("overrides/ whitelist violation:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// timeNow alias for tests.
var _ = time.Now
