package app

import (
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/ui"
)

// TestResolve_KafeSpec loads the real kafe tree and resolves its Apps. This is
// the end-to-end half of registered-views validation (plan registered-views.md):
// every `registered_views` entry must name a view/entity that EXISTS in the kafe
// tree, or Resolve fails at boot. A unit test with a synthetic registry cannot
// catch a typo in the example manifests; this can.
func TestResolve_KafeSpec(t *testing.T) {
	const specPath = "../../examples/kafe/spec"

	loaded, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		t.Fatalf("load kafe spec tree: %v", err)
	}
	for _, perr := range loaded.Errors {
		t.Fatalf("parse error: %v", &perr)
	}

	uiReg := ui.NewRegistry()
	if errs := uiReg.LoadDir(specPath); len(errs) > 0 {
		t.Fatalf("load kafe UI manifests: %v", errs)
	}

	apps, err := Resolve(loaded.Manifests, uiReg)
	if err != nil {
		t.Fatalf("resolve kafe apps: %v", err)
	}

	// The two menu-less Apps are exactly the ones that NEED registered_views:
	// nothing else declares what they expose.
	for _, name := range []string{"kafe-qr", "kafe-kds"} {
		a, ok := apps[name]
		if !ok {
			t.Fatalf("app %q not resolved", name)
		}
		if len(a.Spec.RegisteredViews) == 0 {
			t.Errorf("app %q mounts modules but declares no registered_views — its surface would be empty", name)
		}
	}

	// kafe-qr is the anonymous customer surface: it must expose the three pages
	// the QR flow walks through.
	saw := map[string]bool{}
	for _, rv := range apps["kafe-qr"].Spec.RegisteredViews {
		saw[rv.View] = true
	}
	for _, want := range []string{
		"cafe-order/table-open",
		"cafe-order/menu-catalog",
		"cafe-order/order-status-page",
	} {
		if !saw[want] {
			t.Errorf("kafe-qr registered_views missing %q (got %v)", want, saw)
		}
	}
}
