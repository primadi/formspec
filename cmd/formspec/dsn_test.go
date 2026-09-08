package main

import (
	"path/filepath"
	"testing"
)

// Test resolveDSN — plan dsn-spec-anchored.md.
//
// Konvensi project layout: spec dir tinggal di <root>/spec, sehingga project
// root = parent dari spec dir. Semua path db relative harus di-anchor ke
// sana; absolute & postgres tidak diubah.
func TestResolveDSN(t *testing.T) {
	// Project root tiruan: <tmp>/proj/spec
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "proj", "spec")
	root := filepath.Join(tmp, "proj")

	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "relative sqlite path di-anchor ke project root spec",
			dsn:  "sqlite:.formspec/clinic.db",
			want: "sqlite:" + filepath.Join(root, ".formspec", "clinic.db"),
		},
		{
			name: "relative tanpa scheme diperlakukan sebagai sqlite",
			dsn:  ".formspec/data.db",
			want: "sqlite:" + filepath.Join(root, ".formspec", "data.db"),
		},
		{
			name: "absolute tetap apa adanya",
			dsn:  "sqlite:///tmp/clinic.db",
			want: "sqlite:/tmp/clinic.db",
		},
		{
			name: "postgres tidak diubah",
			dsn:  "postgres://user:pass@localhost:5432/formspec?sslmode=require",
			want: "postgres://user:pass@localhost:5432/formspec?sslmode=require",
		},
		{
			name: "path kosong memakai default engine",
			dsn:  "sqlite:",
			want: "sqlite:" + filepath.Join(root, ".formspec", "data.db"),
		},
		{
			name: "query param dipertahankan",
			dsn:  "sqlite:.formspec/clinic.db?_pragma=journal_mode(WAL)",
			want: "sqlite:" + filepath.Join(root, ".formspec", "clinic.db") + "?_pragma=journal_mode(WAL)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDSN(tt.dsn, specPath)
			if got != tt.want {
				t.Errorf("resolveDSN(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

// resolveDSN harus idempotent — DSN yang sudah ter-anchor tidak berubah
// (penting karena dev me-resolve sebelum DSN diteruskan ke engine).
func TestResolveDSNIdempotent(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "proj", "spec")
	once := resolveDSN("sqlite:.formspec/clinic.db", specPath)
	twice := resolveDSN(once, specPath)
	if once != twice {
		t.Errorf("resolveDSN not idempotent: %q → %q", once, twice)
	}
}

// Spec path yang tidak dinamai "spec" tetap ter-anchor deterministik ke
// parent-nya (proyek kecil dengan manifest langsung di folder).
func TestResolveDSNSpecNotNamedSpec(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "myproject") // manifest langsung di sini
	got := resolveDSN("sqlite:.formspec/data.db", specPath)
	want := "sqlite:" + filepath.Join(tmp, ".formspec", "data.db")
	if got != want {
		t.Errorf("resolveDSN = %q, want %q", got, want)
	}
}
