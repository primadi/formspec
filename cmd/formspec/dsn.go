package main

import (
	"path/filepath"
	"strings"

	formspec "github.com/primadi/formspec/resource"
)

// DSN relatif di-anchor ke lokasi spec (plan dsn-spec-anchored.md).
//
// Masalah: ParseDSN adalah pure parser — path SQLite relative diteruskan
// apa adanya, sehingga SQLite menafsirkannya relatif terhadap CWD proses.
// Jalankan `formspec dev` dari root repo vs dari folder example → dua file
// database berbeda.
//
// Aturan resolusi (keputusan D 2026-09-06):
//   - DSN absolute → dipakai apa adanya.
//   - DSN relative (sqlite / tanpa scheme) → di-anchor ke project root yang
//     di-derive dari lokasi spec (formspec.ProjectRootOf, konvensi
//     08-project-layout.md: spec tinggal di <root>/spec) → file db statis
//     di <project-root>/.formspec/… di mana pun perintah dijalankan.
//   - DSN non-SQLite (postgres) → tidak diubah.
//
// Helper ini idempotent: DSN yang sudah absolute kembali tanpa perubahan.

// resolveDSN mengembalikan DSN dengan path SQLite relative yang sudah
// di-anchor ke project root dari specPath.
func resolveDSN(dsn, specPath string) string {
	// Non-SQLite → untouched.
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return dsn
	}

	rest := dsn
	if after, ok := strings.CutPrefix(rest, "sqlite:"); ok {
		rest = after
	}

	// Pisahkan query param (mis. ?_pragma=journal_mode(WAL)).
	path := rest
	query := ""
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path, query = path[:idx], path[idx:]
	}

	// Path kosong → default engine (ParseDSN: ".formspec/data.db").
	if path == "" {
		path = ".formspec/data.db"
	}

	if filepath.IsAbs(path) {
		// Normalisasi (mis. bentuk 3-slash "sqlite:///tmp/x.db" → "/tmp/x.db")
		// agar DSN yang dihasilkan kanonik dan idempotent.
		return "sqlite:" + filepath.Clean(path) + query
	}

	root := formspec.ProjectRootOf(specPath)
	return "sqlite:" + filepath.Join(root, path) + query
}
