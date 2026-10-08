package api

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

// compressFixtureTree materializes the shared SPA fixture in a temp dir and runs
// the build-time brotli pass over it, returning the dir.
func compressFixtureTree(t *testing.T) (string, BrotliStats) {
	t.Helper()
	dir := writeTree(t, spaFixture())
	stats, err := CompressStaticTree(dir, DefaultBrotliOptions())
	if err != nil {
		t.Fatalf("CompressStaticTree: %v", err)
	}
	return dir, stats
}

func sidecarExists(t *testing.T, dir, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name+brotliSidecar)))
	return err == nil
}

// ─── build-time pass ───

func TestBrotliCompressTree_OnlyEligibleFiles(t *testing.T) {
	dir, stats := compressFixtureTree(t)

	if stats.Written == 0 {
		t.Fatal("no sidecars written — the fixture has compressible assets above the floor")
	}
	if stats.RawBytes == 0 || stats.BrotliByte == 0 {
		t.Errorf("stats not populated: %+v", stats)
	}
	if stats.BrotliByte >= stats.RawBytes {
		t.Errorf("compressed %d >= raw %d — the shrink gate did not apply", stats.BrotliByte, stats.RawBytes)
	}
	if stats.Elapsed <= 0 {
		t.Error("Elapsed not populated")
	}

	// Compressible and above the floor → sidecar. Note the floor applies to root
	// files too: the fixture's index.html is 30 bytes, so it is deliberately
	// absent from this list (a real build's shell is a few KB and does get one).
	for _, name := range []string{
		"assets/index-DR5hGT_C.js",
		"assets/vendor-react-b3kFLWLo.js",
		"assets/app.js",
	} {
		if !sidecarExists(t, dir, name) {
			t.Errorf("%s: expected a .br sidecar", name)
		}
	}

	// Below the floor → no sidecar: the codec header would cost more than it saves.
	for _, name := range []string{"assets/small-AB12cd34.js", "index.html"} {
		if sidecarExists(t, dir, name) {
			t.Errorf("%s got a sidecar; the size floor should exclude it", name)
		}
	}
	// Dense type → never compressed, however large.
	if sidecarExists(t, dir, "assets/geist-latin-BgDaEnEv.woff2") {
		t.Error("woff2 got a sidecar; dense types must be excluded")
	}
	// Incompressible content → no sidecar. Note `.bin` is excluded by EXTENSION
	// before the shrink gate is ever consulted; the gate itself is pinned by
	// TestBrotliCompressTree_RefusesNonShrinkingFile, which uses a `.js` name.
	if sidecarExists(t, dir, "assets/dense-BB12cd34.bin") {
		t.Error("dense .bin got a sidecar; non-compressible types must be excluded")
	}

	// Every sidecar must belong to a source that exists.
	_ = fs.WalkDir(os.DirFS(dir), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, brotliSidecar) {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(strings.TrimSuffix(p, brotliSidecar)))); statErr != nil {
			t.Errorf("orphan sidecar %s", p)
		}
		return nil
	})
}

// A sidecar whose source stopped qualifying would be SERVED (the request path
// only checks existence and size), so a stale one is a correctness problem.
func TestBrotliCompressTree_RemovesStaleSidecars(t *testing.T) {
	dir := writeTree(t, map[string][]byte{
		"assets/big-AB12cd34.js":    compressedFixture(),
		"assets/tiny-CD34ef56.js":   []byte("x"),
		"assets/img-EF56gh78.woff2": []byte("\x00\x01woff2"),
	})
	if _, err := CompressStaticTree(dir, DefaultBrotliOptions()); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if !sidecarExists(t, dir, "assets/big-AB12cd34.js") {
		t.Fatal("setup: expected a sidecar for the big file")
	}

	// Plant sidecars for sources that do NOT qualify, as a stale build would.
	for _, name := range []string{"assets/tiny-CD34ef56.js", "assets/img-EF56gh78.woff2"} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name+brotliSidecar)), []byte("stale"), 0o644); err != nil {
			t.Fatalf("plant %s: %v", name, err)
		}
	}

	stats, err := CompressStaticTree(dir, DefaultBrotliOptions())
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if stats.Stale != 2 {
		t.Errorf("Stale = %d, want 2", stats.Stale)
	}
	if sidecarExists(t, dir, "assets/tiny-CD34ef56.js") {
		t.Error("stale sidecar for a below-floor source survived")
	}
	if sidecarExists(t, dir, "assets/img-EF56gh78.woff2") {
		t.Error("stale sidecar for a dense type survived")
	}
	if !sidecarExists(t, dir, "assets/big-AB12cd34.js") {
		t.Error("the legitimate sidecar was removed")
	}
}

