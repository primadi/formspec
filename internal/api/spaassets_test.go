package api

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/go-chi/chi/v5"
)

// ─── fixtures ───

// jsPadding makes a body comfortably above minCompressSize while staying compressible.
var jsPadding = strings.Repeat("const value = 1234567;\n", 200)

func compressedFixture() []byte {
	return []byte("/* bundle */\n" + jsPadding + "export default value;\n")
}

// denseFixture is random data: incompressible, so gzip cannot shrink it.
func denseFixture(size int) []byte {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return buf
}

// spaFixture is the same tree for both the embed and the dir subtests, so a
// behaviour difference between them is real and not a fixture difference.
func spaFixture() map[string][]byte {
	return map[string][]byte{
		"index.html":                        []byte("<html><body>spa</body></html>"),
		"assets/index-DR5hGT_C.js":          compressedFixture(),
		"assets/vendor-react-b3kFLWLo.js":   compressedFixture(),
		"assets/app.js":                     compressedFixture(), // not fingerprinted
		"assets/geist-latin-BgDaEnEv.woff2": []byte("\x00\x01\x00\x00woff2binary"),
		"assets/small-AB12cd34.js":          []byte("x := 1\n"), // < minCompressSize
		"assets/dense-BB12cd34.bin":         denseFixture(2 * minCompressSize),
		"favicon.svg":                       []byte("<svg><rect/></svg>"),
		"icons.svg":                         []byte("<svg><circle/></svg>"),
		"manifest.json":                     []byte(`{"name":"x"}`),
	}
}

func mapFSFixture() fs.FS {
	m := fstest.MapFS{}
	for name, data := range spaFixture() {
		m[name] = &fstest.MapFile{Data: data, ModTime: time.Unix(1700000000, 0)}
	}
	return m
}

func dirFSFixture(t *testing.T) fs.FS {
	t.Helper()
	return os.DirFS(writeTree(t, spaFixture()))
}

// writeTree materializes a fixture map under a fresh temp dir and returns it.
func writeTree(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// testMux mirrors how BuildHTTP mounts the tree: a workspace-prefixed shell
// mount plus the root-level asset routes. The handlers read chi's wildcard, so
// they can only be exercised through a real router.
func testMux(s *spaAssets) http.Handler {
	r := chi.NewRouter()
	assets := s.AssetHandler()
	r.Get("/assets/*", assets)
	r.Get("/favicon.svg", assets)
	r.Get("/icons.svg", assets)
	r.Get("/manifest.json", assets)

	shell := s.ShellHandler()
	r.Get("/app", shell)
	r.Get("/app/*", shell)
	r.Get("/", shell)
	r.Get("/*", shell)
	return r
}

// eachSource runs fn against both delivery paths so every assertion below
// doubles as an embed/dir parity check.
func eachSource(t *testing.T, fn func(t *testing.T, name string, s *spaAssets)) {
	t.Helper()
	t.Run("embed", func(t *testing.T) {
		fn(t, "embed", newSPAAssets(mapFSFixture(), spaSourceEmbed))
	})
	t.Run("dir", func(t *testing.T) {
		fn(t, "dir", newSPAAssets(dirFSFixture(t), spaSourceDir))
	})
}

// request drives the mux and returns the recorder plus the decoded body.
func request(t *testing.T, h http.Handler, method, target string, header http.Header) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.Bytes()
	switch rec.Header().Get("Content-Encoding") {
	case "gzip":
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("%s: gzip decode: %v", target, err)
		}
		decoded, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("%s: gzip read: %v", target, err)
		}
		body = decoded
	case "br":
		decoded, err := io.ReadAll(brotli.NewReader(bytes.NewReader(body)))
		if err != nil {
			t.Fatalf("%s: brotli read: %v", target, err)
		}
		body = decoded
	}
	return rec, body
}

// ─── cache headers ───

func TestSpaAssets_CacheControl(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		h := testMux(s)
		cases := []struct {
			target string
			want   string
		}{
			{"/assets/index-DR5hGT_C.js", immutableCacheControl},
			{"/assets/vendor-react-b3kFLWLo.js", immutableCacheControl},
			{"/assets/geist-latin-BgDaEnEv.woff2", immutableCacheControl},
			// Not fingerprinted → conservative, even though the name ends with a
			// hash-like run.
			{"/assets/app.js", revalidateCacheControl},
			{"/favicon.svg", "public, max-age=604800"},
			{"/icons.svg", "public, max-age=604800"},
			{"/manifest.json", "public, max-age=3600"},
		}
		for _, tc := range cases {
			rec, _ := request(t, h, http.MethodGet, tc.target, nil)
			if rec.Code != http.StatusOK {
				t.Errorf("%s: status %d, want 200", tc.target, rec.Code)
				continue
			}
			if got := rec.Header().Get("Cache-Control"); got != tc.want {
				t.Errorf("%s: Cache-Control %q, want %q", tc.target, got, tc.want)
			}
			if rec.Header().Get("Content-Length") == "" {
				t.Errorf("%s: Content-Length missing", tc.target)
			}
		}
	})
}

