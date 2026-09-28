package ui

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// `submit.call` makes a Form's submit a SERVICE call rather than an entity write
// (kafe P3). Only the SHAPE is checkable in this registry — it knows entities, not
// services, so whether the action exists is a cross-manifest question. A
// malformed ref, though, is a manifest bug that would otherwise surface as a
// confusing runtime call.
//
// Run with: go test ./internal/ui/ -run TestValidate_SubmitCall
func TestValidate_SubmitCall(t *testing.T) {
	form := func(submit string) string {
		return `
apiVersion: formspec.dev/v1alpha1
kind: Form
metadata: { name: checkin, module: billing }
spec:
  entity: order
  sections:
    - title: T
      fields:
        - { field: number }
` + submit
	}

	cases := []struct {
		name         string
		submit       string
		wantErr      bool
		wantContains string
	}{
		{
			name: "well-formed module.service.action is accepted",
			submit: `  submit:
    call: billing.table-access.open
    redirect: "/menu/{response.guest_token}"
`,
		},
		{
			name: "two segments is refused",
			submit: `  submit:
    call: table-access.open
`,
			wantErr:      true,
			wantContains: "3 dot-separated segments",
		},
		{
			name: "four segments is refused",
			submit: `  submit:
    call: a.b.c.d
`,
			wantErr:      true,
			wantContains: "3 dot-separated segments",
		},
		{
			name: "empty segment is refused",
			submit: `  submit:
    call: billing..open
`,
			wantErr:      true,
			wantContains: "empty segment",
		},
		{
			name: "no call at all is fine (ordinary entity form)",
			submit: `  submit:
    redirect: "/orders"
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loader := manifest.NewLoader("")
			raws, perrs := loader.ParseBytes([]byte(form(tc.submit)), "checkin.yaml")
			if len(perrs) > 0 {
				t.Fatalf("parse: %v", perrs[0])
			}
			r := NewRegistry()
			if errs := r.Load(raws); len(errs) > 0 {
				t.Fatalf("load: %v", errs)
			}

			errs := r.Validate(testResolver())

			var matched string
			for _, e := range errs {
				if strings.Contains(e.Error(), "submit.call") {
					matched = e.Error()
				}
			}
			if !tc.wantErr {
				if matched != "" {
					t.Fatalf("expected no submit.call error, got: %s", matched)
				}
				return
			}
			if matched == "" {
				t.Fatalf("expected a submit.call error, got: %v", errs)
			}
			if !strings.Contains(matched, tc.wantContains) {
				t.Errorf("error should explain %q, got: %s", tc.wantContains, matched)
			}
		})
	}
}
