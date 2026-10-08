// Command formspec-registry is the native production binary for the FormSpec
// Module Registry (todo 13.5.6 / Plan C): a thin wrapper that embeds the
// engine + the registry app spec (app-spec/embed.go, //go:embed) and
// registers native handlers — signature-verify server-side (13.3.3).
//
// Usage:
//
//	formspec-registry [--dsn postgres://...] [--addr :8080] [--spec <dir>]
//	                  [--prod] [--jwt-public-key <pem>] [--web-dir <dist>]
//
// When --spec is omitted, the embedded spec is extracted to a temp dir
// (single-file deployment). When --web-dir is omitted, the embedded SPA
// (web/, synced by `make build-registry`) is served instead.
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
	"strings"
	"syscall"

	appspec "github.com/primadi/formspec/cmd/formspec-registry/app-spec"
	web "github.com/primadi/formspec/cmd/formspec-registry/web"
	"github.com/primadi/formspec/internal/api"
	"github.com/primadi/formspec/internal/devsecret"
	"github.com/primadi/formspec/internal/devserver"
	"github.com/primadi/formspec/internal/vendor"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
	formspec "github.com/primadi/formspec/resource"
	"gopkg.in/yaml.v3"
)

func main() {
	dsn := flag.String("dsn", "", "Database DSN (default: sqlite:.formspec/registry.db; production: postgres://...)")
	specPath := flag.String("spec", "", "Spec directory (default: embedded spec extracted to temp)")
	addr := flag.String("addr", "", "Listen address (default: :8080)")
	prodMode := flag.Bool("prod", false, "Production mode (Postgres + JWT + strict gates)")
	jwtSecret := flag.String("jwt-secret", "", "JWT HMAC secret (dev: auto-generated + persisted in .formspec/dev-jwt-secret when empty)")
	jwtIssuer := flag.String("jwt-issuer", "formspec-registry", "JWT issuer")
	jwtPublicKey := flag.String("jwt-public-key", "", "RSA/ECDSA public key PEM for asymmetric JWT")
	strictMode := flag.Bool("strict", false, "Strict uses enforcement")
	webDir := flag.String("web-dir", "", "Renderer SPA root (serves /{ws}/_admin and portal)")
	configFile := flag.String("config", "", "Config file (formspec-app.yaml format; auto-discovered in CWD when omitted)")
	flag.Parse()

	// ── Config file (formspec-app.yaml) — CLI flags win ──
	applyConfigFile(configFile, &specPath, &dsn, &addr, &jwtSecret, &webDir)

	// ── Defaults for values not set by CLI or config file ──
	if *dsn == "" {
		*dsn = "sqlite:.formspec/registry.db"
	}
	if *addr == "" {
		*addr = ":8080"
	}

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
		_, _ = fmt.Fprintln(os.Stderr, err)
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
		if err := extractSpec(appspec.SpecFS(), tmp); err != nil {
			log.Fatalf("extract embedded spec: %v", err)
		}
		*specPath = filepath.Join(tmp, "spec")
	}

	fmt.Println("🚀 FormSpec Module Registry (native)")
	fmt.Printf("   spec: %s\n   dsn:  %s\n", *specPath, *dsn)

	// SPA source: explicit --web-dir wins; otherwise fall back to the
	// embedded renderer dist (web/) so the binary serves the admin
	// panel and portal out of the box.
	// Auth is uniform across dev and prod (always real JWT). In dev, when no
	// explicit secret is configured, resolve (or generate + persist) the dev
	// secret so sessions survive restarts.
	if !*prodMode && *jwtSecret == "" {
		secret, generated, err := devsecret.Resolve(".formspec")
		if err != nil {
			log.Fatalf("resolve dev jwt secret: %v", err)
		}
		*jwtSecret = secret
		if generated {
			fmt.Printf("   jwt:  generated dev secret → %s\n", filepath.Join(".formspec", devsecret.FileName))
		}
	}

	cfg := formspec.Config{
		DSN:              *dsn,
		SpecPath:         *specPath,
		Addr:             *addr,
		ProdMode:         *prodMode,
		JWTSecret:        *jwtSecret,
		JWTIssuer:        *jwtIssuer,
		JWTPublicKeyPath: *jwtPublicKey,
		StrictMode:       *strictMode,
	}
	// SPA source, in priority order:
	//   1. --web-dir                 (explicit; wins over everything)
	//   2. renderers/react-shadcn/dist   (repo source — freshest in dev)
	//   3. cmd/formspec-registry/web/dist (the synced copy this binary embeds)
	//   4. the embedded dist          (deploy: fast, and works without a repo)
	//
	// Step 2 exists because `make registry-dev` runs `go run`, which compiles
	// WITHOUT -tags formspec_spa — so web/embed.go (the real dist) is not part
	// of the build at all and embed_stub.go answers with a placeholder page.
	// "There is an embed.go" does not mean the embed is used: the build tag
	// decides. Auto-detecting the checkout keeps the dev server honest instead
	// of making the fix "remember to pass --web-dir".
	//
	// Step 3 is the same bundle one directory over: `make build-registry` syncs
	// renderers/react-shadcn/dist into cmd/formspec-registry/web/dist for the
	// //go:embed, and that copy stays on disk (gitignored) after the build. It
	// is the fallback when the repo dist is absent — a checkout where only
	// `make build-registry` was ever run, or dist/ was cleaned. The repo dist
	// comes first because it is what `npm run build` refreshes.
	//
	// Auto-detection is deliberately skipped when the embedded build HAS a real
	// dist (in-process App plugins, build-tag builds): a package must not have
	// its UI swapped by whatever checkout happens to sit above its CWD.
	placeholder := false
	switch {
	case *webDir != "":
		cfg.WebDir = *webDir
		fmt.Printf("   web:  %s (from --web-dir)\n", *webDir)
	default:
		cfg.WebFS = web.DistFS()
		switch {
		case web.Embedded:
			fmt.Println("   web:  embedded SPA (web/dist)")
		default:
			// Renderer source first (freshest), then the binary's own synced
			// copy (same bundle, survives a cleaned renderers/).
			detected := devserver.FindWebDist()
			origin := "auto-detected"
			if detected == "" {
				detected = devserver.FindDistUpwards("cmd", "formspec-registry", "web", "dist")
				origin = "auto-detected (synced copy)"
			}
			if detected != "" {
				cfg.WebDir = detected
				cfg.WebFS = nil
				fmt.Printf("   web:  %s (%s; go run has no embedded SPA)\n", detected, origin)
			} else {
				placeholder = true
				fmt.Println("   web:  ⚠ placeholder — binary built without embedded SPA " +
					"(go install / tanpa -tags formspec_spa) and no repository checkout found. " +
					"Full UI: make build-registry, or --web-dir <renderers/react-shadcn/dist>")
			}
		}
	}

	app, err := formspec.New(cfg)
	if err != nil {
		log.Fatalf("boot: %v", err)
	}

	// ── Native handlers (13.3.3) — only in this binary ──
	app.RegisterNatives(map[string]formspec.NativeHandler{
		"registry.SignatureVerify": signatureVerify,
		"registry.vendor.approve":  vendorApprove(app),
	})
	fmt.Println("✓ native handlers: registry.SignatureVerify, registry.vendor.approve")

	// ── Spec hot-reload (disk-backed --spec only) ──
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if watchSpec {
		go devserver.WatchSpec(ctx, app, *specPath, nil)
	} else {
		fmt.Println("ℹ spec: embedded snapshot — edit cmd/formspec-registry/app-spec/spec + restart, or run with --spec cmd/formspec-registry/app-spec/spec for hot-reload")
	}

	// ── Where to open the UI ──
	//
	// The workspace prefix is not decoration: every surface is served under
	// /{ws}/... (D50), so the bare origin is a 404 and printing it as "the
	// server is here" sends the visitor straight into that 404. The URL comes
	// from the resolved Apps (root_url), never from a hardcoded slug or the
	// retired /{ws}/_admin panel (plan app-scoped-login.md D4) — /_admin now
	// carries only framework auth routes.
	appURLs := app.UIAppURLs()
	if len(appURLs) == 0 {
		appURLs = []string{spec.SurfaceURL(cfg.WorkspaceID, "/")}
	}
	primary := appURLs[0]

	fmt.Printf("✓ Server starting on http://localhost%s\n", *addr)
	if placeholder {
		fmt.Println("⚠ The UI is NOT served — the URL below answers with a placeholder page.")
		fmt.Println("  Fix: make build-registry (embed) atau --web-dir renderers/react-shadcn/dist")
	}
	fmt.Printf("  App  : http://localhost%s%s\n", *addr, primary)
	for _, u := range appURLs[1:] {
		fmt.Printf("  App  : http://localhost%s%s\n", *addr, u)
	}
	fmt.Printf("  Setup: http://localhost%s%s/_admin/setup\n", *addr, strings.TrimSuffix(primary, "/"))
	fmt.Printf("  Framework routes only (no entity panel; D4): http://localhost%s%s/_admin/{setup,change-password,oauth/callback}\n",
		*addr, strings.TrimSuffix(primary, "/"))
	devserver.ServeAppUntilSignal(ctx, app)
	devserver.CleanupPIDFile(pidFile)
}