func TestBrotliCompressTree_Idempotent(t *testing.T) {
	dir := writeTree(t, map[string][]byte{"assets/a-AB12cd34.js": compressedFixture()})

	first, err := CompressStaticTree(dir, DefaultBrotliOptions())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "assets", "a-AB12cd34.js"+brotliSidecar))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}

	second, err := CompressStaticTree(dir, DefaultBrotliOptions())
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "assets", "a-AB12cd34.js"+brotliSidecar))
	if err != nil {
		t.Fatalf("re-read sidecar: %v", err)
	}

	if first.Written != second.Written {
		t.Errorf("Written differs across runs: %d vs %d", first.Written, second.Written)
	}
	if !bytes.Equal(before, after) {
		t.Error("second pass produced different bytes — the pass is not idempotent")
	}
	if second.Stale != 0 {
		t.Errorf("second pass removed %d sidecars; it should remove none", second.Stale)
	}
}

// A compressible EXTENSION says nothing about the CONTENT. The fixture's dense
// `.bin` is excluded by extension alone, so it never reaches the shrink gate —
// this case puts random bytes under a `.js` name, which is the situation the gate
// exists for.
func TestBrotliCompressTree_RefusesNonShrinkingFile(t *testing.T) {
	dir := writeTree(t, map[string][]byte{
		"assets/random-AB12cd34.js": denseFixture(4 * minCompressSize),
		"assets/normal-AB12cd34.js": compressedFixture(),
	})

	stats, err := CompressStaticTree(dir, DefaultBrotliOptions())
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if sidecarExists(t, dir, "assets/random-AB12cd34.js") {
		t.Error("sidecar written for incompressible content — the shrink gate did not apply")
	}
	if !sidecarExists(t, dir, "assets/normal-AB12cd34.js") {
		t.Error("the compressible file lost its sidecar")
	}
	if stats.Written != 1 {
		t.Errorf("Written = %d, want 1", stats.Written)
	}
	if stats.Skipped == 0 {
		t.Error("Skipped not reported for the refused file")
	}
}

func TestCompressEligible(t *testing.T) {
	cases := []struct {
		name string
		size int64
		want bool
	}{
		{"assets/a.js", minCompressSize, true},
		{"assets/a.js", minCompressSize - 1, false},
		{"assets/a.woff2", 1 << 20, false},
		{"assets/a.png", 1 << 20, false},
		{"assets/a.js", maxPrecompressFile + 1, false},
		{"index.html", 4096, true},
	}
	for _, tc := range cases {
		if got := compressEligible(tc.name, tc.size); got != tc.want {
			t.Errorf("compressEligible(%q, %d) = %v, want %v", tc.name, tc.size, got, tc.want)
		}
	}
}

// ─── serving ───

// A build-time sidecar needs no CPU and beats gzip, so it wins when the client
// offers it.
func TestSpaAssets_BrotliPreferredOverGzip(t *testing.T) {
	dir, _ := compressFixtureTree(t)
	s := newSPAAssets(os.DirFS(dir), spaSourceDir)
	h := testMux(s)

	br, brBody := request(t, h, http.MethodGet, "/assets/index-DR5hGT_C.js",
		http.Header{"Accept-Encoding": {"gzip, br"}})
	if got := br.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding %q, want br (a sidecar exists and the client accepts it)", got)
	}
	if !bytes.Equal(brBody, compressedFixture()) {
		t.Error("brotli body did not decode to the source file")
	}
	if got, want := br.Header().Get("Content-Length"), br.Body.Len(); got != strconv.Itoa(want) {
		t.Errorf("Content-Length %q, want %d", got, want)
	}
	if v := br.Header().Get("Vary"); !strings.Contains(v, "Accept-Encoding") {
		t.Errorf("Vary %q, want Accept-Encoding", v)
	}

	// The same file, gzip-only client: the runtime gzip path still answers, so
	// nothing regresses for clients without brotli.
	gz, gzBody := request(t, h, http.MethodGet, "/assets/index-DR5hGT_C.js",
		http.Header{"Accept-Encoding": {"gzip"}})
	if got := gz.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding %q, want gzip for a gzip-only client", got)
	}
	if !bytes.Equal(gzBody, compressedFixture()) {
		t.Error("gzip body did not decode to the source file")
	}
	if br.Body.Len() >= gz.Body.Len() {
		t.Errorf("brotli (%d) should be smaller than gzip (%d) for this fixture", br.Body.Len(), gz.Body.Len())
	}
}

