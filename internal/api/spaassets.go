package api

import (
	"bytes"
	"compress/gzip"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
)

// Static SPA asset delivery: one implementation for the embedded SPA and for a
// `--web-dir` tree (docs_internal/plan/static-asset-caching.md).
//
// Before this file, the four handlers below it in router.go wrote the file
// straight to the ResponseWriter with only a Content-Type header: no
// Cache-Control, no ETag, no Content-Length, and no compression — so every page
// load re-downloaded the whole bundle, including the content-hashed chunks that
// could never change. The Vite build already fingerprints those names
// (`assets/index-DR5hGT_C.js`), which is what makes an `immutable` directive
// safe here.
const (
	// minCompressSize is the smallest body worth compressing, shared by the
	// runtime gzip path and the build-time brotli pass. Below it the ~18-byte
	// codec header/trailer and the CPU cost outweigh the saving, and Vite splits
	// out several such small chunks. Plan D4.
	minCompressSize = 1 << 10 // 1 KiB

	// Pre-compression bounds (plan D7/D8).
	warmMaxFile  = 2 << 20  // per-file ceiling: bigger assets are compressed lazily
	warmMaxTotal = 64 << 20 // total raw bytes worth warming; beyond it, skip warm

	// compressedBudget caps the memoized gzip bodies. A byte budget (not an
	// entry count) keeps the ceiling honest for a bundle dominated by a few
	// multi-hundred-kilobyte vendor chunks; LRU eviction is what lets the warm
	// candidate set be generous without the memory growing per rebuild.
	compressedBudget = 32 << 20

	// hashBudget caps the memoized SHA-256 digests (32 hex chars each).
	hashBudget = 1 << 20

	immutableCacheControl  = "public, max-age=31536000, immutable"
	revalidateCacheControl = "no-cache"
)

// spaSource is how the SPA tree is provided. It is an explicit parameter and
// never inferred from the fs.FS type: fstest.MapFS (used by the API tests) is
// semantically immutable, so inferring the source from the concrete type would
// silently change warm behaviour whenever a refactor swaps the FS (plan D9).
type spaSource int

const (
	spaSourceEmbed spaSource = iota // frozen for the process lifetime
	spaSourceDir                    // may be rewritten by `npm run build`
)

// assetHashRe matches Vite's content hash: a dash, eight [A-Za-z0-9_-] chars,
// then the extension (`index-DR5hGT_C.js`, `geist-latin-wght-normal-BgDaEnEv.woff2`).
var assetHashRe = regexp.MustCompile(`-[A-Za-z0-9_-]{8}\.[A-Za-z0-9]+$`)

// isFingerprinted reports whether name is a content-hashed asset.
//
// The `assets/` prefix is load-bearing, not decoration. Vite's convention puts
// hashed chunks there, while the dist root holds stable names (index.html,
// favicon.svg, manifest.json). Without the prefix, a hand-placed file whose name
// merely LOOKS hashed would be marked immutable for a year: `report-20240101.js`
// matches the pattern (eight [A-Za-z0-9_-] chars before the extension), and
// answering a file that can change with `immutable` is exactly the staleness bug
// this predicate exists to avoid. The cost of the restriction is only that a
// non-default Vite `assetsDir` loses immutable caching.
func isFingerprinted(name string) bool {
	if !strings.HasPrefix(name, "assets/") {
		return false
	}
	return assetHashRe.MatchString(path.Base(name))
}

// staticCacheControl returns the Cache-Control directive for a served file.
// The default is the conservative one: revalidate every time. Only a
// fingerprinted name — where a content change necessarily changes the URL —
// earns the year-long immutable directive. A wrong predicate therefore costs
// cache hits, never correctness.
func staticCacheControl(name string, fingerprinted bool) string {
	switch {
	case fingerprinted:
		return immutableCacheControl
	case name == "favicon.svg" || name == "icons.svg":
		return "public, max-age=604800"
	case name == "manifest.json":
		return "public, max-age=3600"
	default:
		return revalidateCacheControl
	}
}

// mimeTypeByExtension returns a MIME type for the web file extensions the
// renderer bundle ships. It stays a hand-rolled table (rather than
// mime.TypeByExtension) so the response headers do not depend on the host's
// /etc/mime.types.
func mimeTypeByExtension(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "application/javascript; charset=utf-8"
	case ".json", ".map":
		return "application/json"
	case ".webmanifest":
		return "application/manifest+json"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// isCompressible reports whether the extension is worth gzip. Already-dense
// formats (woff2, png, jpeg, webp) are absent on purpose: compressing them costs
// CPU and, at best, saves a couple of bytes.
func isCompressible(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm", ".css", ".js", ".mjs", ".json", ".svg", ".txt", ".map", ".webmanifest":
		return true
	default:
		return false
	}
}

