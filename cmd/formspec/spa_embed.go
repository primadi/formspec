//go:build formspec_spa

// spa_embed.go — embedded SPA untuk build "lengkap" (make build / make release).
//
// cmd/formspec/dist/ TIDAK di-commit (gitignore) — diisi oleh build-spa
// (salinan renderers/react-shadcn/dist) saat build. Tanpa tag ini, file
// spa_stub.go memakai placeholder (lihat di sana) sehingga `go install` dan
// `go build` tetap bisa compile tanpa Node/npm.
package main

import "embed"

//go:embed dist/favicon.svg dist/icons.svg dist/index.html dist/assets/*
var spaFS embed.FS

// Root directory di dalam spaFS (dipakai fs.Sub di dev.go).
const spaEmbedRoot = "dist"

// spaEmbedded true = binary berisi SPA sungguhan.
const spaEmbedded = true
