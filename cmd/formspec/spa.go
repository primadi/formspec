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
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// spaDownloadBaseURL — base URL download artifact spa. Bisa di-override via
// env FORMSPEC_SPA_URL (proxy/enterprise scenario), stretch dari plan §Risks.
const spaDownloadBaseURL = "https://github.com/primadi/formspec/releases/download"

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
		base = spaDownloadBaseURL
	}
	base = strings.TrimRight(base, "/")
	artifactURL := fmt.Sprintf("%s/%s/spa-%s.tar.gz", base, version, version)
	sumsURL := fmt.Sprintf("%s/%s/SHA256SUMS.txt", base, version)

	fmt.Printf("⬇️  Download %s\n", artifactURL)
	artifact, err := spaHTTPGet(artifactURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		fmt.Fprintf(os.Stderr, "   (versi %s mungkin belum punya spa artifact —\n", version)
		fmt.Fprintf(os.Stderr, "    cek https://github.com/primadi/formspec/releases/tag/%s)\n", version)
		os.Exit(1)
	}

	fmt.Printf("⬇️  Download %s\n", sumsURL)
	sums, err := spaHTTPGet(sumsURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	// Verify checksum terhadap SHA256SUMS.txt (baris spa-<v>.tar.gz).
	want := spaChecksumFromSums(string(sums), fmt.Sprintf("spa-%s.tar.gz", version))
	if want == "" {
		fmt.Fprint(os.Stderr, "❌ SHA256SUMS.txt tidak berisi entri untuk spa artifact — download dibatalkan\n")
		os.Exit(1)
	}
	got := sha256.Sum256(artifact)
	if hex.EncodeToString(got[:]) != want {
		fmt.Fprintf(os.Stderr, "❌ Checksum mismatch — download dibatalkan\n   want %s\n   got  %s\n", want, hex.EncodeToString(got[:]))
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
	if err := spaExtract(artifact, dir); err != nil {
		fmt.Fprintf(os.Stderr, "❌ extract: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ SPA %s ter-install di %s\n", version, dir)
	fmt.Printf("   → formspec dev akan otomatis memakai cache ini\n")
}

// spaChecksumFromSums extracts the hex digest for name from a
// "  <hex>  <name>" SHA256SUMS line. "" when absent.
func spaChecksumFromSums(sums, name string) string {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && filepath.Base(fields[1]) == name {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

// spaHTTPGet fetches a URL, returning the body. Mapping status jadi pesan
// yang jelas (404 → release/artifact tidak ada).
func spaHTTPGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download gagal: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 404:
		return nil, fmt.Errorf("artifact tidak ditemukan (404): %s", url)
	case resp.StatusCode != 200:
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// spaExtract extracts a spa-<version>.tar.gz (layout: spa/index.html,
// spa/assets/*) stripping the top-level "spa/" prefix into dest, with a
// path-traversal guard.
func spaExtract(data []byte, dest string) error {
	gz, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// Strip prefix "spa/" — hasil: <dest>/index.html, <dest>/manifest.json.
		name := strings.TrimPrefix(hdr.Name, "spa/")
		if name == "" || name == "." {
			continue
		}
		clean := filepath.Clean(name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("path tidak valid di tar: %s", name)
		}
		target := filepath.Join(dest, clean)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
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