func TestSpaAssets_FingerprintPredicate(t *testing.T) {
	cases := map[string]bool{
		"assets/index-DR5hGT_C.js":          true,
		"assets/vendor-icons-XIbW6kZD.js":   true,
		"assets/geist-latin-BgDaEnEv.woff2": true,
		// The base name carries the hash, so nesting under assets/ does not
		// matter — only the root prefix does.
		"assets/nested/deep-BB12cd34.js": true,
		"assets/app.js":                  false, // no hash
		"assets/index.js":                false,
		"manifest.json":                  false, // root file, not under assets/
		"favicon.svg":                    false,
		"index.html":                     false,
		// The prefix, not the pattern alone, decides: these names LOOK hashed
		// (eight chars before the extension) but live outside assets/, where a
		// file is allowed to keep a stable name while its content changes.
		// Marking them immutable would serve stale content for a year.
		"report-20240101.js":       false,
		"static/index-AB12cd34.js": false,
		"favicon-AB12cd34.svg":     false,
		"chunk-20240101.css":       false,
	}
	for name, want := range cases {
		if got := isFingerprinted(name); got != want {
			t.Errorf("isFingerprinted(%q) = %v, want %v", name, got, want)
		}
	}
}

// The prefix guard must also hold at the HTTP layer: a root-level file whose
// name looks hashed must be revalidated, not marked immutable.
func TestSpaAssets_HashLikeRootFileIsNotImmutable(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		probe := newSPAAssets(fstest.MapFS{
			"report-20240101.js": {Data: compressedFixture()},
		}, s.source)
		rec, _ := request(t, testMux(probe), http.MethodGet, "/report-20240101.js", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Cache-Control"); got != revalidateCacheControl {
			t.Errorf("Cache-Control %q, want %q for a root-level hash-like name", got, revalidateCacheControl)
		}
		if rec.Header().Get("ETag") == "" {
			t.Error("revalidated file has no ETag")
		}
	})
}

// The renderer's real build output is the ground truth for the predicate. If a
// Vite config change stops content-hashing asset names, `immutable` would become
// a correctness bug — this catches that. dist/ is gitignored, so a missing tree
// skips rather than fails (recorded as a limitation in the plan).
func TestSpaAssets_FingerprintMatchesRealBuild(t *testing.T) {
	dist := filepath.Join("..", "..", "renderers", "react-shadcn", "dist", "assets")
	entries, err := os.ReadDir(dist)
	if err != nil {
		t.Skipf("built SPA not present (%v) — run `make web-build` to enable this guard", err)
	}
	var checked int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if ext := filepath.Ext(name); ext != ".js" && ext != ".css" && ext != ".woff2" {
			continue
		}
		checked++
		if !isFingerprinted("assets/" + name) {
			t.Errorf("build artifact %q is not fingerprinted — immutable caching would serve stale content", name)
		}
	}
	if checked == 0 {
		t.Fatal("no build artifacts inspected")
	}
	t.Logf("verified %d build artifact(s) against the fingerprint predicate", checked)
}

// ─── conditional requests ───

func TestSpaAssets_ETagRevalidation(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		h := testMux(s)
		// An asset (validated per file) and the shell (validated per index.html).
		for _, target := range []string{"/assets/app.js", "/app/orders/42"} {
			rec, body := request(t, h, http.MethodGet, target, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status %d, want 200", target, rec.Code)
			}
			etag := rec.Header().Get("ETag")
			if etag == "" {
				t.Fatalf("%s: no ETag", target)
			}
			if len(body) == 0 {
				t.Fatalf("%s: empty body", target)
			}

			again, againBody := request(t, h, http.MethodGet, target, http.Header{"If-None-Match": {etag}})
			if again.Code != http.StatusNotModified {
				t.Errorf("%s: status %d, want 304", target, again.Code)
			}
			if len(againBody) != 0 {
				t.Errorf("%s: 304 carried %d body bytes", target, len(againBody))
			}
		}
	})
}

