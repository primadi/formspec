package api

import "testing"

// AllowedFileType — the server half of the `allowed_types` contract (gap #4b).
// The documented canonical form is a bare extension (`[jpg]`), and that was the
// one form the matcher did not understand: uploads of perfectly allowed files
// were rejected with "File type not allowed". The client matcher
// (`lib/media.ts`) mirrors this table, so a change here needs the same change
// there.
func TestAllowedFileType(t *testing.T) {
	cases := []struct {
		name        string
		allowed     []string
		contentType string
		filename    string
		want        bool
	}{
		{"canonical bare extension matches", []string{"jpg", "png"}, "image/jpeg", "foto.JPG", true},
		{"bare extension does not match another ext", []string{"jpg"}, "application/pdf", "menu.pdf", false},
		{"dotted extension still works", []string{".jpg"}, "image/jpeg", "foto.jpg", true},
		{"exact mime still works", []string{"image/jpeg"}, "image/jpeg", "foto.bin", true},
		{"mime wildcard still works", []string{"image/*"}, "image/webp", "foto.webp", true},
		{"unlisted type is rejected", []string{"jpg"}, "image/gif", "anim.gif", false},
		{"case-insensitive entry", []string{"JPG"}, "image/jpeg", "foto.jpg", true},
		{"no extension at all", []string{"jpg"}, "image/jpeg", "foto", false},
	}
	for _, c := range cases {
		got := AllowedFileType(c.allowed, c.contentType, c.filename)
		if got != c.want {
			t.Errorf("%s: AllowedFileType(%v, %q, %q) = %v, want %v",
				c.name, c.allowed, c.contentType, c.filename, got, c.want)
		}
	}
}
