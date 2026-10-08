package main

import (
	"fmt"
	"os"

	"github.com/primadi/formspec/internal/api"
)

// `formspec spa compress` — build-time brotli pass (todo 8.4.4).
//
// This is a CLI verb rather than a shell one-liner in the Makefile so the rule
// for "is this file worth compressing" stays in ONE place: the same Go code that
// serves the assets (internal/api). A shell/Node implementation would be a
// second copy of `isCompressible` + the size floor, and the failure mode of
// drift is silent — a stricter copy just loses compression on files nobody
// looks at.
//
// Plan: docs_internal/plan/brotli-precompress-build.md.
func runSpaCompress(args []string) {
	dir := "renderers/react-shadcn/dist"
	quality := 11
	quiet := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dir":
			if i+1 >= len(args) {
				fatalSpaCompress("--dir butuh nilai")
			}
			dir = args[i+1]
			i++
		case "--quality":
			if i+1 >= len(args) {
				fatalSpaCompress("--quality butuh nilai")
			}
			n, err := parseQuality(args[i+1])
			if err != nil {
				fatalSpaCompress(err.Error())
			}
			quality = n
			i++
		case "--quiet", "-q":
			quiet = true
		case "-h", "--help", "help":
			_, _ = fmt.Fprint(os.Stdout, spaCompressUsage)
			return
		default:
			fatalSpaCompress("argumen tidak dikenal: " + args[i])
		}
	}

	if _, err := os.Stat(dir); err != nil {
		fatalSpaCompress(fmt.Sprintf("direktori tidak ditemukan: %s", dir))
	}

	opts := api.DefaultBrotliOptions()
	opts.Quality = quality

	stats, err := api.CompressStaticTree(dir, opts)
	if err != nil {
		fatalSpaCompress(err.Error())
	}
	if !quiet {
		total := int64(1)
		if stats.RawBytes > 0 {
			total = stats.RawBytes
		}
		ratio := 100 * float64(stats.BrotliByte) / float64(total)
		fmt.Printf("✅ pre-compress (brotli q%d): %d file, %d → %d bytes (%.1f%%) dalam %dms\n",
			opts.Quality, stats.Written, stats.RawBytes, stats.BrotliByte, ratio, stats.Elapsed)
		if stats.Skipped > 0 {
			fmt.Printf("   %d berkas dilewati (terlalu kecil / tidak menyusut / > 8 MiB)\n", stats.Skipped)
		}
		if stats.Stale > 0 {
			fmt.Printf("   %d sidecar .br basi dihapus\n", stats.Stale)
		}
	}
}

const spaCompressUsage = `Usage: formspec spa compress [--dir <dist>] [--quality 0-11] [--quiet]

Tulis sidecar <file>.br (brotli) untuk aset kompresibel di <dist>, dan hapus
sidecar yang sumbernya tidak lagi layak. Dijalankan SETELAH build SPA — bukan
saat request: biaya encoder (q11 ≈ 1,7 s/MiB) dibayar sekali di sini, sementara
klien menanggung nol biaya decode tambahan untuk kualitas yang lebih tinggi.

Default: --dir renderers/react-shadcn/dist --quality 11
`

func parseQuality(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("--quality harus angka 0-11, dapat %q", s)
		}
		n = n*10 + int(r-'0')
	}
	if n > 11 {
		return 0, fmt.Errorf("--quality maksimum 11 (brotli), dapat %d", n)
	}
	return n, nil
}

func fatalSpaCompress(msg string) {
	_, _ = fmt.Fprintf(os.Stderr, "❌ formspec spa compress: %s\n\n%s", msg, spaCompressUsage)
	os.Exit(1)
}
