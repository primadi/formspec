// ─── `formspec spa` — download & cache embedded-UI artifact ───
//
// Binary hasil `go install` (build tanpa -tags formspec_spa) tidak memuat
// embedded SPA. `formspec spa install` menutup gap itu: download artifact
// spa-<version>.tar.gz dari GitHub Releases VERSI YANG SAMA dengan binary
// (bukan "latest" — SPA harus cocok dengan kontrak bundle/meta CLI), verify
// terhadap SHA256SUMS.txt dari release yang sama, lalu extract ke cache
// ~/.formspec/spa/<version>/.
//
// Eksplisit, bukan silent-magic: download hanya terjadi saat user menjalankan
// command ini (security by default). `formspec dev` membaca cache sebagai
// fallback sebelum embedded FS — lihat spaCacheDir di dev.go.
//
// Plan: docs_internal/plan/spa-install-command.md §Fase 2.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runSpa(args []string) {
	if len(args) < 1 {
		spaUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "install":
		runSpaInstall(args[1:])
	case "path":
		runSpaPath()
	case "remove":
		runSpaRemove(args[1:])
	case "-h", "--help", "help":
		spaUsage()
	default:
		fmt.Fprintf(os.Stderr, "❌ Subcommand spa tidak dikenal: %s\n\n", args[0])
		spaUsage()
		os.Exit(1)
	}
}

func spaUsage() {
	fmt.Fprint(os.Stderr, `Usage: formspec spa <subcommand>

Subcommands:
  install          Download & verify SPA artifact versi binary ini ke cache
  path             Print path cache SPA versi ini (untuk --web-dir)
  remove [--all]   Hapus cache SPA versi ini / semua versi
`)
}

// spaCacheDirFor returns the cache directory for this binary's version,
// WITHOUT checking existence.
func spaCacheDirFor() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".formspec", "spa", version)
}

// spaCacheDir returns the installed SPA cache directory for this binary's
// version, or "" when not installed (index.html missing).
func spaCacheDir() string {
	dir := spaCacheDirFor()
	if dir == "" {
		return ""
	}
	if st, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !st.IsDir() {
		return dir
	}
	return ""
}

// spaCacheInstalled melaporkan apakah cache SPA untuk versi tertentu sudah
// terpasang (dipakai `formspec upgrade` untuk mengingatkan `spa install` ulang
// setelah versi binary berubah).
func spaCacheInstalled(ver string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	st, err := os.Stat(filepath.Join(home, ".formspec", "spa", ver, "index.html"))
	return err == nil && !st.IsDir()
}

func runSpaInstall(args []string) {
	force := false
	for _, a := range args {
		if a == "--force" {
			force = true
		} else {
			fmt.Fprintf(os.Stderr, "❌ Flag tidak dikenal: %s\n", a)
			os.Exit(1)
		}
	}

	if version == "dev" {
		fmt.Fprint(os.Stderr, `❌ Binary ini adalah build "dev" (go run / go build tanpa ldflags).

Build dev tidak punya versi rilis untuk di-match. Untuk UI saat development:
  - jalankan dari repo checkout — auto-detect renderers/react-shadcn/dist, atau
  - make build-formspec (embed SPA), atau
  - formspec dev --dev-ui (Vite HMR)
`)
		os.Exit(1)
	}

	dir := spaCacheDirFor()
	if dir == "" {
		fmt.Fprintln(os.Stderr, "❌ Tidak bisa resolve home directory")
		os.Exit(1)
	}

	if !force {
		if st, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !st.IsDir() {
			fmt.Printf("✅ SPA %s sudah ter-install di %s\n", version, dir)
			fmt.Println("   (gunakan --force untuk download ulang)")
			return
		}
	}

	base := os.Getenv("FORMSPEC_SPA_URL")
	if base == "" {
		base = releaseBaseURL()
	}
	base = strings.TrimRight(base, "/")
	artifactName := fmt.Sprintf("spa-%s.tar.gz", version)
	artifactURL := fmt.Sprintf("%s/%s/%s", base, version, artifactName)
	sumsURL := fmt.Sprintf("%s/%s/SHA256SUMS.txt", base, version)

	fmt.Printf("⬇️  Download %s\n", artifactURL)
	artifact, err := releaseGet(artifactURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		fmt.Fprintf(os.Stderr, "   (versi %s mungkin belum punya spa artifact —\n", version)
		fmt.Fprintf(os.Stderr, "    cek https://github.com/primadi/formspec/releases/tag/%s)\n", version)
		os.Exit(1)
	}

	fmt.Printf("⬇️  Download %s\n", sumsURL)
	sums, err := releaseGet(sumsURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	// Verify checksum terhadap SHA256SUMS.txt (baris spa-<v>.tar.gz).
	if err := verifyArtifact(artifact, string(sums), artifactName); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
	fmt.Println("🔒 Checksum SHA256 OK")

	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintf(os.Stderr, "❌ bersihkan cache lama: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
	if err := extractTarGz(artifact, dir, "spa/"); err != nil {
		fmt.Fprintf(os.Stderr, "❌ extract: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ SPA %s ter-install di %s\n", version, dir)
	fmt.Printf("   → formspec dev akan otomatis memakai cache ini\n")
}

func runSpaPath() {
	dir := spaCacheDirFor()
	if dir == "" {
		fmt.Fprintln(os.Stderr, "❌ Tidak bisa resolve home directory")
		os.Exit(1)
	}
	fmt.Println(dir)
	if spaCacheDir() == "" {
		fmt.Fprintln(os.Stderr, "   (belum ter-install — jalankan: formspec spa install)")
	}
}

func runSpaRemove(args []string) {
	all := false
	for _, a := range args {
		if a == "--all" {
			all = true
		} else {
			fmt.Fprintf(os.Stderr, "❌ Flag tidak dikenal: %s\n", a)
			os.Exit(1)
		}
	}
	spaBase := spaCacheBaseDir()
	if spaBase == "" {
		fmt.Fprintln(os.Stderr, "❌ Tidak bisa resolve home directory")
		os.Exit(1)
	}
	target := filepath.Join(spaBase, version)
	label := version
	if all {
		target = spaBase
		label = "semua versi"
	}
	if st, err := os.Stat(target); err != nil || !st.IsDir() {
		fmt.Printf("✅ Cache %s tidak ada — tidak ada yang perlu dihapus\n", label)
		return
	}
	if err := os.RemoveAll(target); err != nil {
		fmt.Fprintf(os.Stderr, "❌ remove: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Cache SPA %s dihapus: %s\n", label, target)
}

// spaCacheBaseDir returns ~/.formspec/spa (the multi-version cache root).
func spaCacheBaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".formspec", "spa")
}
