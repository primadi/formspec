package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spaChecksumFromSums — parse entri "  <hex>  <name>".
func TestSpaChecksumFromSums(t *testing.T) {
	sums := "  e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  formspec-linux-amd64.tar.gz\n" +
		"  abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  spa-v0.0.4.tar.gz\n"
	got := spaChecksumFromSums(sums, "spa-v0.0.4.tar.gz")
	want := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if spaChecksumFromSums(sums, "spa-v0.0.5.tar.gz") != "" {
		t.Fatal("missing entry harus mengembalikan string kosong")
	}
}

// helper: buat tar.gz dengan entry "spa/<name>" → isi.
func makeTestSpaTar(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: "spa/" + name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
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

// spaExtract: strip prefix "spa/", isi index.html + manifest.json.
func TestSpaExtract(t *testing.T) {
	data := makeTestSpaTar(t, map[string]string{
		"index.html":    "<html>ok</html>",
		"assets/a.js":   "console.log(1)",
		"manifest.json": `{"version": "v0.0.4"}`,
	})
	dest := t.TempDir()
	if err := spaExtract(data, dest); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"index.html", filepath.Join("assets", "a.js"), "manifest.json"} {
		if _, err := os.Stat(filepath.Join(dest, f)); err != nil {
			t.Fatalf("%s tidak ter-extract: %v", f, err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(dest, "index.html"))
	if string(b) != "<html>ok</html>" {
		t.Fatalf("isi index.html salah: %q", b)
	}
}

// spaExtract: path traversal di tar harus ditolak.
func TestSpaExtractRejectsTraversal(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	payload := "evil"
	if err := tw.WriteHeader(&tar.Header{Name: "spa/../../evil.txt", Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte(payload))
	tw.Close()
	gz.Close()

	dest := t.TempDir()
	if err := spaExtract(buf.Bytes(), dest); err == nil {
		t.Fatal("path traversal harus ditolak, tapi extract sukses")
	}
	// Tidak boleh ada file di luar dest.
	if _, err := os.Stat(filepath.Join(dest, "..", "formspec")); err == nil {
		t.Fatal("file keluar dari dest seharusnya tidak ada")
	}
}

// spaCacheDir: hanya return path bila index.html ada.
func TestSpaCacheDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Setelah Setenv, UserHomeDir pakai HOME env (unix) — helper membaca ulang.
	if spaCacheDir() != "" {
		t.Fatal("cache harus kosong sebelum install")
	}
	dir := spaCacheDirFor()
	if dir == "" {
		t.Fatal("spaCacheDirFor tidak boleh kosong")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if spaCacheDir() != dir {
		t.Fatalf("got %q, want %q", spaCacheDir(), dir)
	}
	if !strings.HasSuffix(spaCacheDirFor(), filepath.Join(".formspec", "spa", version)) {
		t.Fatalf("layout cache salah: %q", spaCacheDirFor())
	}
}
