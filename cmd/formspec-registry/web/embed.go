//go:build formspec_spa

// Package web embeds the built renderer SPA (renderers/react-shadcn/dist)
// so formspec-registry can serve the admin panel and portal UI without
// --web-dir — single-file deployment, mirroring the spec embed in
// app-spec/embed.go.
//
// The dist tree is synced from renderers/react-shadcn/dist by
// `make build-registry` (same pattern as build-formspec → cmd/formspec/dist).
// After changing frontend code run:
//
//	make web-build && make build-registry
//
// web/dist TIDAK di-commit (gitignore). Tanpa tag formspec_spa file
// embed_stub.go memakai placeholder — go install / go build tetap compile.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFiles embed.FS

// embeddedRoot is the directory inside distFiles to strip via fs.Sub.
const embeddedRoot = "dist"

// Embedded true = binary berisi SPA sungguhan.
const Embedded = true

// DistFS returns the embedded SPA rooted at the dist directory
// (index.html at the FS root), ready for formspec.Config.WebFS.
func DistFS() fs.FS {
	sub, err := fs.Sub(distFiles, embeddedRoot)
	if err != nil {
		// Unreachable: embeddedRoot is a compile-time directory in distFiles.
		panic("formspec-registry/web: embedded dist missing: " + err.Error())
	}
	return sub
}
