package api

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
)

// Build-time brotli pre-compression (todo 8.4.4, plan brotli-precompress-build.md).
//
// This lives in internal/api — next to the code that SERVES the assets — on
// purpose. The rule for "is this file worth compressing" is one rule, and a
// build-time implementation written in shell/Node would be a second copy of it
// that can drift. Drift here is silent: a stricter copy just loses compression
// on some files, and a looser one burns CPU for nothing. Sharing
// `isCompressible` / `minCompressSize` / `compressEligible` means the build
// pass and the request path cannot disagree.
//
// Why brotli at all, given gzip already runs at request time: measured over the
// real 1.6 MB bundle, brotli q11 produces 369 KB where gzip-6 produces 442 KB
// (−16.5%), and — the part that decides it — decoding q11 is NOT more expensive
// than decoding q5 (~6.0 ms vs 6.2 ms). Encoder quality is an encode-side knob;
// the client pays nothing for it. The 2.8 s encoder cost is paid once here,
// instead of per request.

// compressEligible decides whether a file is a candidate for pre-compression.
//
// Both the build pass and (transitively) the request path derive their decision
// from this, so the two cannot disagree about which files are worth the effort.
func compressEligible(name string, size int64) bool {
	return isCompressible(name) && size >= minCompressSize && size <= maxPrecompressFile
}

// maxPrecompressFile caps the build-time pass. Quality 11 costs ~1.7 s per MiB,
// so an unexpectedly large artifact (a stray video, a database dump dropped in
// dist/) would stall the build. Assets this big also compress poorly relative to
// the time spent. The runtime gzip path keeps its own, smaller warm ceiling.
const maxPrecompressFile = 8 << 20 // 8 MiB

// brotliSidecar is the suffix for a pre-compressed sibling. It is deliberately
// an ordinary suffix (not a dotfile): `cp -r dist/*` in the Makefile and
// `//go:embed all:dist` both pick up `name.js.br` without special handling.
const brotliSidecar = ".br"

// BrotliOptions configures a pre-compression pass.
type BrotliOptions struct {
	// Quality is the brotli encoder quality (0..11). 11 is the default because
	// the cost is paid once at build time and the client pays nothing for it.
	Quality int
	// LGWin is the base-2 log of the encoder window (10..24, 0 = codec default,
	// 22 = 4 MiB). It is NOT raised to 24: the decoder allocates a window per
	// stream, and 4 MiB already exceeds every asset in this bundle, so a larger
	// window would cost client memory without buying any ratio.
	LGWin int
}

// DefaultBrotliOptions is the setting the Makefile drives.
func DefaultBrotliOptions() BrotliOptions {
	return BrotliOptions{Quality: 11, LGWin: 22}
}

// BrotliStats reports what a pass did. Reported in full so a build log shows
// both the win and the work skipped — a pass that silently compressed nothing
// would otherwise look identical to one that succeeded.
type BrotliStats struct {
	Written    int   // sidecars written
	Skipped    int   // eligible-looking files refused (too small, did not shrink, oversize)
	Stale      int   // sidecars removed because their source no longer qualifies
	RawBytes   int64 // bytes of source covered by Written
	BrotliByte int64 // bytes actually written
	Elapsed    int64 // milliseconds
}

