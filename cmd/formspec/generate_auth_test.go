package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAuthModule(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "external", "auth")

	if err := generateAuthModule(target, false); err != nil {
		t.Fatalf("generateAuthModule: %v", err)
	}

	// Verify the expected files exist.
	for _, rel := range []string{
		"module.yaml",
		"master/user/entity.yaml",
		"transaction/session/entity.yaml",
		"master/role/entity.yaml",
		"master/api-key/entity.yaml",
		"transaction/app-membership/entity.yaml",
		"master/workspace/entity.yaml",
	} {
		path := filepath.Join(target, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}

	// Second run without --force must fail (files exist).
	if err := generateAuthModule(target, false); err == nil {
		t.Error("expected error on second run without --force")
	}

	// With --force it succeeds.
	if err := generateAuthModule(target, true); err != nil {
		t.Fatalf("generateAuthModule --force: %v", err)
	}
}

func TestGenerateAuthModule_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "external", "auth")

	if err := generateAuthModule(target, false); err != nil {
		t.Fatalf("generateAuthModule: %v", err)
	}

	// The generated manifests must load without errors through the real
	// manifest loader (validates kind + apiVersion).
	loader := newManifestLoader(target)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(res.Errors) > 0 {
		for _, e := range res.Errors {
			t.Errorf("manifest error: %v", e)
		}
	}
	if len(res.Manifests) != 13 {
		t.Errorf("expected 13 manifests (module + 6 entities + 3 forms + 2 tables + 1 page), got %d", len(res.Manifests))
	}
}
