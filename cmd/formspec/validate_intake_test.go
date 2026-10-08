package main

import (
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// Plan: docs_internal/plan/intake-challenge-pow.md.
//
// `challenge: true` is a request to be gated; only an App can answer it. These
// tests pin the cross-manifest half of that rule — the half that can see the
// App and the Entity at the same time, which is exactly what a single-manifest
// check cannot.

func appWithIntake(access spec.AppAccess, withPolicy bool) manifest.RawManifest {
	appSpec := map[string]any{
		"root_url": "/",
		"modules":  []any{"m"},
		"access":   string(access),
	}
	if withPolicy {
		appSpec["intake"] = map[string]any{
			"challenge": map[string]any{
				"provider": "pow",
				"mode":     "always",
			},
		}
	}
	return manifest.RawManifest{Kind: "App", Metadata: manifest.RawMetadata{Name: "app"}, Spec: appSpec}
}

func entityWithChallenge(optedIn bool) manifest.RawManifest {
	action := map[string]any{"name": "create"}
	if optedIn {
		action["challenge"] = true
	}
	return manifest.RawManifest{
		Kind:     "Entity",
		Metadata: manifest.RawMetadata{Name: "order", Module: "m"},
		Spec: map[string]any{
			"version": "v1",
			"plural":  "orders",
			"actions": []any{action},
		},
	}
}

func TestValidateIntakeOptIns(t *testing.T) {
	cases := map[string]struct {
		manifests []manifest.RawManifest
		wantFail  bool
	}{
		"opt-in with a public policy is fine": {
			[]manifest.RawManifest{appWithIntake(spec.AppAccessPublic, true), entityWithChallenge(true)},
			false,
		},
		"opt-in with no policy anywhere is an error": {
			[]manifest.RawManifest{appWithIntake(spec.AppAccessPublic, false), entityWithChallenge(true)},
			true,
		},
		"opt-in with no App at all is an error": {
			[]manifest.RawManifest{entityWithChallenge(true)},
			true,
		},
		"no opt-in needs no policy": {
			[]manifest.RawManifest{entityWithChallenge(false)},
			false,
		},
		// A PRIVATE App's policy does not cover anonymous intake: a private
		// surface has no anonymous caller to gate, so the opt-in would still be
		// inert. Treating it as coverage would hide the mistake.
		"a private App's policy does not count": {
			[]manifest.RawManifest{appWithIntake(spec.AppAccessPrivate, true), entityWithChallenge(true)},
			true,
		},
	}
	for name, c := range cases {
		got := validateIntakeOptIns(c.manifests)
		if c.wantFail && len(got) == 0 {
			t.Errorf("%s: expected a rejection, got none", name)
		}
		if !c.wantFail && len(got) != 0 {
			t.Errorf("%s: expected no rejection, got %v", name, got)
		}
		if c.wantFail && len(got) > 0 {
			for src, msg := range got {
				if msg == "" {
					t.Errorf("%s: rejection for %s carries no message", name, src)
				}
			}
		}
	}
}
