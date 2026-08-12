package manifest

import (
	"strings"
	"testing"
)

func TestDiscover(t *testing.T) {
	// Integration test: discover will be tested with example specs
	l := NewLoader(".")
	files, err := l.Discover()
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	// Should find at least no files in empty dir
	t.Logf("Found %d manifest files", len(files))
}

func TestParseFile(t *testing.T) {
	l := NewLoader(".")
	// Test with a YAML that doesn't exist
	_, errs := l.parseFile("nonexistent.yaml")
	if len(errs) == 0 {
		t.Error("Expected errors for nonexistent file")
	}
}

func TestValidate(t *testing.T) {
	l := NewLoader(".")

	tests := []struct {
		name    string
		raw     RawManifest
		wantErr bool
	}{
		{
			name: "valid manifest",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Kind:       "Entity",
				Metadata:   RawMetadata{Name: "test"},
			},
			wantErr: false,
		},
		{
			name: "missing apiVersion",
			raw: RawManifest{
				Kind:     "Entity",
				Metadata: RawMetadata{Name: "test"},
			},
			wantErr: true,
		},
		{
			name: "missing kind",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Metadata:   RawMetadata{Name: "test"},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Kind:       "Entity",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := l.Validate(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_EntityExtensionValidation(t *testing.T) {
	l := NewLoader(".")

	tests := []struct {
		name    string
		raw     RawManifest
		wantErr bool
		errMsg  string // substring to check in error
	}{
		{
			name: "base entity without extension",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Kind:       "Entity",
				Metadata:   RawMetadata{Name: "customer"},
				Spec: map[string]any{
					"version": "v1",
					"fields": []any{
						map[string]any{"name": "name", "type": "string", "required": true},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "extension with required field rejected",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Kind:       "Entity",
				Metadata:   RawMetadata{Name: "invoice-ext"},
				Spec: map[string]any{
					"version": "v1",
					"extend_storage": map[string]any{
						"target":    "billing/invoice",
						"namespace": "custext",
					},
					"fields": []any{
						map[string]any{"name": "project_code", "type": "string", "required": true},
					},
				},
			},
			wantErr: true,
			errMsg:  "cannot be required",
		},
		{
			name: "extension without required accepted",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Kind:       "Entity",
				Metadata:   RawMetadata{Name: "invoice-ext"},
				Spec: map[string]any{
					"version": "v1",
					"extend_storage": map[string]any{
						"target":    "billing/invoice",
						"namespace": "custext",
					},
					"fields": []any{
						map[string]any{"name": "project_code", "type": "string"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "extension missing target",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Kind:       "Entity",
				Metadata:   RawMetadata{Name: "invoice-ext"},
				Spec: map[string]any{
					"version": "v1",
					"extend_storage": map[string]any{
						"namespace": "custext",
					},
					"fields": []any{
						map[string]any{"name": "code", "type": "string"},
					},
				},
			},
			wantErr: true,
			errMsg:  "target",
		},
		{
			name: "extension invalid target format",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Kind:       "Entity",
				Metadata:   RawMetadata{Name: "invoice-ext"},
				Spec: map[string]any{
					"version": "v1",
					"extend_storage": map[string]any{
						"target":    "billing",
						"namespace": "custext",
					},
					"fields": []any{
						map[string]any{"name": "code", "type": "string"},
					},
				},
			},
			wantErr: true,
			errMsg:  "target",
		},
		{
			name: "extension missing namespace",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Kind:       "Entity",
				Metadata:   RawMetadata{Name: "invoice-ext"},
				Spec: map[string]any{
					"version": "v1",
					"extend_storage": map[string]any{
						"target": "billing/invoice",
					},
					"fields": []any{
						map[string]any{"name": "code", "type": "string"},
					},
				},
			},
			wantErr: true,
			errMsg:  "namespace",
		},
		{
			name: "non-entity kind skips entity validation",
			raw: RawManifest{
				APIVersion: "formspec.dev/v1alpha1",
				Kind:       "Module",
				Metadata:   RawMetadata{Name: "billing"},
				Spec:       map[string]any{"version": "v1"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := l.Validate(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}
