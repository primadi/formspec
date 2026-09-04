package vendor

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/primadi/formspec/internal/manifest"
	"gopkg.in/yaml.v3"
)

// InstallResult reports what an install did.
type InstallResult struct {
	Entry       LockEntry
	Dir         string // vendors/{dir}
	Active      bool   // marker state after install
	Updated     bool   // true when re-installing an existing source
	AppManifest string // App manifest file that received the marker
}

// Options configures Install.
type Options struct {
	// ProjectRoot is the directory containing formspec.lock + vendors/.
	ProjectRoot string
	// AppManifestPath is the App manifest receiving the marker block.
	// When empty, the first kind: App manifest under SpecPath is used.
	AppManifestPath string
	// SpecPath is the spec directory (for local-module conflict scan and
	// App manifest discovery).
	SpecPath string
	// Version tags the install (git ref/tarball name/"local").
	Version string
	// Use activates the module immediately (--use).
	Use bool
	// SourceOverride replaces the lock/marker source label (registry
	// installs record the registry ref instead of the temp tarball path).
	SourceOverride string
	// Signature/TrustTier are recorded from the registry on install --from
	// (13.3.8) so the lock reflects the verified provenance.
	Signature string
	TrustTier string
}

// Install runs the full flow (todo 13.1.2):
// fetch → stage → validate (module.yaml) → copy to vendors/ → checksum →
// lock → marker. Re-install preserves activation state (D-g) and updates
// version + checksum only.
func Install(ctx context.Context, source string, opts Options) (*InstallResult, error) {
	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("source is required")
	}
	normSource, err := NormalizeSource(source)
	if err != nil {
		return nil, err
	}
	if opts.SourceOverride != "" {
		normSource = opts.SourceOverride
	}
	lockPath := filepath.Join(opts.ProjectRoot, "formspec.lock")
	lock, err := LoadLock(lockPath)
	if err != nil {
		return nil, err
	}

	// ── Fetch → stage ──
	stage, err := os.MkdirTemp("", "formspec-vendor-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	version := opts.Version
	switch {
	case isGitURL(source):
		if err := gitClone(ctx, source, stage); err != nil {
			return nil, err
		}
		if version == "" {
			version = "git"
		}
	case strings.HasSuffix(source, ".tar.gz") || strings.HasSuffix(source, ".tgz"):
		if err := extractTarball(source, stage); err != nil {
			return nil, err
		}
		if version == "" {
			version = filepath.Base(source)
		}
	default:
		// Local folder — copy the tree.
		if err := copyTree(source, stage); err != nil {
			return nil, fmt.Errorf("read source folder: %w", err)
		}
		if version == "" {
			version = "local"
		}
	}

	// ── Locate + validate the module root ──
	moduleRoot, err := FindModuleRoot(stage)
	if err != nil {
		return nil, err
	}
	name, err := readModuleName(moduleRoot)
	if err != nil {
		return nil, err
	}

	// ── Alias (Opsi B): conflict against lock + local modules ──
	taken, err := takenNames(opts.SpecPath, lock)
	if err != nil {
		return nil, err
	}
	// The module's own previous entry does not conflict with itself.
	if prev := lock.FindBySource(normSource); prev != nil {
		delete(taken, prev.EffectiveName())
	}
	alias := ResolveAlias(name, normSource, sortedKeys(taken))

	entry := LockEntry{
		Source:      normSource,
		Name:        name,
		Alias:       alias,
		Version:     version,
		TrustTier:   "community", // signature verification lands with 13.3.6
		InstalledAt: nowStamp(),
	}
	if opts.TrustTier != "" {
		entry.TrustTier = opts.TrustTier
	}
	if opts.Signature != "" {
		entry.Signature = opts.Signature
	}

	// ── Copy to vendors/{effective}/ ──
	dir := filepath.Join(opts.ProjectRoot, "vendors", entry.DirName())
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := copyTree(moduleRoot, dir); err != nil {
		return nil, err
	}
	// Normalize the vendored module.yaml to the EFFECTIVE name (todo
	// 13.1.4): the boot loader registers entities by metadata.module, so
	// an aliased module must declare its effective name on disk. Done
	// BEFORE the checksum so the lock covers the normalized tree.
	if err := rewriteModuleName(dir, name, entry.EffectiveName()); err != nil {
		return nil, err
	}
	checksum, err := TreeChecksum(dir)
	if err != nil {
		return nil, err
	}
	entry.Checksum = checksum

	// ── Lock update (preserve signature/trust_tier/overrides on re-install) ──
	updated := false
	if prev := lock.FindBySource(normSource); prev != nil {
		updated = true
		entry.Signature = prev.Signature
		if opts.Signature != "" {
			entry.Signature = opts.Signature
		}
		if prev.TrustTier != "" {
			entry.TrustTier = prev.TrustTier
		}
		// Shadow-copy records survive re-install (todo 13.2.4): drift
		// detection needs the fork-base checksums across version updates.
		entry.Overrides = prev.Overrides
		// Replace in place.
		for i := range lock.Modules {
			if lock.Modules[i].Source == normSource {
				lock.Modules[i] = entry
			}
		}
	} else {
		lock.Modules = append(lock.Modules, entry)
	}
	if err := lock.Save(lockPath); err != nil {
		return nil, err
	}

	// ── Marker in App manifest (preserve active state on update, D-g) ──
	appPath := opts.AppManifestPath
	if appPath == "" {
		appPath, err = findAppManifest(opts.SpecPath)
		if err != nil {
			return nil, err
		}
	}
	appData, err := os.ReadFile(appPath)
	if err != nil {
		return nil, fmt.Errorf("read App manifest: %w", err)
	}
	activeIfNew := opts.Use
	newContent := UpsertMarker(string(appData), normSource, version, entry.EffectiveName(), activeIfNew)
	if err := os.WriteFile(appPath, []byte(newContent), 0644); err != nil {
		return nil, err
	}
	active := markerActive(newContent, normSource, opts.Use)

	return &InstallResult{
		Entry:       entry,
		Dir:         dir,
		Active:      active,
		Updated:     updated,
		AppManifest: appPath,
	}, nil
}