// ─── Config file (formspec-app.yaml) ───
//
// Sama formatnya dengan config `formspec dev` (cmd/formspec/dev_config.go).
// Nilai config hanya berlaku untuk flag yang tidak di-set CLI.

// registryConfigFile adalah subset field formspec-app.yaml yang relevan
// untuk formspec-registry.
type registryConfigFile struct {
	Spec      *string `yaml:"spec"`
	DSN       *string `yaml:"dsn"`
	Addr      *string `yaml:"addr"`
	JWTSecret *string `yaml:"jwt-secret"`
	WebDir    *string `yaml:"web-dir"`
}

// applyConfigFile membaca config file dan mengisi nilai flag yang masih
// kosong (CLI menang). Tanpa --config, auto-discover formspec-app.yaml di
// CWD (legacy formspec-sidecar.yaml ikut didukung).
func applyConfigFile(configFile *string, specPath, dsn, addr, jwtSecret, webDir **string) {
	path := *configFile
	if path == "" {
		for _, c := range []string{"formspec-app.yaml", "formspec-app.yml", "formspec-sidecar.yaml"} {
			if _, err := os.Stat(c); err == nil {
				path = c
				break
			}
		}
	}
	if path == "" {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read config %s: %v", path, err)
	}
	var cf registryConfigFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		log.Fatalf("parse config %s: %v", path, err)
	}
	fmt.Printf("⚙ config: %s\n", path)

	if cf.Spec != nil && **specPath == "" {
		**specPath = *cf.Spec
	}
	if cf.DSN != nil && **dsn == "" {
		**dsn = *cf.DSN
	}
	if cf.Addr != nil && **addr == "" {
		**addr = *cf.Addr
	}
	if cf.JWTSecret != nil && **jwtSecret == "" {
		**jwtSecret = *cf.JWTSecret
	}
	if cf.WebDir != nil && **webDir == "" {
		**webDir = *cf.WebDir
	}
}