// Each representation must carry its own validator, so a cache keyed on the ETag
// can never be handed the wrong body.
func TestSpaAssets_BrotliETagIsDistinct(t *testing.T) {
	dir, _ := compressFixtureTree(t)
	s := newSPAAssets(os.DirFS(dir), spaSourceDir)
	h := testMux(s)

	// A non-fingerprinted file, so an ETag is actually emitted.
	br, _ := request(t, h, http.MethodGet, "/assets/app.js", http.Header{"Accept-Encoding": {"br"}})
	gz, _ := request(t, h, http.MethodGet, "/assets/app.js", http.Header{"Accept-Encoding": {"gzip"}})
	plain, _ := request(t, h, http.MethodGet, "/assets/app.js", nil)

	tags := map[string]string{"br": br.Header().Get("ETag"), "gzip": gz.Header().Get("ETag"), "identity": plain.Header().Get("ETag")}
	for name, tag := range tags {
		if tag == "" {
			t.Fatalf("%s: no ETag", name)
		}
	}
	if tags["br"] == tags["gzip"] || tags["br"] == tags["identity"] || tags["gzip"] == tags["identity"] {
		t.Errorf("ETags are not distinct per representation: %+v", tags)
	}
	if !strings.Contains(tags["br"], "-br") {
		t.Errorf("br ETag %q lacks the -br suffix", tags["br"])
	}

	// And revalidating with the br validator must 304 without a body.
	again, againBody := request(t, h, http.MethodGet, "/assets/app.js",
		http.Header{"Accept-Encoding": {"br"}, "If-None-Match": {tags["br"]}})
	if again.Code != http.StatusNotModified {
		t.Errorf("conditional br request: %d, want 304", again.Code)
	}
	if len(againBody) != 0 {
		t.Errorf("304 carried %d bytes", len(againBody))
	}
}

// A range request is answered identity: the compressed stream is not a byte
// range of the file.
func TestSpaAssets_BrotliSkippedForRange(t *testing.T) {
	dir, _ := compressFixtureTree(t)
	h := testMux(newSPAAssets(os.DirFS(dir), spaSourceDir))

	rec, body := request(t, h, http.MethodGet, "/assets/index-DR5hGT_C.js",
		http.Header{"Accept-Encoding": {"br"}, "Range": {"bytes=0-9"}})
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status %d, want 206", rec.Code)
	}
	if enc := rec.Header().Get("Content-Encoding"); enc != "" {
		t.Errorf("range response Content-Encoding %q, want identity", enc)
	}
	if len(body) != 10 {
		t.Errorf("range body %d bytes, want 10", len(body))
	}
}

// Sidecars exist to be selected by Accept-Encoding, not fetched: serving
// `app.js.br` would hand out a brotli stream with an octet-stream type.
func TestSpaAssets_SidecarIsNotServable(t *testing.T) {
	dir := writeTree(t, map[string][]byte{"assets/app.js": compressedFixture()})
	if _, err := CompressStaticTree(dir, DefaultBrotliOptions()); err != nil {
		t.Fatalf("compress: %v", err)
	}
	s := newSPAAssets(os.DirFS(dir), spaSourceDir)

	if rec, _ := request(t, testMux(s), http.MethodGet, "/assets/app.js.br", nil); rec.Code != http.StatusNotFound {
		t.Errorf("asset handler served a sidecar: %d, want 404", rec.Code)
	}
	if rec, _ := request(t, testMux(s), http.MethodGet, "/app/assets/app.js.br", nil); rec.Code != http.StatusNotFound {
		t.Errorf("shell handler served a sidecar: %d, want 404", rec.Code)
	}
}