// A fingerprinted asset is answered straight from the immutable directive, so no
// ETag is emitted — and no hashing cost is paid either.
func TestSpaAssets_FingerprintedAssetHasNoETag(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		rec, _ := request(t, testMux(s), http.MethodGet, "/assets/index-DR5hGT_C.js", nil)
		if got := rec.Header().Get("ETag"); got != "" {
			t.Errorf("fingerprinted asset emitted ETag %q, want none", got)
		}
	})
}

// Rebuilding a file in the watched directory must invalidate its validator, or
// the browser would keep a 304 forever after `npm run build`.
func TestSpaAssets_DirRebuildInvalidatesETag(t *testing.T) {
	dir := writeTree(t, map[string][]byte{"assets/app.js": compressedFixture()})
	h := testMux(newSPAAssets(os.DirFS(dir), spaSourceDir))

	first, firstBody := request(t, h, http.MethodGet, "/assets/app.js", nil)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on first response")
	}

	// Replace the file with different content — `npm run build` in miniature.
	replacement := []byte("/* rebuilt */\n" + jsPadding + "export const v2 = 1;\n")
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), replacement, 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	// A conditional request with the OLD validator must NOT be answered 304.
	second, secondBody := request(t, h, http.MethodGet, "/assets/app.js", http.Header{"If-None-Match": {etag}})
	if second.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 after rebuild", second.Code)
	}
	if bytes.Equal(firstBody, secondBody) {
		t.Error("served the pre-rebuild body after the file changed")
	}
	if second.Header().Get("ETag") == etag {
		t.Error("ETag did not change after the file was rewritten")
	}
}

// ─── compression ───

func TestSpaAssets_Gzip(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		h := testMux(s)
		rec, body := request(t, h, http.MethodGet, "/assets/index-DR5hGT_C.js",
			http.Header{"Accept-Encoding": {"gzip"}})
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("Content-Encoding %q, want gzip", got)
		}
		if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
			t.Errorf("Vary %q, want Accept-Encoding", got)
		}
		if !bytes.Equal(body, compressedFixture()) {
			t.Error("decompressed body differs from the source file")
		}
		// Content-Length describes the compressed representation. net/http drops
		// it whenever Content-Encoding is set, so the handler must supply it.
		if got := rec.Header().Get("Content-Length"); got != fmt.Sprint(rec.Body.Len()) {
			t.Errorf("Content-Length %q, want %d", got, rec.Body.Len())
		}
	})
}

// The size floor is a hard gate: one byte under stays identity, at the floor it
// compresses. Both sides are pinned so a later refactor cannot quietly turn the
// boundary into `>` or `<=`.
func TestSpaAssets_GzipMinSizeBoundary(t *testing.T) {
	cases := []struct {
		label    string
		data     []byte
		wantGzip bool
	}{
		{fmt.Sprintf("%d bytes", minCompressSize-1), []byte(strings.Repeat("a", minCompressSize-1)), false},
		{fmt.Sprintf("%d bytes", minCompressSize), []byte(strings.Repeat("b", minCompressSize)), true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			sources := []struct {
				name string
				s    *spaAssets
			}{
				{"embed", newSPAAssets(fstest.MapFS{
					"assets/probe.js": {Data: tc.data},
				}, spaSourceEmbed)},
				{"dir", newSPAAssets(os.DirFS(writeTree(t, map[string][]byte{"assets/probe.js": tc.data})), spaSourceDir)},
			}
			for _, src := range sources {
				rec, body := request(t, testMux(src.s), http.MethodGet, "/assets/probe.js",
					http.Header{"Accept-Encoding": {"gzip"}})
				enc := rec.Header().Get("Content-Encoding")
				if tc.wantGzip && enc != "gzip" {
					t.Errorf("%s: Content-Encoding %q, want gzip", src.name, enc)
				}
				if !tc.wantGzip && enc != "" {
					t.Errorf("%s: Content-Encoding %q, want identity", src.name, enc)
				}
				if !bytes.Equal(body, tc.data) {
					t.Errorf("%s: body mismatch", src.name)
				}
				// Vary is set whenever the type is compressible, even for an
				// identity response, so a shared cache never mixes representations.
				if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
					t.Errorf("%s: Vary %q, want Accept-Encoding", src.name, got)
				}
			}
		})
	}
}