// vendorApprove implements the registry.vendor.approve action (Fase 6 —
// vendor upgrade flow): an admin approves a pending vendor application →
// vendor status becomes active + the owner user is granted the `vendor` role
// and registry permissions (registry.vendor.*, registry.module.*).
func vendorApprove(app *formspec.App) formspec.NativeHandler {
	return func(ctx context.Context, params formspec.NativeParams) (any, error) {
		owner, _ := params.Resource["owner_username"].(string)
		if owner == "" {
			return nil, fmt.Errorf("vendor has no owner_username")
		}

		// 1. Activate the vendor record (status → active) with CAS.
		store, err := app.Registry().GetEntityStore("registry", "vendor")
		if err != nil {
			return nil, fmt.Errorf("resolve vendor store: %w", err)
		}
		rec, err := store.GetByID(ctx, db.GetByIDParams{WorkspaceID: params.WorkspaceID, ID: params.ResourceID})
		if err != nil {
			return nil, fmt.Errorf("load vendor: %w", err)
		}
		data := map[string]any{}
		for k, v := range rec.Data {
			data[k] = v
		}
		data["status"] = "active"
		if _, err := store.Update(ctx, db.UpdateParams{
			WorkspaceID: params.WorkspaceID,
			ID:          params.ResourceID,
			Version:     rec.Version,
			UpdatedBy:   params.UserID,
			Data:        data,
			// Registry activation writes an engine-owned status field as part
			// of approving a vendor — a framework transition, not a caller edit.
			SystemCaller: true,
		}); err != nil {
			return nil, fmt.Errorf("activate vendor: %w", err)
		}

		// 2. Grant the owner user the vendor role + registry permissions.
		svc := api.GetAuthService()
		if svc == nil {
			return nil, fmt.Errorf("auth service not configured")
		}
		if err := svc.GrantRoles(ctx, params.WorkspaceID, owner,
			[]string{"vendor"},
			[]string{"registry.vendor.*", "registry.module.*"},
		); err != nil {
			return nil, fmt.Errorf("grant vendor role to %s: %w", owner, err)
		}

		return map[string]any{"approved": true, "vendor": params.ResourceID, "owner": owner}, nil
	}
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
