// Command `formspec serve` — production single-server (todo Fase 8.1).
//
//	formspec serve --mode=production --dsn postgres://... --jwt-secret ... \
//	    --cors-origin https://app.example.com [--tls-cert cert.pem --tls-key key.pem] \
//	    [--metrics-addr :9102]
//
// Production mode disables every dev shortcut (todo 8.1.1):
//   - no dev auth / synthetic identity — JWT mandatory (todo 8.1.2):
//     HS256 via --jwt-secret, or RS256/ES256 via --jwt-public-key
//   - no auto-approve, no spec seeding
//   - Postgres DSN mandatory — SQLite refused (todo 8.1.4)
//   - CORS allow-list mandatory — `*` refused (todo 8.1.5)
//   - TLS supported via --tls-cert/--tls-key (todo 8.1.3)
//
// A separate admin listener (--metrics-addr, default :9102) exposes
// GET /metrics (Prometheus, todo 8.2.4) and GET /health (machine-readable,
// todo 8.2.6) — never business traffic.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/primadi/formspec/internal/observability"
	formspec "github.com/primadi/formspec/resource"
)

// runServe implements `formspec serve`.
func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	var (
		mode          = fs.String("mode", "production", "server mode: production (only mode implemented)")
		specPath      = fs.String("spec", "./spec", "manifest directory")
		dsn           = fs.String("dsn", "", "datastore DSN (production: postgres://... mandatory)")
		addr          = fs.String("addr", ":8080", "listen address")
		workspaceID   = fs.String("workspace", "default", "workspace ID")
		jwtSecret     = fs.String("jwt-secret", "", "HS256 shared secret (or --jwt-public-key)")
		jwtPublicKey  = fs.String("jwt-public-key", "", "PEM file with RSA/ECDSA public key (RS256/ES256)")
		jwtIssuer     = fs.String("jwt-issuer", "formspec", "expected JWT issuer")
		tlsCert       = fs.String("tls-cert", "", "TLS certificate PEM (enables HTTPS)")
		tlsKey        = fs.String("tls-key", "", "TLS private key PEM")
		metricsAddr   = fs.String("metrics-addr", ":9102", "admin listener (/metrics, /health); empty = disabled")
		logLevel      = fs.String("log-level", "info", "minimum log level (debug|info|warn|error)")
		logDebug      = fs.Bool("log-debug", false, "enable debug records (operator control — record this toggle)")
		invokeTimeout = fs.Duration("invoke-timeout", 30*time.Second, "sidecar invoke timeout")
		corsOrigins   = &repeatableFlag{}
	)
	fs.Var(corsOrigins, "cors-origin", "allowed CORS origin (repeatable, mandatory in production)")
	fs.Parse(args)

	if *mode != "production" {
		fmt.Fprintf(os.Stderr, "formspec serve: unsupported mode %q (want production; use `formspec dev` for development)\n", *mode)
		os.Exit(2)
	}

	// ── Production constraints (todo 8.1.1) ──
	fail := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "formspec serve: "+format+"\n", a...)
		os.Exit(1)
	}

	// 8.1.4 — Postgres mandatory in production.
	if *dsn == "" {
		fail("--dsn is required in production mode (Postgres)")
	}
	if strings.HasPrefix(*dsn, "sqlite:") {
		fail("SQLite is not allowed in production mode — use a postgres:// DSN (todo 8.1.4)")
	}

	// 8.1.2 — JWT mandatory: HS256 secret or RS256/ES256 public key.
	if *jwtSecret == "" && *jwtPublicKey == "" {
		fail("production mode requires --jwt-secret (HS256) or --jwt-public-key (RS256/ES256)")
	}

	// 8.1.5 — CORS allow-list mandatory; `*` refused.
	if len(corsOrigins.vals) == 0 {
		fail("production mode requires at least one --cors-origin <url> (CORS allow-list, todo 8.1.5)")
	}
	for _, o := range corsOrigins.vals {
		if o == "*" {
			fail("Access-Control-Allow-Origin: * is forbidden in production mode (todo 8.1.5)")
		}
	}

	// 8.1.3 — TLS pair must be complete and loadable.
	var tlsConfig *tls.Config
	if *tlsCert != "" || *tlsKey != "" {
		if *tlsCert == "" || *tlsKey == "" {
			fail("--tls-cert and --tls-key must be provided together")
		}
		cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		if err != nil {
			fail("load TLS key pair: %v", err)
		}
		tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}

	// ── Observability (todo 8.2) ──
	minLevel := observability.LevelInfo
	if *logLevel == "debug" || *logLevel == "warn" || *logLevel == "error" {
		minLevel = observability.Level(*logLevel)
	}
	logger := observability.NewLogger(os.Stdout, minLevel)
	logger.SetBase(observability.Fields{"environment": "production"})
	// Debug gate (todo 8.2.2): off by default; --log-debug is the explicit
	// operator control and must be recorded in the operator's audit trail.
	logger.SetDebugEnabled(*logDebug)

	metrics := observability.NewMetrics()
	health := observability.NewHealth()

	// ── Boot engine ──
	cfg := formspec.Config{
		SpecPath:             *specPath,
		DSN:                  *dsn,
		Addr:                 *addr,
		WorkspaceID:          *workspaceID,
		ProdMode:             true, // no dev auth, no seeding, strict uses (8.1.1)
		JWTSecret:            *jwtSecret,
		JWTPublicKeyPath:     *jwtPublicKey,
		JWTIssuer:            *jwtIssuer,
		CORSOrigins:          corsOrigins.vals,
		Logger:               logger,
		Metrics:              metrics,
		Health:               health,
		EnableAPIAuth:        true,
		SidecarInvokeTimeout: *invokeTimeout,
	}

	app, err := formspec.New(cfg)
	if err != nil {
		fail("engine boot: %v", err)
	}
	logger.Info(observability.Fields{
		"message": "engine loaded",
		"routes":  app.RouteCount(),
	})

	// ── Admin listener (todo 8.2.4/8.2.6) ──
	if *metricsAddr != "" {
		adminMux := http.NewServeMux()
		adminMux.Handle("/metrics", metrics.Handler())
		adminMux.Handle("/health", health.Handler())
		adminSrv := &http.Server{Addr: *metricsAddr, Handler: adminMux}
		go func() {
			log.Printf("[formspec] admin listener on %s (/metrics, /health)", *metricsAddr)
			if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("[formspec] admin listener error: %v", err)
			}
		}()
		defer adminSrv.Shutdown(context.Background())
	}

	// ── Serve ──
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app.StartBackgroundWorkers()
	srv := &http.Server{Addr: *addr, Handler: app.Handler(), TLSConfig: tlsConfig}
	errCh := make(chan error, 1)
	go func() {
		scheme := "http"
		if tlsConfig != nil {
			scheme = "https"
		}
		logger.Info(observability.Fields{
			"message": "REST API listening",
			"addr":    *addr,
			"scheme":  scheme,
		})
		if tlsConfig != nil {
			errCh <- srv.ListenAndServeTLS("", "")
		} else {
			errCh <- srv.ListenAndServe()
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info(observability.Fields{"message": "shutting down"})
	case err := <-errCh:
		if err != http.ErrServerClosed {
			logger.Error(observability.Fields{
				"message":    "server error",
				"error_code": "SERVER_ERROR",
			})
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

// repeatableFlag collects repeated --flag values (stdlib flag keeps only
// the last occurrence for plain string flags).
type repeatableFlag struct{ vals []string }

func (r *repeatableFlag) String() string { return strings.Join(r.vals, ",") }
func (r *repeatableFlag) Set(v string) error {
	r.vals = append(r.vals, v)
	return nil
}