// Incompressible content above the floor must be sent identity: a compressed
// response is never allowed to be bigger than the file it represents.
func TestSpaAssets_GzipRejectsNonShrinkingBody(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		data := denseFixture(4 * minCompressSize)
		// A compressible extension carrying incompressible bytes is exactly the
		// case the shrink gate exists for.
		probe := newSPAAssets(fstest.MapFS{
			"assets/dense-BB12cd34.js": {Data: data},
		}, s.source)
		rec, body := request(t, testMux(probe), http.MethodGet, "/assets/dense-BB12cd34.js",
			http.Header{"Accept-Encoding": {"gzip"}})
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("Content-Encoding %q, want identity for a non-shrinking body", enc)
		}
		if !bytes.Equal(body, data) {
			t.Errorf("body %d bytes, want %d", len(body), len(data))
		}
	})
}

func TestSpaAssets_GzipNegotiation(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		h := testMux(s)
		cases := map[string]bool{ // Accept-Encoding → expect gzip
			"gzip":              true,
			"gzip, deflate":     true,
			"identity;q=0":      false,
			"*":                 true, // wildcard covers gzip
			"gzip;q=0":          false,
			"gzip;q=0, *":       false,
			"gzip;q=0.001":      true,
			"br, zstd":          false,
			"":                  false, // no header at all
			"GZIP":              true,  // case-insensitive
			"gzip;q=0.0, br":    false,
			"deflate, gzip;q=1": true,
		}
		for accept, wantGzip := range cases {
			hdr := http.Header{}
			if accept != "" {
				hdr.Set("Accept-Encoding", accept)
			}
			rec, _ := request(t, h, http.MethodGet, "/assets/index-DR5hGT_C.js", hdr)
			if gotGzip := rec.Header().Get("Content-Encoding") == "gzip"; gotGzip != wantGzip {
				t.Errorf("Accept-Encoding %q: gzip=%v, want %v", accept, gotGzip, wantGzip)
			}
		}
	})
}

func TestSpaAssets_DenseTypesNeverCompressed(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		rec, _ := request(t, testMux(s), http.MethodGet, "/assets/geist-latin-BgDaEnEv.woff2",
			http.Header{"Accept-Encoding": {"gzip"}})
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("woff2 Content-Encoding %q, want identity", enc)
		}
	})
}

// A compressed representation must not be answered as a byte range of the file:
// ranges are served identity.
func TestSpaAssets_RangeIsIdentity(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		rec, body := request(t, testMux(s), http.MethodGet, "/assets/index-DR5hGT_C.js",
			http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-9"}})
		if rec.Code != http.StatusPartialContent {
			t.Fatalf("status %d, want 206", rec.Code)
		}
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("range response Content-Encoding %q, want identity", enc)
		}
		if len(body) != 10 {
			t.Errorf("range body %d bytes, want 10", len(body))
		}
	})
}

func TestSpaAssets_VaryNotEmptyWhenBelowFloor(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		rec, _ := request(t, testMux(s), http.MethodGet, "/assets/small-AB12cd34.js",
			http.Header{"Accept-Encoding": {"gzip"}})
		if enc := rec.Header().Get("Content-Encoding"); enc != "" {
			t.Errorf("small asset Content-Encoding %q, want identity", enc)
		}
		if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
			t.Errorf("Vary %q, want Accept-Encoding", got)
		}
	})
}

func TestSpaAssets_NonCompressibleHasNoVary(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		rec, _ := request(t, testMux(s), http.MethodGet, "/assets/geist-latin-BgDaEnEv.woff2", nil)
		if got := rec.Header().Get("Vary"); got != "" {
			t.Errorf("Vary %q, want empty for a type that is never compressed", got)
		}
	})
}

// The gzip and identity representations must not share a validator: a cache
// keyed on the ETag would otherwise hand a compressed body to a client that did
// not ask for one.
func TestSpaAssets_ETagDistinguishesEncodings(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		h := testMux(s)
		plain, _ := request(t, h, http.MethodGet, "/assets/app.js", nil)
		gz, _ := request(t, h, http.MethodGet, "/assets/app.js", http.Header{"Accept-Encoding": {"gzip"}})
		if plain.Header().Get("ETag") == gz.Header().Get("ETag") {
			t.Error("gzip and identity share an ETag")
		}
	})
}

// ─── handlers ───

