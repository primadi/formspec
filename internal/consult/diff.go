package consult

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/primadi/formspec/internal/textdiff"
)

// osStdin/osStdout indirections keep the REPL testable.
func osStdin() *os.File  { return os.Stdin }
func osStdout() *os.File { return os.Stdout }

// DraftDiff is one draft file and its unified diff against the real tree.
type DraftDiff struct {
	Path    string // spec-relative draft path
	Unified string // unified diff text ("" for new files)
	IsNew   bool
}

// DiffDrafts compares the session's draft/ directory against the real spec
// tree (10.4.2, docs/ai/02 §4): spec-to-spec unified diff, no compile step.
// The draft layout mirrors the spec tree, so the real path is derived by
// replacing the draft root with the spec root.
func DiffDrafts(sessionDir string) ([]DraftDiff, error) {
	draftRoot := filepath.Join(sessionDir, "draft")
	specRoot := findSpecRoot(sessionDir)
	if specRoot == "" {
		return nil, nil
	}
	var out []DraftDiff
	err := filepath.WalkDir(draftRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(draftRoot, p)
		if err != nil {
			return err
		}
		target := filepath.Join(specRoot, rel)
		newContent, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		oldContent, targetErr := os.ReadFile(target)
		isNew := os.IsNotExist(targetErr)
		if !isNew && targetErr != nil {
			return targetErr
		}
		out = append(out, DraftDiff{
			Path:    filepath.ToSlash(rel),
			Unified: unifiedDiff(string(oldContent), string(newContent)),
			IsNew:   isNew,
		})
		return nil
	})
	return out, err
}

// findSpecRoot locates the spec directory relative to the session dir:
// .formspec/consult/{id} → project root → spec/.
func findSpecRoot(sessionDir string) string {
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(sessionDir)))
	for _, candidate := range []string{
		filepath.Join(projectRoot, "spec"),
		projectRoot, // layout where the spec IS the project root
	} {
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			if hasYAML(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// hasYAML reports whether dir contains at least one .yaml/.yml file.
func hasYAML(dir string) bool {
	found := false
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".yaml" || ext == ".yml" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// unifiedDiff renders a unified diff between two texts (delegates to the
// shared internal/textdiff package — the same implementation powers
// `formspec override diff`).
func unifiedDiff(oldText, newText string) string {
	return textdiff.Unified(oldText, newText)
}