// acceptsEncoding parses Accept-Encoding and reports whether `token` is
// acceptable, honouring q-values. An explicit `token;q=0` wins over a wildcard,
// per RFC 9110 §12.5.3. It serves both gzip and brotli — the grammar is
// identical, and two parsers would be two chances to get q-value handling wrong.
func acceptsEncoding(header, token string) bool {
	if header == "" {
		return false
	}
	wildcard := false
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		name := strings.ToLower(strings.TrimSpace(fields[0]))
		if name != token && name != "*" {
			continue
		}
		q := 1.0
		for _, param := range fields[1:] {
			param = strings.TrimSpace(param)
			if v, ok := strings.CutPrefix(strings.ToLower(param), "q="); ok {
				if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
					q = f
				}
			}
		}
		if name == token {
			return q > 0
		}
		wildcard = wildcard || q > 0
	}
	return wildcard
}

// isSidecar reports whether name is a pre-compressed artifact rather than a file
// the renderer asks for. Sidecars exist to be selected by Accept-Encoding, never
// to be fetched directly: serving `app.js.br` as a download named `app.js.br`
// (with an octet-stream type) would hand out a brotli stream that nothing can
// run, and it would be fingerprinted as a cacheable asset.
func isSidecar(name string) bool {
	return strings.HasSuffix(name, brotliSidecar) || strings.HasSuffix(name, ".gz")
}

// cleanAssetPath normalizes a chi wildcard into an fs.FS-valid path relative to
// the SPA root. Anything that would escape the root (`..`, absolute, empty)
// becomes "".
func cleanAssetPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ""
	}
	p = path.Clean(p)
	if !fs.ValidPath(p) || p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}

// ─── byte-budget LRU ───

// byteLRU is a tiny LRU keyed by string with a byte budget. It backs both the
// gzip memo and the ETag digest memo.
type byteLRU struct {
	mu     sync.Mutex
	order  *list.List
	items  map[string]*list.Element
	budget int
	used   int
}

type lruItem struct {
	key   string
	value []byte
}

func newByteLRU(budget int) *byteLRU {
	return &byteLRU{order: list.New(), items: make(map[string]*list.Element), budget: budget}
}

func (c *byteLRU) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*lruItem).value, true
}

func (c *byteLRU) put(key string, value []byte) {
	if len(value) > c.budget {
		return // never evict the whole cache to hold one entry
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		item := el.Value.(*lruItem)
		c.used += len(value) - len(item.value)
		item.value = value
		c.order.MoveToFront(el)
		c.evictLocked()
		return
	}
	c.items[key] = c.order.PushFront(&lruItem{key: key, value: value})
	c.used += len(value)
	c.evictLocked()
}

func (c *byteLRU) evictLocked() {
	for c.used > c.budget {
		el := c.order.Back()
		if el == nil {
			return
		}
		item := el.Value.(*lruItem)
		c.order.Remove(el)
		delete(c.items, item.key)
		c.used -= len(item.value)
	}
}

func (c *byteLRU) stats() (entries, used int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len(), c.used
}

// ─── spaAssets ───

// spaAssets serves the built renderer SPA from an fs.FS with HTTP caching,
// conditional requests and compression (brotli, then gzip, then identity).
type spaAssets struct {
	fsys   fs.FS
	source spaSource

	comp   *byteLRU // memoized compressed bodies, keyed by name|size|mtime (+encoding)
	hashes *byteLRU // memoized hex digests of raw bodies, same key

	// brMu/brSeen memoize whether a `.br` sibling exists for a source file. The
	// common case is "no sidecar" (a --web-dir tree that was never compressed),
	// and without the negative cache every request would pay an extra Stat.
	// Keyed by the SOURCE file's (name, size, mtime): a rebuild replaces the
	// source, so the probe is redone exactly when it could have changed.
	brMu   sync.Mutex
	brSeen map[string]string // source memo key → sidecar memo key ("" = absent)

	// compressFn is injectable so tests can count/observe compression without
	// parsing gzip output.
	compressFn func([]byte) ([]byte, error)
	compressN  atomic.Int64

	// warmTotalLimit overrides warmMaxTotal when non-zero. It exists so the
	// "candidates exceed the budget" path is reachable from a test without
	// generating 64 MiB of fixtures.
	warmTotalLimit int64
}

