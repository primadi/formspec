// Package web embeds the built renderer SPA (renderers/react-shadcn/dist)
// so formspec-registry can serve the admin panel and portal UI without
// --web-dir — single-file deployment, mirroring the spec embed in
// registry/embed.go.
//
// The dist tree is synced from renderers/react-shadcn/dist by
// `make build-registry` (same pattern as build-formspec → cmd/formspec/dist).
// After changing frontend code run:
//
//	make web-build && make build-registry
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFiles embed.FS

// DistFS returns the embedded SPA rooted at the dist directory
// (index.html at the FS root), ready for formspec.Config.WebFS.
func DistFS() fs.FS {
	sub, err := fs.Sub(distFiles, "dist")
	if err != nil {
		// Unreachable: "dist" is a compile-time directory in distFiles.
		panic("registry/web: embedded dist missing: " + err.Error())
	}
	return sub
}
