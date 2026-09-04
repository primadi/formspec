// Package vendor — module vendoring (todo 13.1, docs/spec/platform/
// 08-project-layout.md §6, technical note D-e–D-g).
//
// Layout:
//
//	project/
//	  formspec.lock            # lockfile: source, version, checksum, trust_tier
//	  vendors/{module}/        # installed vendor modules — READ-ONLY
//	  overrides/{module}/      # shadow copies (13.2, belum di batch ini)
//
// Activation model: vendor modules are inactive by default. `formspec module
// install` writes a structured marker block into the App manifest; the entry
// is commented (inactive) unless --use. Uncomment to activate. Re-install
// preserves the active/inactive state (D-g) — only the version in the marker
// header and the lock entry are updated.
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

	"gopkg.in/yaml.v3"
)

// Lock is the formspec.lock document.
type Lock struct {
	Modules []LockEntry `yaml:"modules"`
}

// LockEntry is one installed vendor module.
type LockEntry struct {
	// Source is the install origin (git URL, folder path, tarball path,
	// or registry ref — 13.3).
	Source string `yaml:"source"`
	// Name is the module's metadata.name (as declared by its author).
	Name string `yaml:"name"`
	// Alias is the effective name when Name conflicts with another
	// installed/local module (Opsi B — fixed at install time). Empty = no
	// alias; effective name == Name.
	Alias string `yaml:"alias,omitempty"`
	// Version is the installed version tag (from source ref or "local").
	Version string `yaml:"version"`
	// Checksum is the sha256 tree hash of vendors/{dir} at install time.
	Checksum string `yaml:"checksum"`
	// Signature is the ed25519 signature over the checksum (13.3.6).
	Signature string `yaml:"signature,omitempty"`
	// TrustTier: official | verified | community (default until signed).
	TrustTier string `yaml:"trust_tier,omitempty"`
	// InstalledAt is the last install/update timestamp.
	InstalledAt string `yaml:"installed_at"`
	// Overrides records shadow copies adopted from this module (todo 13.2,
	// §6.4): kind/name + fork-base checksum for drift detection (§5.3).
	Overrides []OverrideEntry `yaml:"overrides,omitempty"`
}

// EffectiveName returns the alias when set, else Name.
func (e LockEntry) EffectiveName() string {
	if e.Alias != "" {
		return e.Alias
	}
	return e.Name
}

// DirName is the vendors/ subdirectory for this entry — always the effective
// name so the disk layout matches what the boot loader registers.
func (e LockEntry) DirName() string { return e.EffectiveName() }

// LoadLock reads formspec.lock from path. A missing file yields an empty
// lock (no error) — vendoring is optional.
func LoadLock(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Lock{}, nil
		}
		return nil, err
	}
	var lock Lock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &lock, nil
}

// Save writes the lock file (0644, sorted by effective name for stable diffs).
func (l *Lock) Save(path string) error {
	sort.Slice(l.Modules, func(i, j int) bool {
		return l.Modules[i].EffectiveName() < l.Modules[j].EffectiveName()
	})
	data, err := yaml.Marshal(l)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// FindBySource returns the lock entry for a source, if installed.
func (l *Lock) FindBySource(source string) *LockEntry {
	for i := range l.Modules {
		if l.Modules[i].Source == source {
			return &l.Modules[i]
		}
	}
	return nil
}

// FindByEffectiveName returns the lock entry whose effective name matches.
func (l *Lock) FindByEffectiveName(name string) *LockEntry {
	for i := range l.Modules {
		if l.Modules[i].EffectiveName() == name {
			return &l.Modules[i]
		}
	}
	return nil
}

// TreeChecksum computes a deterministic sha256 over a directory tree:
// sorted relative paths (slash-separated), each hashed as path + NUL +
// content + NUL. Directory entries themselves do not contribute.
func TreeChecksum(dir string) (string, error) {
	h := sha256.New()
	var files []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, p := range files {
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		h.Write([]byte(filepath.ToSlash(rel)))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// nowStamp is overridable in tests.
var nowStamp = func() string { return time.Now().UTC().Format(time.RFC3339) }

// NormalizeSource canonicalizes a source string for lock identity: git URLs
// keep their form; local paths are made absolute so re-install from a
// different cwd still matches the lock entry.
func NormalizeSource(source string) (string, error) {
	switch {
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "git@") || strings.HasSuffix(source, ".git"):
		return source, nil
	default:
		abs, err := filepath.Abs(source)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
}