func newSPAAssets(fsys fs.FS, source spaSource) *spaAssets {
	if fsys == nil {
		return nil
	}
	return &spaAssets{
		fsys:           fsys,
		source:         source,
		comp:           newByteLRU(compressedBudget),
		hashes:         newByteLRU(hashBudget),
		brSeen:         make(map[string]string),
		compressFn:     gzipCompress,
		warmTotalLimit: warmMaxTotal,
	}
}

func newSPAAssetsFromDir(dir string) *spaAssets {
	return newSPAAssets(os.DirFS(dir), spaSourceDir)
}

// memoKey ties a memo entry to the exact bytes it was derived from. A rebuild
// that replaces the file changes size and/or mtime, so the stale entry is simply
// never looked up again — warm can never serve bytes that no longer exist.
func memoKey(name string, size int64, mod time.Time) string {
	return name + "|" + strconv.FormatInt(size, 10) + "|" + strconv.FormatInt(mod.UnixNano(), 10)
}

func gzipCompress(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ShellHandler serves the SPA shell. A path that matches a real file is served
// as that file; anything else falls back to index.html, so a hard refresh of
// /{ws}/app/orders/42 still boots the client-side router.
func (s *spaAssets) ShellHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := cleanAssetPath(chi.URLParam(r, "*"))
		if name == "" {
			s.serve(w, r, "index.html")
			return
		}
		if info, err := fs.Stat(s.fsys, name); err != nil || info.IsDir() {
			s.serve(w, r, "index.html")
			return
		}
		s.serve(w, r, name)
	}
}

// AssetHandler serves the root-level files Vite references with absolute paths
// (/assets/*, /favicon.svg, /icons.svg, /manifest.json). A miss is a 404 — never
// the SPA shell, so a broken bundle stays loud instead of answering a script
// request with HTML.
func (s *spaAssets) AssetHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var name string
		if wildcard := chi.URLParam(r, "*"); wildcard != "" {
			name = "assets/" + wildcard
		} else {
			name = strings.TrimPrefix(r.URL.Path, "/")
		}
		name = cleanAssetPath(name)
		if name == "" {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "asset not found")
			return
		}
		if info, err := fs.Stat(s.fsys, name); err != nil || info.IsDir() {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "asset not found")
			return
		}
		s.serve(w, r, name)
	}
}