// Nothing in the fixture is compressed, so a brotli-only client must fall back
// to gzip rather than receiving an uncompressed body it could have avoided.
func TestSpaAssets_BrotliAbsentFallsBackToGzip(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		rec, body := request(t, testMux(s), http.MethodGet, "/assets/index-DR5hGT_C.js",
			http.Header{"Accept-Encoding": {"br, gzip"}})
		if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("Content-Encoding %q, want gzip when no sidecar exists", got)
		}
		if !bytes.Equal(body, compressedFixture()) {
			t.Error("body mismatch")
		}
	})
}

// A brotli-capable client talking to a tree with no sidecars and no gzip match
// must still receive the bytes.
func TestSpaAssets_BrotliOnlyClientGetsGzipOrIdentity(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		rec, body := request(t, testMux(s), http.MethodGet, "/assets/index-DR5hGT_C.js",
			http.Header{"Accept-Encoding": {"br"}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want 200", rec.Code)
		}
		if !bytes.Equal(body, compressedFixture()) {
			t.Error("client did not receive the file bytes")
		}
		// No sidecar in this fixture and no gzip accepted → identity.
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("Content-Encoding %q, want identity", enc)
		}
	})
}

// The sidecar body must survive a rebuild of the sidecar itself: the memo key
// carries the sidecar's own size and mtime.
func TestSpaAssets_BrotliBodyRefreshedAfterResidecar(t *testing.T) {
	dir := writeTree(t, map[string][]byte{"assets/app.js": compressedFixture()})
	s := newSPAAssets(os.DirFS(dir), spaSourceDir)
	h := testMux(s)

	if _, err := CompressStaticTree(dir, DefaultBrotliOptions()); err != nil {
		t.Fatalf("compress: %v", err)
	}
	// Warm the memo with the first sidecar.
	if _, body := request(t, h, http.MethodGet, "/assets/app.js", http.Header{"Accept-Encoding": {"br"}}); !bytes.Equal(body, compressedFixture()) {
		t.Fatal("first brotli response did not match the source")
	}

	// Replace the SOURCE and re-compress, as `npm run build` would.
	updated := []byte("/* v2 */\n" + jsPadding + "export const v2 = 1;\n")
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), updated, 0o644); err != nil {
		t.Fatalf("rewrite source: %v", err)
	}
	if _, err := CompressStaticTree(dir, DefaultBrotliOptions()); err != nil {
		t.Fatalf("re-compress: %v", err)
	}

	rec, body := request(t, h, http.MethodGet, "/assets/app.js", http.Header{"Accept-Encoding": {"br"}})
	if rec.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("Content-Encoding %q, want br", rec.Header().Get("Content-Encoding"))
	}
	if !bytes.Equal(body, updated) {
		t.Error("served post-rebuild content as stale bytes")
	}
}

// Round-trip the encoder through the decoder the SERVER would use, so a
// malformed sidecar cannot be written silently.
func TestBrotliSidecarRoundTrip(t *testing.T) {
	dir := writeTree(t, map[string][]byte{"assets/a-AB12cd34.js": compressedFixture()})
	if _, err := CompressStaticTree(dir, DefaultBrotliOptions()); err != nil {
		t.Fatalf("compress: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "assets", "a-AB12cd34.js"+brotliSidecar))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	got, err := readAllBrotli(data)
	if err != nil {
		t.Fatalf("decode sidecar: %v", err)
	}
	if !bytes.Equal(got, compressedFixture()) {
		t.Error("sidecar did not round-trip to the source")
	}
}

func readAllBrotli(data []byte) ([]byte, error) {
	return io.ReadAll(brotli.NewReader(bytes.NewReader(data)))
}

// The sidecar must inherit the source's permission bits. os.CreateTemp creates
// 0600, which would leave a dist/ that serves for the building user and 403s for
// anyone else reading the tree.
func TestBrotliCompressTree_SidecarInheritsSourceMode(t *testing.T) {
	dir := writeTree(t, map[string][]byte{"assets/a-AB12cd34.js": compressedFixture()})
	src := filepath.Join(dir, "assets", "a-AB12cd34.js")
	if err := os.Chmod(src, 0o640); err != nil {
		t.Fatalf("chmod source: %v", err)
	}
	if _, err := CompressStaticTree(dir, DefaultBrotliOptions()); err != nil {
		t.Fatalf("compress: %v", err)
	}
	info, err := os.Stat(src + brotliSidecar)
	if err != nil {
		t.Fatalf("stat sidecar: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Errorf("sidecar mode %o, want 640 (inherited from the source)", got)
	}
}
