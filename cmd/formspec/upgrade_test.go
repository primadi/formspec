package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── helpers ─────────────────────────────────────────────────────────────────

// fakeBinary membuat script shell yang melaporkan versi — cukup untuk smoke
// test di Unix.
func fakeBinary(t *testing.T, tag string) []byte {
	t.Helper()
	return []byte("#!/bin/sh\necho \"formspec " + tag + "\"\n")
}

// releaseServer menyajikan artifact + SHA256SUMS (checksum atas arsip) dan
// endpoint releases/latest untuk satu tag.
func releaseServer(t *testing.T, tag, asset string, member string, binary []byte) *httptest.Server {
	t.Helper()

	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: member, Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	payload := archive.Bytes()

	sum := sha256.Sum256(payload)
	sums := hex.EncodeToString(sum[:]) + "  " + asset + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			_, _ = fmt.Fprintf(w, `{"tag_name":%q}`, tag)
		case "/" + tag + "/" + asset:
			_, _ = w.Write(payload)
		case "/" + tag + "/SHA256SUMS.txt":
			_, _ = fmt.Fprint(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ── guard: dev build ────────────────────────────────────────────────────────

func TestUpgradeRunRejectsDevBuild(t *testing.T) {
	old := version
	version = "dev"
	defer func() { version = old }()

	err := upgradeRun("", false, false, false, true)
	if err == nil {
		t.Fatal("upgradeRun pada build dev: want error, got nil")
	}
	if !strings.Contains(err.Error(), "dev") {
		t.Errorf("error tidak menyebut dev: %v", err)
	}
}

// ── --check / up-to-date / dry-run ──────────────────────────────────────────

func TestUpgradeRunCheckOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"tag_name":"v9.9.9"}`)
	}))
	// httptest.Server.Close returns no error — nothing to ignore here.
	defer srv.Close()
	t.Setenv("FORMSPEC_RELEASE_API", srv.URL)

	old := version
	version = "v0.0.6"
	defer func() { version = old }()

	if err := upgradeRun("", true, false, false, true); err != nil {
		t.Fatalf("upgradeRun --check: %v", err)
	}
}

func TestUpgradeRunAlreadyLatest(t *testing.T) {
	old := version
	version = "v0.0.7"
	defer func() { version = old }()

	if err := upgradeRun("v0.0.7", false, false, false, true); err != nil {
		t.Fatalf("upgradeRun already latest: %v", err)
	}
}

func TestUpgradeRunDryRunDoesNotTouchBinary(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, binaryName(runtime.GOOS))
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldResolve, oldVersion := resolveUpgradeExePath, version
	resolveUpgradeExePath = func() (string, error) { return exe, nil }
	version = "v0.0.6"
	defer func() { resolveUpgradeExePath, version = oldResolve, oldVersion }()

	if err := upgradeRun("v0.0.7", false, true, false, true); err != nil {
		t.Fatalf("upgradeRun --dry-run: %v", err)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != "OLD" {
		t.Errorf("dry-run mengubah binary: %q", got)
	}
}

// ── downloadVerifiedBinary ──────────────────────────────────────────────────

func TestDownloadVerifiedBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture shell script hanya untuk Unix")
	}
	asset, err := releaseAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	bin := fakeBinary(t, "v9.9.9")
	srv := releaseServer(t, "v9.9.9", asset, binaryName(runtime.GOOS), bin)

	dir := t.TempDir()
	path, err := downloadVerifiedBinary(srv.URL, "v9.9.9", asset, dir)
	if err != nil {
		t.Fatalf("downloadVerifiedBinary: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(bin) {
		t.Errorf("isi binary baru salah: %q", got)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o111 == 0 {
		t.Errorf("binary baru tidak executable: %v", st.Mode().Perm())
	}
}

func TestDownloadVerifiedBinaryChecksumMismatch(t *testing.T) {
	asset, err := releaseAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "SHA256SUMS.txt") {
			_, _ = fmt.Fprintf(w, "deadbeef  %s\n", asset)
			return
		}
		_, _ = fmt.Fprint(w, "garbage-archive")
	}))
	// httptest.Server.Close returns no error — nothing to ignore here.
	defer srv.Close()

	dir := t.TempDir()
	if _, err := downloadVerifiedBinary(srv.URL, "v9.9.9", asset, dir); err == nil {
		t.Fatal("checksum mismatch: want error, got nil")
	}
	if _, err := os.Stat(upgradeTempPath(dir)); err == nil {
		t.Error("temp file dibuat walau checksum mismatch")
	}
}

// ── swapBinary ──────────────────────────────────────────────────────────────

func TestSwapBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uji jalur Unix")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "formspec")
	neu := filepath.Join(dir, ".formspec-new")
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(neu, []byte("NEW"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := swapBinary(exe, neu); err != nil {
		t.Fatalf("swapBinary: %v", err)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != "NEW" {
		t.Errorf("setelah swap = %q, want NEW", got)
	}
	if _, err := os.Stat(neu); err == nil {
		t.Error("file temp masih ada setelah swap")
	}
}

func TestSwapBinaryWindowsRenameOld(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "formspec.exe")
	neu := filepath.Join(dir, "formspec-new.exe")
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(neu, []byte("NEW"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := swapBinaryWindows(exe, neu); err != nil {
		t.Fatalf("swapBinaryWindows: %v", err)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != "NEW" {
		t.Errorf("setelah swap = %q, want NEW", got)
	}
	if _, err := os.Stat(exe + ".old"); err == nil {
		t.Error(".old tidak tersapu")
	}
}

func TestCleanupUpgradeOld(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "formspec.exe")
	if err := os.WriteFile(exe+".old", []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	cleanupUpgradeOld(exe)
	if _, err := os.Stat(exe + ".old"); err == nil {
		t.Error(".old masih ada setelah cleanup")
	}
}

// ── smokeTestVersion ────────────────────────────────────────────────────────

func TestSmokeTestVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture shell script hanya untuk Unix")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good")
	if err := os.WriteFile(good, fakeBinary(t, "v9.9.9"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := smokeTestVersion(good, "v9.9.9"); err != nil {
		t.Errorf("smokeTestVersion cocok: %v", err)
	}
	if err := smokeTestVersion(good, "v1.0.0"); err == nil {
		t.Error("smokeTestVersion tag beda: want error, got nil")
	}

	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := smokeTestVersion(bad, "v9.9.9"); err == nil {
		t.Error("smokeTestVersion binary gagal-jalan: want error, got nil")
	}
}

// ── ensureWritableDir ───────────────────────────────────────────────────────

func TestEnsureWritableDirRejectsReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root mengabaikan permission bit")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o755) }()

	err := ensureWritableDir(dir)
	if err == nil {
		t.Fatal("direktori read-only: want error, got nil")
	}
	if !strings.Contains(err.Error(), "install.sh") {
		t.Errorf("error tidak menyertakan fallback installer: %v", err)
	}
}

func TestEnsureWritableDirOK(t *testing.T) {
	if err := ensureWritableDir(t.TempDir()); err != nil {
		t.Fatalf("ensureWritableDir pada temp dir: %v", err)
	}
}

// ── spaCacheInstalled (hint setelah upgrade) ────────────────────────────────

func TestSpaCacheInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if spaCacheInstalled("v0.0.9") {
		t.Fatal("cache belum ada, harus false")
	}
	dir := filepath.Join(home, ".formspec", "spa", "v0.0.9")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !spaCacheInstalled("v0.0.9") {
		t.Error("cache dengan index.html harus terdeteksi true")
	}
}

// ── E2E: upgrade penuh terhadap binary palsu di temp dir ────────────────────

func TestUpgradeRunEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture shell script hanya untuk Unix")
	}
	asset, err := releaseAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	srv := releaseServer(t, "v9.9.9", asset, binaryName(runtime.GOOS), fakeBinary(t, "v9.9.9"))
	t.Setenv("FORMSPEC_RELEASE_BASE", srv.URL)
	t.Setenv("FORMSPEC_RELEASE_API", srv.URL)

	dir := t.TempDir()
	exe := filepath.Join(dir, binaryName(runtime.GOOS))
	if err := os.WriteFile(exe, fakeBinary(t, "v0.0.6"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldResolve, oldVersion := resolveUpgradeExePath, version
	resolveUpgradeExePath = func() (string, error) { return exe, nil }
	version = "v0.0.6"
	defer func() { resolveUpgradeExePath, version = oldResolve, oldVersion }()

	if err := upgradeRun("", false, false, false, true); err != nil {
		t.Fatalf("upgradeRun E2E: %v", err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "v9.9.9") {
		t.Errorf("binary tidak ter-upgrade, isi: %q", got)
	}
	if _, err := os.Stat(upgradeTempPath(dir)); err == nil {
		t.Error("file temp tertinggal setelah upgrade sukses")
	}
}