// Uninstall removes a vendor module: vendors/ dir + lock entry + marker
// block (todo 13.1.5). The module is identified by effective name.
func Uninstall(projectRoot, specPath, effectiveName string) (bool, error) {
	lockPath := filepath.Join(projectRoot, "formspec.lock")
	lock, err := LoadLock(lockPath)
	if err != nil {
		return false, err
	}
	entry := lock.FindByEffectiveName(effectiveName)
	if entry == nil {
		return false, nil
	}

	// Remove vendors dir.
	if err := os.RemoveAll(filepath.Join(projectRoot, "vendors", entry.DirName())); err != nil {
		return false, err
	}
	// Remove lock entry.
	var kept []LockEntry
	for _, m := range lock.Modules {
		if m.EffectiveName() != effectiveName {
			kept = append(kept, m)
		}
	}
	lock.Modules = kept
	if err := lock.Save(lockPath); err != nil {
		return false, err
	}
	// Remove marker block.
	appPath, err := findAppManifest(specPath)
	if err == nil {
		if data, err := os.ReadFile(appPath); err == nil {
			newContent, removed := RemoveMarker(string(data), entry.Source)
			if removed {
				if err := os.WriteFile(appPath, []byte(newContent), 0644); err != nil {
					return true, err
				}
			}
		}
	}
	return true, nil
}

// VerifyResult is one module's integrity check.
type VerifyResult struct {
	EffectiveName string
	OK            bool
	Reason        string // "" when OK
}

// rewriteModuleName sets metadata.name in the vendored module.yaml to the
// effective name (alias normalization, todo 13.1.4). No-op when the name
// already matches.
func rewriteModuleName(dir, oldName, effectiveName string) error {
	if oldName == effectiveName {
		return nil
	}
	for _, candidate := range []string{"module.yaml", "module.yml"} {
		p := filepath.Join(dir, candidate)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		md, ok := doc["metadata"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s: no metadata block", p)
		}
		md["name"] = effectiveName
		out, err := yaml.Marshal(doc)
		if err != nil {
			return err
		}
		return os.WriteFile(p, out, 0644)
	}
	return fmt.Errorf("vendors/%s: module.yaml not found", filepath.Base(dir))
}

