//go:build !formspec_spa

// Package web — default build TANPA tag formspec_spa (go install, go test,
// go build dari clone bersih). Menyediakan placeholder agar go:embed tidak
// gagal bila web/dist belum di-build. Lihat embed.go untuk dokumentasi.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:stub
var distFiles embed.FS

// embeddedRoot is the directory inside distFiles to strip via fs.Sub.
const embeddedRoot = "stub"

// Embedded false = binary TIDAK berisi SPA sungguhan.
const Embedded = false

// DistFS returns the embedded placeholder rooted at its directory,
// ready for formspec.Config.WebFS.
func DistFS() fs.FS {
	sub, err := fs.Sub(distFiles, embeddedRoot)
	if err != nil {
		// Unreachable: embeddedRoot is a compile-time directory in distFiles.
		panic("formspec-registry/web: embedded stub missing: " + err.Error())
	}
	return sub
}
