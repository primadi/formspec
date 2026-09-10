//go:build !formspec_spa

// spa_stub.go — default build TANPA tag formspec_spa (go install, go test,
// go run dari clone bersih). Menyediakan spaFS placeholder agar go:embed
// tidak gagal bila cmd/formspec/dist belum di-build. Lihat spa_embed.go.
package main

import "embed"

//go:embed spa_stub/*
var spaFS embed.FS

// Root directory di dalam spaFS (dipakai fs.Sub di dev.go).
const spaEmbedRoot = "spa_stub"

// spaEmbedded false = binary TIDAK berisi SPA sungguhan — dev.go menampilkan
// peringatan dan placeholder page memandu user.
const spaEmbedded = false