func TestSpaAssets_ShellFallbackAndAssetMiss(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		h := testMux(s)

		// A real file below the mount is served as itself, not as the shell.
		rec, body := request(t, h, http.MethodGet, "/app/assets/app.js", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("deep asset: status %d, want 200", rec.Code)
		}
		if !bytes.Equal(body, compressedFixture()) {
			t.Errorf("deep asset served %q", body)
		}
		if rec.Header().Get("ETag") == "" {
			t.Error("real asset through the shell has no ETag")
		}

		// Any other path falls back to index.html so client-side routes survive a
		// hard refresh — with revalidation, never an immutable directive.
		for _, target := range []string{"/app", "/app/orders/42", "/", "/deep/nested/route"} {
			rec, body := request(t, h, http.MethodGet, target, nil)
			if rec.Code != http.StatusOK {
				t.Errorf("shell %s: status %d, want 200", target, rec.Code)
			}
			if !strings.Contains(string(body), "spa") {
				t.Errorf("shell %s: got %q, want index.html", target, body)
			}
			if got := rec.Header().Get("Cache-Control"); got != revalidateCacheControl {
				t.Errorf("shell %s: Cache-Control %q, want %q", target, got, revalidateCacheControl)
			}
		}

		// The asset handler is strict: a miss is a 404, never the SPA shell, so a
		// broken bundle surfaces as a broken script instead of HTML.
		miss, missBody := request(t, h, http.MethodGet, "/assets/does-not-exist.js", nil)
		if miss.Code != http.StatusNotFound {
			t.Errorf("asset miss: status %d, want 404", miss.Code)
		}
		if strings.Contains(string(missBody), "spa") {
			t.Error("asset miss answered with the SPA shell")
		}
	})
}

func TestSpaAssets_TraversalRejected(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		h := testMux(s)
		for _, target := range []string{"/assets/../../etc/passwd", "/assets/..%2f..%2fetc/passwd"} {
			rec, body := request(t, h, http.MethodGet, target, nil)
			if rec.Code == http.StatusOK {
				t.Fatalf("%s: served the SPA root escape (body %q)", target, body)
			}
		}
	})
}

// ─── warm (pre-compress) ───

func TestSpaAssets_WarmEligibility(t *testing.T) {
	embed := newSPAAssets(mapFSFixture(), spaSourceEmbed)
	dir := newSPAAssets(dirFSFixture(t), spaSourceDir)

	cases := map[string]struct{ embed, dir bool }{
		"assets/index-DR5hGT_C.js":          {true, true}, // fingerprinted
		"assets/app.js":                     {true, false},
		"index.html":                        {true, false},
		"favicon.svg":                       {true, false},
		"assets/geist-latin-BgDaEnEv.woff2": {false, false}, // dense type
	}
	for name, want := range cases {
		if got := embed.warmEligible(name); got != want.embed {
			t.Errorf("embed warmEligible(%q) = %v, want %v", name, got, want.embed)
		}
		if got := dir.warmEligible(name); got != want.dir {
			t.Errorf("dir warmEligible(%q) = %v, want %v", name, got, want.dir)
		}
	}
}

// Warm must actually populate the memo, and that must be observable as "no
// compression work on the first request".
func TestSpaAssets_WarmSkipsFirstRequestCompression(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		s.Warm()
		var calls atomic.Int64
		s.compressFn = func(b []byte) ([]byte, error) {
			calls.Add(1)
			return gzipCompress(b)
		}

		rec, body := request(t, testMux(s), http.MethodGet, "/assets/index-DR5hGT_C.js",
			http.Header{"Accept-Encoding": {"gzip"}})
		if rec.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("Content-Encoding %q, want gzip", rec.Header().Get("Content-Encoding"))
		}
		if !bytes.Equal(body, compressedFixture()) {
			t.Error("warm body differs from the source file")
		}
		if n := calls.Load(); n != 0 {
			t.Errorf("first request compressed %d time(s); Warm should have memoized it", n)
		}
	})
}

func TestSpaAssets_WarmSkipsSmallAndDenseFiles(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		s.Warm()
		for _, name := range []string{"assets/small-AB12cd34.js", "assets/dense-BB12cd34.bin"} {
			info, err := fs.Stat(s.fsys, name)
			if err != nil {
				t.Fatalf("stat %s: %v", name, err)
			}
			if _, ok := s.comp.get(memoKey(name, info.Size(), info.ModTime())); ok {
				t.Errorf("Warm memoized %s, which the bounds/gates exclude", name)
			}
		}
	})
}