func (s *spaAssets) serve(w http.ResponseWriter, r *http.Request, name string) {
	if isSidecar(name) {
		// A pre-compressed artifact is selected via Accept-Encoding, never
		// fetched by name (see isSidecar).
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	info, err := fs.Stat(s.fsys, name)
	if err != nil || info.IsDir() {
		if name != "index.html" {
			// The file vanished between the caller's Stat and here, or a deep
			// link reached the shell: fall back to index.html, and let THAT
			// branch produce the 404 if the shell is missing too.
			s.serve(w, r, "index.html")
			return
		}
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	raw, err := fs.ReadFile(s.fsys, name)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	fingerprinted := isFingerprinted(name)
	w.Header().Set("Cache-Control", staticCacheControl(name, fingerprinted))
	w.Header().Set("Content-Type", mimeTypeByExtension(name))

	// Preference order: brotli (a build-time artifact, decoded for free) →
	// gzip (computed here or pre-warmed) → identity. Each layer is a strict
	// improvement on the one below, and every layer has a fallback, so a client
	// that accepts nothing still gets the bytes.
	body := raw
	encoding := "" // "" | "br" | "gzip"
	if isCompressible(name) {
		// Set even when the response ends up identity (below the size floor, or
		// a Range request) so a shared cache never mixes representations.
		w.Header().Set("Vary", "Accept-Encoding")
		if s.wantsCompression(r, raw) {
			// Negotiate first, then do the work: probing/loading a brotli body
			// for a client that cannot read it would be pure waste.
			accept := r.Header.Get("Accept-Encoding")
			if acceptsEncoding(accept, "br") {
				if br, ok := s.precompressed(name, info, raw); ok {
					body, encoding = br, "br"
				}
			}
			if encoding == "" && acceptsEncoding(accept, "gzip") {
				if gz, ok := s.compressed(name, info, raw); ok {
					body, encoding = gz, "gzip"
				}
			}
		}
	}
	if encoding != "" {
		w.Header().Set("Content-Encoding", encoding)
	}

	if !fingerprinted {
		// Fingerprinted names are immutable, so a validator would only add
		// hashing cost: the URL itself is the cache key.
		w.Header().Set("ETag", s.etag(name, info, raw, encoding))
	}

	if encoding != "" {
		// http.ServeContent deliberately omits Content-Length for an encoded
		// body (a stream's length is unknown to it), but here it is known
		// exactly — and a length lets the browser show progress and detect a
		// truncated transfer. Range requests are served identity, so this never
		// conflicts with the sub-range length ServeContent computes.
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}

	// ServeContent supplies the conditional-request handling (If-None-Match →
	// 304, If-Modified-Since), Range → 206, and an accurate Content-Length —
	// the pieces the previous hand-rolled write was missing.
	http.ServeContent(w, r, name, info.ModTime(), bytes.NewReader(body))
}

// wantsCompression applies every gate that does not depend on the client's
// encoding preference: small bodies and range requests are answered identity.
func (s *spaAssets) wantsCompression(r *http.Request, raw []byte) bool {
	if r.Header.Get("Range") != "" {
		// A range request is answered identity: the compressed body is not a
		// byte range of the file.
		return false
	}
	return len(raw) >= minCompressSize
}

// precompressed returns the build-time `.br` sibling of name, if one exists and
// is smaller than the source.
//
// Existence is probed once per (name, size, mtime) and memoized — including the
// negative answer, which is the common case for a tree that was never
// compressed. The body is memoized in the same LRU as gzip, under a distinct
// key, so the two encodings can never be served as each other.
func (s *spaAssets) precompressed(name string, info fs.FileInfo, raw []byte) ([]byte, bool) {
	srcKey := memoKey(name, info.Size(), info.ModTime())
	brKey, known := s.brLookup(srcKey)
	if !known {
		brKey = s.probeSidecar(name, info, len(raw))
	}
	if brKey == "" {
		return nil, false
	}
	if br, ok := s.comp.get(brKey); ok {
		return br, true
	}
	br, err := fs.ReadFile(s.fsys, name+brotliSidecar)
	if err != nil || len(br) >= len(raw) {
		// The sidecar disappeared or stopped being an improvement (a rebuild
		// landing between the probe and the read). Answer identity/gzip rather
		// than serving something larger than the file.
		return nil, false
	}
	s.comp.put(brKey, br)
	return br, true
}

// brLookup reads the sidecar probe cache. known=false means "not probed yet".
func (s *spaAssets) brLookup(srcKey string) (brKey string, known bool) {
	s.brMu.Lock()
	defer s.brMu.Unlock()
	v, ok := s.brSeen[srcKey]
	return v, ok
}

// probeSidecar stats the `.br` sibling and records the answer. The recorded key
// carries the sidecar's own size and mtime so a compressed body is never reused
// across a rebuild of the sidecar.
func (s *spaAssets) probeSidecar(name string, info fs.FileInfo, rawLen int) string {
	var key string
	if bi, err := fs.Stat(s.fsys, name+brotliSidecar); err == nil && !bi.IsDir() {
		// A sidecar that is not smaller is worse than useless; treat it as
		// absent so the check is not repeated per request.
		if bi.Size() < int64(rawLen) {
			key = memoKey(name+":br", bi.Size(), bi.ModTime())
		}
	}
	s.brMu.Lock()
	s.brSeen[memoKey(name, info.Size(), info.ModTime())] = key
	s.brMu.Unlock()
	return key
}

// compressed returns the gzip body for name, computing and memoizing it on first
// use. ok is false when compression is refused — the result did not shrink, or
// the compressor failed — and the caller then serves identity. Only bodies that
// are actually sent compressed get memoized.
func (s *spaAssets) compressed(name string, info fs.FileInfo, raw []byte) ([]byte, bool) {
	key := memoKey(name, info.Size(), info.ModTime())
	if gz, ok := s.comp.get(key); ok {
		return gz, true
	}
	s.compressN.Add(1)
	gz, err := s.compressFn(raw)
	if err != nil || len(gz) >= len(raw) {
		return nil, false
	}
	s.comp.put(key, gz)
	return gz, true
}

// etag builds a strong validator for the representation. The digest is over the
// RAW bytes and memoized per (name, size, mtime); each encoding derives from it
// with a suffix, so the compressed bodies are never hashed and the three
// representations still carry distinct ETags (RFC 9110 §8.8.3). Sharing one
// digest across encodings is safe precisely because the suffix separates them —
// a cache keyed on the ETag can never be handed the wrong body.
func (s *spaAssets) etag(name string, info fs.FileInfo, raw []byte, encoding string) string {
	key := memoKey(name, info.Size(), info.ModTime())
	digest, ok := s.hashes.get(key)
	if !ok {
		sum := sha256.Sum256(raw)
		digest = []byte(hex.EncodeToString(sum[:16]))
		s.hashes.put(key, digest)
	}
	switch encoding {
	case "br":
		return `"` + string(digest) + `-br"`
	case "gzip":
		return `"` + string(digest) + `-gzip"`
	default:
		return `"` + string(digest) + `"`
	}
}

// warmEligible decides which files are worth pre-compressing at start. An
// embedded tree is frozen for the process lifetime, so every compressible asset
// qualifies. A directory tree is rewritten by `npm run build`, so only
// content-hashed names do: warming anything else would be work that a rebuild
// invalidates, and the lazy path already covers it (plan D7).
func (s *spaAssets) warmEligible(name string) bool {
	if !isCompressible(name) {
		return false
	}
	if s.source == spaSourceEmbed {
		return true
	}
	return isFingerprinted(name)
}

// Warm pre-compresses the assets that cannot change during this process, so the
// first request for a large chunk is not also the one that pays for gzip.
//
// It is safe to call more than once and concurrently with serving: the memo key
// is (name, size, mtime), so a rebuild landing after Warm simply misses the memo
// and is compressed on demand. Warm only ever makes a future request cheaper; it
// never changes what a request is answered with.
func (s *spaAssets) Warm() {
	if s == nil || s.fsys == nil {
		return
	}
	start := time.Now()

	type candidate struct {
		name string
		info fs.FileInfo
	}
	var (
		cands  []candidate
		total  int64
		tooBig bool
	)
	_ = fs.WalkDir(s.fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry: skip, never fail the walk
		}
		if d.IsDir() || !s.warmEligible(p) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() < minCompressSize || info.Size() > warmMaxFile {
			return nil
		}
		total += info.Size()
		if total > s.warmTotalLimit {
			tooBig = true
			return fs.SkipAll
		}
		cands = append(cands, candidate{name: p, info: info})
		return nil
	})
	if tooBig {
		log.Printf("formspec: static asset pre-compress skipped: candidates exceed %d bytes (lazy gzip stays active)", s.warmTotalLimit)
		return
	}
	if len(cands) == 0 {
		return
	}

	workers := runtime.GOMAXPROCS(0)
	if workers > len(cands) {
		workers = len(cands)
	}
	if workers < 1 {
		workers = 1
	}

	var (
		warmed   atomic.Int64
		rawBytes atomic.Int64
		gzBytes  atomic.Int64
		brFiles  atomic.Int64
		brBytes  atomic.Int64
	)
	work := make(chan candidate)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				key := memoKey(c.name, c.info.Size(), c.info.ModTime())
				if _, ok := s.comp.get(key); ok {
					continue // already warm — Warm is idempotent
				}
				raw, err := fs.ReadFile(s.fsys, c.name)
				if err != nil {
					continue
				}
				// A build-time `.br` sibling needs no CPU at all — only a read. The
				// old embedded-SPA case that this file was written for
				// (cmd/formspec, cmd/formspec-registry) is exactly the case that now
				// ships sidecars, so preferring them here removes work rather than
				// adding it.
				if br, ok := s.precompressed(c.name, c.info, raw); ok {
					brFiles.Add(1)
					brBytes.Add(int64(len(br)))
				}
				gz, ok := s.compressed(c.name, c.info, raw)
				if !ok {
					continue
				}
				warmed.Add(1)
				rawBytes.Add(int64(len(raw)))
				gzBytes.Add(int64(len(gz)))
			}
		}()
	}
	for _, c := range cands {
		work <- c
	}
	close(work)
	wg.Wait()

	entries, used := s.comp.stats()
	// File names are deliberately absent: the log reports volume, not business
	// content (platform/09-observability.md §PII).
	log.Printf("formspec: static asset pre-compress: %d file(s), %d → %d bytes in %s (brotli sidecars: %d, %d bytes; memo: %d entries, %d bytes)",
		warmed.Load(), rawBytes.Load(), gzBytes.Load(), time.Since(start).Round(time.Millisecond),
		brFiles.Load(), brBytes.Load(), entries, used)
}
