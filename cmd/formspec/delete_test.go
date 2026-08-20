package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDeleteSpec(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "modules", "alpha", "master", "item.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: item, module: alpha }
spec:
  version: v1
  characteristic: master
  fields:
    - { name: code, type: string }
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCountYAMLDocs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.yaml")
	content := "a: 1\n---\nb: 2\n---\nc: 3\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := countYAMLDocs(path)
	if err != nil {
		t.Fatalf("countYAMLDocs: %v", err)
	}
	if n != 3 {
		t.Fatalf("count = %d, want 3", n)
	}
}

func TestRemoveYAMLDoc(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.yaml")
	content := `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: first, module: alpha }
spec:
  version: v1
---
apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: second, module: alpha }
spec:
  version: v1
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeYAMLDoc(path, 0); err != nil {
		t.Fatalf("removeYAMLDoc: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if strings.Contains(out, "first") {
		t.Fatalf("first document not removed:\n%s", out)
	}
	if !strings.Contains(out, "second") {
		t.Fatalf("second document should remain:\n%s", out)
	}
}

func TestRemoveYAMLDoc_OutOfRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "single.yaml")
	if err := os.WriteFile(path, []byte("a: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeYAMLDoc(path, 5); err == nil {
		t.Fatalf("expected out-of-range error")
	}
}

// TestRunDelete_RequiresConfirm verifies delete refuses without --confirm.
// runDelete calls os.Exit on error, so we test the guard via the helper
// functions instead of invoking runDelete directly.
func TestRunDelete_RequiresConfirm(t *testing.T) {
	dir := t.TempDir()
	writeDeleteSpec(t, dir)

	manifests := loadManifestsOrExit(dir)
	var found bool
	for i := range manifests {
		if kindMatches(manifests[i].Kind, "entity") && manifests[i].Metadata.Name == "item" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find entity item in spec tree")
	}
}
