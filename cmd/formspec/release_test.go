package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ── releaseAssetName ────────────────────────────────────────────────────────

func TestReleaseAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
		wantErr      bool
	}{
		{"linux", "amd64", "formspec-linux-amd64.tar.gz", false},
		{"linux", "arm64", "formspec-linux-arm64.tar.gz", false},
		{"darwin", "amd64", "formspec-darwin-amd64.tar.gz", false},
		{"darwin", "arm64", "formspec-darwin-arm64.tar.gz", false},
		{"windows", "amd64", "formspec-windows-amd64.zip", false},
		{"windows", "arm64", "formspec-windows-arm64.zip", false},
		{"plan9", "amd64", "", true},
		{"linux", "386", "", true},
	}
	for _, c := range cases {
		got, err := releaseAssetName(c.goos, c.goarch)
		if c.wantErr {
			if err == nil {
				t.Errorf("releaseAssetName(%s,%s) = %q, want error", c.goos, c.goarch, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("releaseAssetName(%s,%s) error: %v", c.goos, c.goarch, err)
			continue
		}
		if got != c.want {
			t.Errorf("releaseAssetName(%s,%s) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

// ── checksumFromSums / verifyArtifact ───────────────────────────────────────

func TestChecksumFromSums(t *testing.T) {
	sums := "aa11  formspec-linux-amd64.tar.gz\nbb22  spa-v1.0.0.tar.gz\n\n"
	if got := checksumFromSums(sums, "formspec-linux-amd64.tar.gz"); got != "aa11" {
		t.Errorf("checksumFromSums = %q, want aa11", got)
	}
	if got := checksumFromSums(sums, "nope.tar.gz"); got != "" {
		t.Errorf("checksumFromSums absent = %q, want empty", got)
	}
	// Base-name matching: entri dengan direktori tetap cocok.
	if got := checksumFromSums("cc33  ./dist/formspec-linux-amd64.tar.gz\n", "formspec-linux-amd64.tar.gz"); got != "cc33" {
		t.Errorf("checksumFromSums base-name = %q, want cc33", got)
	}
}

func TestVerifyArtifact(t *testing.T) {
	data := []byte("hello release")
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	name := "formspec-linux-amd64.tar.gz"

	if err := verifyArtifact(data, hexSum+"  "+name+"\n", name); err != nil {
		t.Errorf("verifyArtifact valid: %v", err)
	}
	if err := verifyArtifact(data, "deadbeef  "+name+"\n", name); err == nil {
		t.Error("verifyArtifact mismatch: want error, got nil")
	}
	if err := verifyArtifact(data, "deadbeef  other.tar.gz\n", name); err == nil {
		t.Error("verifyArtifact absent entry: want error, got nil")
	}
}

// ── latestTagFrom ───────────────────────────────────────────────────────────

func TestLatestTagFrom(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, `{"tag_name":"v9.9.9","name":"release"}`)
	}))
	// httptest.Server.Close returns no error — nothing to ignore here.
	defer srv.Close()

	got, err := latestTagFrom(srv.URL + "/repos/o/r")
	if err != nil {
		t.Fatalf("latestTagFrom: %v", err)
	}
	if got != "v9.9.9" {
		t.Errorf("latestTagFrom = %q, want v9.9.9", got)
	}

	if _, err := latestTagFrom(srv.URL + "/missing"); err == nil {
		t.Error("latestTagFrom 404: want error, got nil")
	}
}

// ── extractTarGz ────────────────────────────────────────────────────────────

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractTarGzStripsPrefix(t *testing.T) {
	data := tarGz(t, map[string]string{
		"spa/index.html":    "<html></html>",
		"spa/assets/app.js": "console.log(1)",
		"spa/manifest.json": `{"version":"v1"}`,
	})
	dest := t.TempDir()
	if err := extractTarGz(data, dest, "spa/"); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "index.html"))
	if err != nil {
		t.Fatalf("read extracted index.html: %v", err)
	}
	if string(got) != "<html></html>" {
		t.Errorf("index.html = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "assets", "app.js")); err != nil {
		t.Errorf("assets/app.js absen: %v", err)
	}
}

func TestExtractTarGzRejectsTraversal(t *testing.T) {
	data := tarGz(t, map[string]string{"../evil.txt": "pwn"})
	dest := t.TempDir()
	if err := extractTarGz(data, dest, ""); err == nil {
		t.Error("extractTarGz traversal: want error, got nil")
	}
}

// ── extractArchiveMember ────────────────────────────────────────────────────

func TestExtractArchiveMemberTarGz(t *testing.T) {
	data := tarGz(t, map[string]string{"formspec": "BINARY-CONTENT"})
	dest := filepath.Join(t.TempDir(), "out", "formspec.new")
	if err := extractArchiveMember(data, "formspec-linux-amd64.tar.gz", "formspec", dest); err != nil {
		t.Fatalf("extractArchiveMember: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "BINARY-CONTENT" {
		t.Errorf("content = %q", got)
	}
	st, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o111 == 0 {
		t.Errorf("mode %v tidak executable", st.Mode().Perm())
	}
}

func TestExtractArchiveMemberZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("formspec.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("WIN-BINARY")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "formspec.exe.new")
	if err := extractArchiveMember(buf.Bytes(), "formspec-windows-amd64.zip", "formspec.exe", dest); err != nil {
		t.Fatalf("extractArchiveMember zip: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "WIN-BINARY" {
		t.Errorf("content = %q", got)
	}
}

func TestExtractArchiveMemberMissing(t *testing.T) {
	data := tarGz(t, map[string]string{"readme.txt": "no binary here"})
	dest := filepath.Join(t.TempDir(), "formspec.new")
	if err := extractArchiveMember(data, "x.tar.gz", "formspec", dest); err == nil {
		t.Error("extractArchiveMember missing member: want error, got nil")
	}
}
