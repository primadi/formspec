// Command formspec-registry is the native production binary for the FormSpec
// Module Registry (todo 13.5.6 / Plan C): a thin wrapper that embeds the
// engine + the registry app spec (registry/embed.go, //go:embed) and
// registers native handlers — signature-verify server-side (13.3.3).
//
// Usage:
//
//	formspec-registry [--dsn postgres://...] [--addr :8080] [--spec <dir>]
//	                  [--prod] [--jwt-public-key <pem>] [--web-dir <dist>]
//
// When --spec is omitted, the embedded spec is extracted to a temp dir
// (single-file deployment). When --web-dir is omitted, the embedded SPA
// (registry/web, synced by `make build-registry`) is served instead.
// Native handlers registered here are unavailable
// in `formspec dev` — the signature-verify service only exists in this binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/primadi/formspec/internal/devserver"
	"github.com/primadi/formspec/internal/vendor"
	native "github.com/primadi/formspec/registry"
	web "github.com/primadi/formspec/registry/web"
	formspec "github.com/primadi/formspec/resource"
)

func main() {
	dsn := flag.String("dsn", "sqlite:.formspec/registry.db", "Database DSN (production: postgres://...)")
	specPath := flag.String("spec", "", "Spec directory (default: embedded spec extracted to temp)")
	addr := flag.String("addr", ":8080", "Listen address")
	prodMode := flag.Bool("prod", false, "Production mode (Postgres + JWT + strict gates)")
	devAuth := flag.Bool("dev-auth", true, "Dev mode + real JWT auth (login & authorization enforced)")
	jwtSecret := flag.String("jwt-secret", "", "JWT HMAC secret (dev only)")
	jwtIssuer := flag.String("jwt-issuer", "formspec-registry", "JWT issuer")
	jwtPublicKey := flag.String("jwt-public-key", "", "RSA/ECDSA public key PEM for asymmetric JWT")
	strictMode := flag.Bool("strict", false, "Strict uses enforcement")
	webDir := flag.String("web-dir", "", "Renderer SPA root (serves /{ws}/_admin and portal)")
	flag.Parse()

	// ── Dev DX (same as `formspec dev`): PID file + auto-kill previous ──
	// A second invocation kills the first instead of failing on the port.
	pidFile := filepath.Join(".formspec", "registry.pid")
	devserver.AutoKillPrevious(pidFile)
	devserver.WritePIDFile(pidFile)

	// ── Port conflict resolution ──
	// Kill a previous formspec-registry holding the port; a foreign process
	// yields a descriptive error. /proc/<pid>/comm truncates to 15 chars
	// ("formspec-regist"), so match that too.
	if err := devserver.EnsurePort(*addr, "formspec-regist"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Disk-backed spec (explicit --spec) → hot-reload watcher, same as
	// `formspec dev`. The embedded spec is a compile-time snapshot extracted
	// to a temp dir — watching it is useless, so log a hint instead.
	watchSpec := *specPath != ""

	if *specPath == "" {
		// Extract the embedded spec so Config.SpecPath (a disk path) can read it.
		tmp, err := os.MkdirTemp("", "formspec-registry-spec-*")
		if err != nil {
			log.Fatalf("extract embedded spec: %v", err)
		}
		if err := extractSpec(native.SpecFS(), tmp); err != nil {
			log.Fatalf("extract embedded spec: %v", err)
		}
		*specPath = filepath.Join(tmp, "spec")
	}

	fmt.Println("🚀 FormSpec Module Registry (native)")
	fmt.Printf("   spec: %s\n   dsn:  %s\n", *specPath, *dsn)

	// SPA source: explicit --web-dir wins; otherwise fall back to the
	// embedded renderer dist (registry/web) so the binary serves the admin
	// panel and portal out of the box.
	cfg := formspec.Config{
		DSN:              *dsn,
		SpecPath:         *specPath,
		Addr:             *addr,
		ProdMode:         *prodMode,
		DevAuth:          *devAuth,
		JWTSecret:        *jwtSecret,
		JWTIssuer:        *jwtIssuer,
		JWTPublicKeyPath: *jwtPublicKey,
		StrictMode:       *strictMode,
	}
	if *webDir != "" {
		cfg.WebDir = *webDir
		fmt.Printf("   web:  %s (from --web-dir)\n", *webDir)
	} else {
		cfg.WebFS = web.DistFS()
		fmt.Println("   web:  embedded SPA (registry/web/dist)")
	}

	app, err := formspec.New(cfg)
	if err != nil {
		log.Fatalf("boot: %v", err)
	}

	// ── Native handlers (13.3.3) — only in this binary ──
	app.RegisterNatives(map[string]formspec.NativeHandler{
		"registry.SignatureVerify": signatureVerify,
	})
	fmt.Println("✓ native handlers: registry.SignatureVerify")

	// ── Spec hot-reload (disk-backed --spec only) ──
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if watchSpec {
		go devserver.WatchSpec(ctx, app, *specPath, nil)
	} else {
		fmt.Println("ℹ spec: embedded snapshot — edit registry/spec + restart, or run with --spec registry/spec for hot-reload")
	}

	fmt.Printf("✓ Server starting on http://localhost%s\n", *addr)
	devserver.ServeAppUntilSignal(ctx, app)
	devserver.CleanupPIDFile(pidFile)
}

// signatureVerify implements the registry.signature-verify.verify service
// action: ed25519 verify of a tree checksum (13.3.3). Inputs (Params):
// checksum, signature, public_key — all base64/hex strings as produced by
// `formspec sign`. Output: { valid: bool, error: string }.
func signatureVerify(_ context.Context, params formspec.NativeParams) (any, error) {
	checksum, _ := params.Params["checksum"].(string)
	signature, _ := params.Params["signature"].(string)
	publicKey, _ := params.Params["public_key"].(string)
	if checksum == "" || signature == "" || publicKey == "" {
		return map[string]any{"valid": false, "error": "checksum, signature, and public_key are required"}, nil
	}
	if err := vendor.VerifyChecksum(publicKey, checksum, signature); err != nil {
		return map[string]any{"valid": false, "error": err.Error()}, nil
	}
	return map[string]any{"valid": true}, nil
}

// extractSpec writes the embedded spec tree to dest (which becomes the
// parent of the "spec/" directory).
func extractSpec(src fs.FS, dest string) error {
	return fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == "." {
			return err
		}
		target := filepath.Join(dest, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}