// ActiveModules returns the effective names of vendor modules whose marker
// entry in the App manifest is uncommented (todo 13.1.4 — hanya module
// aktif yang di-register saat boot). Entries not present in the lock are
// skipped (stale markers).
func ActiveModules(projectRoot, specPath string) ([]string, error) {
	lock, err := LoadLock(filepath.Join(projectRoot, "formspec.lock"))
	if err != nil {
		return nil, err
	}
	appPath, err := findAppManifest(specPath)
	if err != nil {
		return nil, nil // no App manifest — nothing active
	}
	data, err := os.ReadFile(appPath)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, m := range FindMarkers(string(data)) {
		if !m.Active || !m.EntrySet() {
			continue
		}
		if lock.FindBySource(m.Source) != nil {
			out = append(out, m.Entry)
		}
	}
	return out, nil
} // Verify recomputes tree checksums against the lock (todo 13.1.6) — manual
// modifications of vendors/ are detected here.
func Verify(projectRoot string) ([]VerifyResult, error) {
	lock, err := LoadLock(filepath.Join(projectRoot, "formspec.lock"))
	if err != nil {
		return nil, err
	}
	var out []VerifyResult
	for _, entry := range lock.Modules {
		res := VerifyResult{EffectiveName: entry.EffectiveName(), OK: true}
		dir := filepath.Join(projectRoot, "vendors", entry.DirName())
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			res.OK = false
			res.Reason = "vendors/" + entry.DirName() + " missing"
		} else {
			checksum, err := TreeChecksum(dir)
			if err != nil {
				res.OK = false
				res.Reason = err.Error()
			} else if checksum != entry.Checksum {
				res.OK = false
				res.Reason = "checksum mismatch — vendors/ was modified after install"
			}
		}
		out = append(out, res)
	}
	return out, nil
}

// ─── helpers ───

func isGitURL(source string) bool {
	return strings.HasPrefix(source, "git@") ||
		(strings.Contains(source, "://") && strings.HasSuffix(source, ".git"))
}

func gitClone(ctx context.Context, url, dest string) error {
	cmd := execCommand(ctx, "git", "clone", "--depth", "1", url, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func extractTarball(path, dest string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		// Defend against path traversal.
		target := filepath.Join(dest, filepath.Clean("/"+hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("tarball entry escapes destination: %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

// FindModuleRoot locates the directory containing module.yaml: the stage
// root itself, or its single child directory (git/tarball layouts).
func FindModuleRoot(stage string) (string, error) {
	if fileExists(filepath.Join(stage, "module.yaml")) {
		return stage, nil
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, filepath.Join(stage, e.Name()))
		}
	}
	if len(dirs) == 1 && fileExists(filepath.Join(dirs[0], "module.yaml")) {
		return dirs[0], nil
	}
	return "", fmt.Errorf("no module.yaml found at source root or single subdirectory")
}

// readModuleName parses module.yaml's metadata.name via the manifest loader
// — the same parser the engine uses.
func readModuleName(moduleRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(moduleRoot, "module.yaml"))
	if err != nil {
		return "", err
	}
	loader := manifest.NewLoader(".")
	docs, errs := loader.ParseBytes(data, "module.yaml")
	if len(errs) > 0 {
		return "", fmt.Errorf("parse module.yaml: %v", errs[0].Message)
	}
	if len(docs) == 0 {
		return "", fmt.Errorf("module.yaml is empty")
	}
	if docs[0].Metadata.Name == "" {
		return "", fmt.Errorf("module.yaml missing metadata.name")
	}
	return docs[0].Metadata.Name, nil
}

// ReadModuleName is the exported form of readModuleName (used by the
// publish CLI to resolve the module's registry name).
func ReadModuleName(moduleRoot string) (string, error) {
	return readModuleName(moduleRoot)
}

// takenNames collects every effective name in use: lock entries + local
// module dirs under spec/modules (or spec root when modules/ is absent).
func takenNames(specPath string, lock *Lock) (map[string]bool, error) {
	taken := map[string]bool{}
	for _, m := range lock.Modules {
		taken[m.EffectiveName()] = true
	}
	// Local modules: scan spec/modules/* (fallback: spec root).
	localRoot := filepath.Join(specPath, "modules")
	if _, err := os.Stat(localRoot); os.IsNotExist(err) {
		localRoot = specPath
	}
	entries, err := os.ReadDir(localRoot)
	if err != nil {
		return taken, nil // no local modules — not an error
	}
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			taken[e.Name()] = true
		}
	}
	return taken, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// findAppManifest locates the first kind: App manifest under specPath.
func findAppManifest(specPath string) (string, error) {
	var found string
	filepath.WalkDir(specPath, func(p string, d os.DirEntry, err error) error {
		if err != nil || found != "" || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		loader := manifest.NewLoader(".")
		docs, _ := loader.ParseBytes(data, p)
		for _, doc := range docs {
			if strings.EqualFold(doc.Kind, "App") {
				found = p
				return filepath.SkipAll
			}
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("no kind: App manifest found under %s (pass --app)", specPath)
	}
	return found, nil
}

// markerActive re-parses the marker after upsert to report the final state.
func markerActive(content, source string, fallback bool) bool {
	for _, m := range FindMarkers(content) {
		if m.Source == source {
			return m.Active
		}
	}
	return fallback
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
