package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

func TestKindMatches(t *testing.T) {
	cases := []struct {
		actual, requested string
		want              bool
	}{
		{"Entity", "entity", true},
		{"Entity", "Entity", true},
		{"Entity", "document", true}, // deprecated alias
		{"Document", "document", true},
		{"Dashboard", "dashboard", true},
		{"Dashboard", "entity", false},
	}
	for _, tc := range cases {
		if got := kindMatches(tc.actual, tc.requested); got != tc.want {
			t.Errorf("kindMatches(%q, %q) = %v, want %v", tc.actual, tc.requested, got, tc.want)
		}
	}
}

func TestSpecVersionOf(t *testing.T) {
	cases := []struct {
		name string
		m    manifest.RawManifest
		want string
	}{
		{"entity version", manifest.RawManifest{Spec: map[string]any{"version": "v1"}}, "v1"},
		{"apiVersion fallback", manifest.RawManifest{APIVersion: "formspec.dev/v1"}, "v1"},
		{"empty", manifest.RawManifest{}, ""},
	}
	for _, tc := range cases {
		if got := specVersionOf(tc.m); got != tc.want {
			t.Errorf("specVersionOf(%s) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestDescribeEntity_Output verifies describeEntity prints fields, actions,
// state machine, and expose for an Entity manifest.
func TestDescribeEntity_Output(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "entity.yaml")
	content := `apiVersion: formspec.dev/v1
kind: Entity
metadata: { name: order, module: billing }
spec:
  version: v1
  characteristic: transaction
  plural: orders
  fields:
    - { name: number, type: string, rules: [required] }
    - { name: total, type: money }
  actions:
    - name: approve
      required_permission: billing.orders.approve
  state_machine:
    field: status
    initial: draft
    states:
      - { name: draft }
      - { name: approved }
    transitions:
      - { from: draft, to: approved }
  expose:
    - { type: rest, actions: [list, find, create, update, delete] }
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := manifest.NewLoader(dir)
	res, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	var m *manifest.RawManifest
	for i := range res.Manifests {
		if res.Manifests[i].Metadata.Name == "order" {
			m = &res.Manifests[i]
			break
		}
	}
	if m == nil {
		t.Fatalf("order manifest not found")
	}

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	describeEntity(m)
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	for _, want := range []string{
		"Characteristic: transaction",
		"Fields (2):",
		"number",
		"total",
		"Actions (1):",
		"approve",
		"billing.orders.approve",
		"State machine (field=status, initial=draft):",
		"draft → approved",
		"Expose:",
		"rest",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("describe output missing %q\n--- output ---\n%s", want, out)
		}
	}
}
