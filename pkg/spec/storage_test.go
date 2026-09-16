package spec

import (
	"strings"
	"testing"
)

// allowed_types (gap #4b). The canonical form is a bare extension, and it was
// the one form that matched nothing: `allowed_types: [jpg]` — exactly what the
// docs show — fell through every branch of the matcher and rejected legitimate
// uploads on both the client and the server. These tests pin the four accepted
// spellings so the documented one cannot silently stop working again.

func TestValidateStorageSpec_AcceptedForms(t *testing.T) {
	cases := []struct {
		name  string
		types []string
	}{
		{"canonical bare extension", []string{"jpg", "png", "pdf"}},
		{"dotted extension", []string{".jpg", ".png"}},
		{"exact mime", []string{"image/jpeg", "application/pdf"}},
		{"mime wildcard", []string{"image/*"}},
		{"mixed", []string{"jpg", ".png", "image/webp", "image/*"}},
		{"empty list", nil},
	}
	for _, c := range cases {
		s := &StorageSpec{AllowedTypes: c.types}
		if err := ValidateStorageSpec("photo", s); err != nil {
			t.Errorf("%s: expected no error, got %v", c.name, err)
		}
	}
}

func TestValidateStorageSpec_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		types   []string
		wantSub string
	}{
		{"empty entry", []string{"jpg", ""}, "is empty"},
		{"uppercase", []string{"JPG"}, "must be lowercase"},
		{"comma separated list", []string{"jpg, png"}, "not a known form"},
		{"glob-ish", []string{"*.jpg"}, "not a known form"},
		{"prose", []string{"images only"}, "not a known form"},
	}
	for _, c := range cases {
		s := &StorageSpec{AllowedTypes: c.types}
		err := ValidateStorageSpec("photo", s)
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("%s: error %q does not contain %q", c.name, err.Error(), c.wantSub)
		}
	}
}

// A file field carrying a bad allowed_types entry must fail entity validation —
// otherwise the mistake surfaces only when a user picks a file.
func TestValidateEntitySpec_StorageAllowedTypes(t *testing.T) {
	bad := &EntitySpec{
		Version: "v1",
		Fields: []Field{{
			Name: "photo", Type: FieldFile,
			Storage: &StorageSpec{AllowedTypes: []string{"JPEG photos"}},
		}},
	}
	if err := ValidateEntitySpec(bad); err == nil {
		t.Fatal("expected the entity to be rejected for a malformed allowed_types entry")
	}

	ok := &EntitySpec{
		Version: "v1",
		Fields: []Field{{
			Name: "photo", Type: FieldFile,
			Storage: &StorageSpec{AllowedTypes: []string{"jpg", "png"}, MaxSizeMB: 2, MaxCount: 1},
		}},
	}
	if err := ValidateEntitySpec(ok); err != nil {
		t.Errorf("canonical allowed_types must be accepted, got %v", err)
	}
}

func TestStorageAllowsImage(t *testing.T) {
	cases := []struct {
		types []string
		want  bool
	}{
		{[]string{"jpg", "png"}, true},
		{[]string{"image/jpeg"}, true},
		{[]string{"image/*"}, true},
		{[]string{".webp"}, true},
		{[]string{"pdf", "docx"}, false},
		{nil, false},
	}
	for _, c := range cases {
		if got := StorageAllowsImage(&StorageSpec{AllowedTypes: c.types}); got != c.want {
			t.Errorf("StorageAllowsImage(%v) = %v, want %v", c.types, got, c.want)
		}
	}
}
