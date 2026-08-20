package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunNewModule_ScaffoldsModule verifies `formspec new module <name>`
// writes a valid Module manifest at spec/modules/{name}/module.yaml.
func TestRunNewModule_ScaffoldsModule(t *testing.T) {
	work := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	runNewModule([]string{"cafe-master"})

	path := filepath.Join("spec", "modules", "cafe-master", "module.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read module.yaml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "kind: Module") {
		t.Fatalf("module.yaml missing kind: Module\n%s", content)
	}
	if !strings.Contains(content, "name: cafe-master") {
		t.Fatalf("module.yaml missing name\n%s", content)
	}
}

// TestRunNewEntity_ScaffoldsEntity verifies `formspec new entity <name>
// --module <module>` writes a valid Entity manifest at
// spec/modules/{module}/{characteristic}/{entity}/entity.yaml with basic
// fields and a default expose block.
func TestRunNewEntity_ScaffoldsEntity(t *testing.T) {
	work := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	runNewEntity([]string{"menu-item", "--module", "cafe-master"})

	path := filepath.Join("spec", "modules", "cafe-master", "master", "menu-item", "entity.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read entity.yaml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "kind: Entity") {
		t.Fatalf("entity.yaml missing kind: Entity\n%s", content)
	}
	if !strings.Contains(content, "name: menu-item") {
		t.Fatalf("entity.yaml missing name\n%s", content)
	}
	if !strings.Contains(content, "module: cafe-master") {
		t.Fatalf("entity.yaml missing module\n%s", content)
	}
	if !strings.Contains(content, "characteristic: master") {
		t.Fatalf("entity.yaml missing characteristic\n%s", content)
	}
	if !strings.Contains(content, "expose:") {
		t.Fatalf("entity.yaml missing expose block\n%s", content)
	}
}

// TestRunNewEntity_InvalidCharacteristic verifies an invalid characteristic
// is rejected.
func TestRunNewEntity_InvalidCharacteristic(t *testing.T) {
	work := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	// Should exit(1) — capture via a subprocess-free check: runNewEntity
	// calls os.Exit on invalid characteristic, so we can't call it directly
	// in-process. Instead verify the validation logic via the valid set.
	if !validCharacteristics["master"] || !validCharacteristics["transaction"] ||
		!validCharacteristics["reference"] || !validCharacteristics["summary"] {
		t.Fatalf("validCharacteristics missing a closed-set member")
	}
	if validCharacteristics["bogus"] {
		t.Fatalf("validCharacteristics should reject unknown characteristic")
	}
}

// TestDetectModule verifies module detection from a CWD nested under
// spec/modules/{module}/.
func TestDetectModule(t *testing.T) {
	work := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	// Create a nested path under spec/modules/alpha/...
	nested := filepath.Join("spec", "modules", "alpha", "master", "thing")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	if got := detectModule(); got != "alpha" {
		t.Fatalf("detectModule() = %q, want alpha", got)
	}
}
