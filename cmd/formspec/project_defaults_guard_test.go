package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoCommandHardcodesProjectDefaults is the guard for the CLASS of bug fixed
// on 2026-09-29, not just its instances.
//
// The bug was not that one command had a wrong default: every data-lifecycle
// command carried its own copy of the literals `formspec dev` uses only when
// formspec-app.yaml is absent —
//
//	specPath := "spec"
//	dsn := "sqlite:.formspec/data.db"
//	workspace := "demo"
//
// so in a project WITH a config file (every `formspec init` project) each
// command quietly opened a different database, or wrote under a different
// tenant, than the server the developer was looking at. Fixing the individual
// call sites without a guard means the next command repeats it — which is
// exactly how the divergence accumulated the first time.
//
// The assertion is on the shape that actually distinguishes a correct command
// from a diverging one: a literal assignment to these names. Values reaching
// them through loadProjectDefaults() are the point, so the test looks for the
// assignments rather than for the strings, which also appear legitimately in
// the helper, in flag help text and in scaffolding templates.
func TestNoCommandHardcodesProjectDefaults(t *testing.T) {
	// Files that legitimately contain the literals: the helper that defines the
	// fallbacks, the dev server that owns them, the scaffold generators that
	// WRITE a formspec-app.yaml containing that DSN, and this test.
	allowed := map[string]bool{
		"project_defaults.go":            true,
		"project_defaults_test.go":       true,
		"project_defaults_guard_test.go": true,
		"dev.go":                         true, // defaultSpecPath/defaultDSN live here; dev reads the config natively
		"dev_config.go":                  true,
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	bad := []string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || allowed[name] {
			continue
		}
		// Scaffolding writes a config FILE whose content includes the DSN, so the
		// literal is data there, not a default the command uses.
		if strings.HasPrefix(name, "generate_") || name == "template_init.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "Usage:") {
				continue
			}
			switch {
			case strings.HasPrefix(trimmed, `dsn := "sqlite:.formspec/data.db"`),
				strings.HasPrefix(trimmed, `specPath := "spec"`),
				strings.HasPrefix(trimmed, `workspaceID := "demo"`),
				strings.HasPrefix(trimmed, `workspace := "demo"`):
				bad = append(bad, name+":"+itoa(i+1)+": "+trimmed)
			}
		}
	}

	if len(bad) > 0 {
		t.Fatalf(
			"these commands hardcode a project default instead of calling loadProjectDefaults()/finishProjectDefaults():\n  %s\n\n"+
				"The literals are what `formspec dev` uses when formspec-app.yaml is ABSENT; using them in a project that has the file "+
				"targets a different database or tenant than the server serves (plan docs_internal/plan/cli-command-config-parity.md).",
			strings.Join(bad, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