// CompressStaticTree writes `<file>.br` siblings for the compressible files under
// dir, and removes sidecars whose source no longer qualifies.
//
// It is idempotent and safe to re-run: every sidecar is rewritten from its
// source, and a `.br` is only kept when it is actually smaller. Running it twice
// produces identical output, so a build that re-runs does not accumulate state.
// The results are NAMED so the deferred timer can write Elapsed into them: with
// unnamed results, `return stats, nil` copies the struct into the result slots
// before deferred functions run, and the assignment would be lost.
func CompressStaticTree(dir string, opts BrotliOptions) (stats BrotliStats, err error) {
	start := time.Now()
	// Elapsed is reported even on the error paths below, so a failed pass still
	// says how long it spent before failing.
	defer func() { stats.Elapsed = time.Since(start).Milliseconds() }()
	root := os.DirFS(dir)

	// Collect the work first so it can be parallelized; quality 11 is slow
	// enough (~1.7 s/MiB) that a serial pass is noticeable on a real bundle.
	type job struct {
		name string
		size int64
	}
	var (
		jobs     []job
		sidecars []string // every .br under the tree, to prune later
	)
	err = fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, brotliSidecar) {
			sidecars = append(sidecars, p)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if !compressEligible(p, info.Size()) {
			stats.Skipped++
			return nil
		}
		jobs = append(jobs, job{name: p, size: info.Size()})
		return nil
	})
	if err != nil {
		return stats, fmt.Errorf("walk %s: %w", dir, err)
	}

	// Sources that qualify, so pruning can tell "stale" from "fine".
	qualified := make(map[string]bool, len(jobs))

	workers := runtime.GOMAXPROCS(0)
	if workers > len(jobs) {
		workers = len(jobs)
	}
	if workers < 1 {
		workers = 1
	}

	var (
		mu       sync.Mutex
		work     = make(chan job)
		wg       sync.WaitGroup
		firstErr error
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range work {
				raw, err := fs.ReadFile(root, j.name)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("read %s: %w", j.name, err)
					}
					mu.Unlock()
					continue
				}
				var buf bytes.Buffer
				w := brotli.NewWriterOptions(&buf, brotli.WriterOptions{
					Quality: opts.Quality,
					LGWin:   opts.LGWin,
				})
				if _, err := w.Write(raw); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("compress %s: %w", j.name, err)
					}
					mu.Unlock()
					continue
				}
				if err := w.Close(); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("compress %s: %w", j.name, err)
					}
					mu.Unlock()
					continue
				}
				// A sidecar that is not smaller would make the served response
				// bigger than the file it represents — the same gate the runtime
				// path applies to gzip.
				if buf.Len() >= len(raw) {
					mu.Lock()
					stats.Skipped++
					mu.Unlock()
					continue
				}
				if err := writeSidecar(filepath.Join(dir, filepath.FromSlash(j.name)), buf.Bytes()); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					continue
				}
				mu.Lock()
				stats.Written++
				stats.RawBytes += int64(len(raw))
				stats.BrotliByte += int64(buf.Len())
				qualified[j.name] = true
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		work <- j
	}
	close(work)
	wg.Wait()
	if firstErr != nil {
		return stats, firstErr
	}

	// Prune: a `.br` whose source is gone, is no longer eligible, or was not
	// written by this pass is stale. Vite empties dist/ on every build, so this
	// is a safety net rather than the main mechanism — but a stale sidecar would
	// be SERVED (the request path only checks that it exists and is smaller), so
	// leaving one behind is a correctness problem, not just clutter.
	sort.Strings(sidecars)
	for _, sc := range sidecars {
		src := strings.TrimSuffix(sc, brotliSidecar)
		if qualified[src] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(sc))); err != nil {
			log.Printf("formspec: pre-compress: remove stale %s: %v", sc, err)
			continue
		}
		stats.Stale++
	}

	return stats, nil
}

// writeSidecar writes the compressed bytes next to the source via a temp file and
// a rename, so a reader never observes a half-written `.br`.
//
// The sidecar inherits the SOURCE file's permission bits rather than the temp
// file's: os.CreateTemp creates 0600, which would leave dist/ with
// root-only-readable assets — a tree that serves fine for the user who built it
// and 403s for anyone else (a different user, a container, a copied deployment).
func writeSidecar(src string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(src); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(src)
	tmp, err := os.CreateTemp(dir, filepath.Base(src)+".brtmp*")
	if err != nil {
		return fmt.Errorf("create sidecar for %s: %w", src, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write sidecar for %s: %w", src, err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod sidecar for %s: %w", src, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close sidecar for %s: %w", src, err)
	}
	if err := os.Rename(tmpName, src+brotliSidecar); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("install sidecar for %s: %w", src, err)
	}
	return nil
}