func TestSpaAssets_WarmIdempotent(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		s.Warm()
		var calls atomic.Int64
		s.compressFn = func(b []byte) ([]byte, error) {
			calls.Add(1)
			return gzipCompress(b)
		}
		s.Warm()
		if n := calls.Load(); n != 0 {
			t.Errorf("second Warm compressed %d file(s); it must skip memoized entries", n)
		}
	})
}

func TestSpaAssets_WarmSkipsWhenOverTotalBudget(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		s.warmTotalLimit = minCompressSize // far below the fixture's total
		var calls atomic.Int64
		s.compressFn = func(b []byte) ([]byte, error) {
			calls.Add(1)
			return gzipCompress(b)
		}
		s.Warm()
		if n := calls.Load(); n != 0 {
			t.Errorf("Warm compressed %d file(s) despite exceeding the total budget", n)
		}
		if _, used := s.comp.stats(); used != 0 {
			t.Errorf("memo holds %d bytes after a skipped warm, want 0", used)
		}
	})
}

// Warm is an optimisation, never a correctness dependency: a file rewritten
// after Warm must be served with its new bytes.
func TestSpaAssets_WarmDoesNotFreezeContent(t *testing.T) {
	dir := writeTree(t, map[string][]byte{"assets/index-DR5hGT_C.js": compressedFixture()})
	s := newSPAAssets(os.DirFS(dir), spaSourceDir)
	s.Warm()

	replacement := []byte("/* rebuilt */\n" + jsPadding + "export const v2 = 1;\n")
	if err := os.WriteFile(filepath.Join(dir, "assets", "index-DR5hGT_C.js"), replacement, 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	_, body := request(t, testMux(s), http.MethodGet, "/assets/index-DR5hGT_C.js",
		http.Header{"Accept-Encoding": {"gzip"}})
	if !bytes.Equal(body, replacement) {
		t.Error("served pre-rebuild bytes: warm froze the content")
	}
}

// ─── byte-LRU ───

func TestByteLRU_EvictsWithinBudget(t *testing.T) {
	lru := newByteLRU(100)
	for i := 0; i < 50; i++ {
		lru.put(fmt.Sprintf("k%d", i), make([]byte, 20))
	}
	entries, used := lru.stats()
	if used > 100 {
		t.Errorf("used %d bytes, want <= 100", used)
	}
	if entries == 0 {
		t.Fatal("evicted everything")
	}
	if _, ok := lru.get("k49"); !ok {
		t.Error("most recent key was evicted")
	}
	if _, ok := lru.get("k0"); ok {
		t.Error("oldest key survived beyond the budget")
	}
}

func TestByteLRU_RefusesOversizedEntry(t *testing.T) {
	lru := newByteLRU(10)
	lru.put("big", make([]byte, 11))
	if _, ok := lru.get("big"); ok {
		t.Error("stored an entry larger than the whole budget")
	}
	if _, used := lru.stats(); used != 0 {
		t.Errorf("used %d bytes, want 0", used)
	}
}

// A warm candidate set larger than the memo budget must still serve correctly:
// evicted entries are simply recomputed on demand.
func TestSpaAssets_EvictedEntryStillServed(t *testing.T) {
	const each = 3 * minCompressSize
	files := fstest.MapFS{}
	for i := 0; i < 6; i++ {
		files[fmt.Sprintf("assets/chunk%02d-AB12cd34.js", i)] = &fstest.MapFile{Data: compressedFixture()}
	}
	s := newSPAAssets(files, spaSourceEmbed)
	s.comp = newByteLRU(each) // holds roughly one entry

	s.Warm()
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("/assets/chunk%02d-AB12cd34.js", i)
		rec, body := request(t, testMux(s), http.MethodGet, name, http.Header{"Accept-Encoding": {"gzip"}})
		if rec.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("%s: Content-Encoding %q, want gzip", name, rec.Header().Get("Content-Encoding"))
		}
		if !bytes.Equal(body, compressedFixture()) {
			t.Errorf("%s: body mismatch after eviction", name)
		}
	}
	if _, used := s.comp.stats(); used > each {
		t.Errorf("memo used %d bytes, want <= %d", used, each)
	}
}

// Each source must be servable concurrently without racing on the memo.
func TestSpaAssets_ConcurrentServe(t *testing.T) {
	eachSource(t, func(t *testing.T, _ string, s *spaAssets) {
		h := testMux(s)
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodGet, "/assets/index-DR5hGT_C.js", nil)
				req.Header.Set("Accept-Encoding", "gzip")
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Errorf("status %d, want 200", rec.Code)
				}
			}()
		}
		wg.Wait()
	})
}
