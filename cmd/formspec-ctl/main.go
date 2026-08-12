// formspec-ctl is the Control Plane binary — governance, policy, keys,
// environments, contracts, transparency log, artifact registration,
// snapshot serving, and evidence collection.
//
// It never reads business data and never executes business logic.
//
// formspec-ctl also hosts the Cloud Owner emergency CLI (freeze, revoke
// sessions, key rotate, policy test, log verify) as conventional code
// inside this same binary — never a separate process with its own
// dependencies, so it keeps working when the platform it is repairing does
// not (see docs_old/spec/11-reference.md D43, docs/cli-tools/02-formspec-ctl.md).
// The emergency subcommands are not implemented yet — see
// docs/cli-tools/02-formspec-ctl.md §5.
//
// Usage:
//
//	formspec-ctl serve [flags]
//
// Flags:
//
//	--dev          Run in development mode (HTTP, self-signed, relaxed policy)
//	--port <port>  HTTP listen port (default: 8443)
//	--control-db  Database path for control state (dev default: .formspec/control.db)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/primadi/formspec/internal/artifact"
	"github.com/primadi/formspec/internal/control"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "freeze", "revoke", "key", "policy", "log":
		fmt.Fprintf(os.Stderr, "formspec-ctl %s: not implemented yet — see docs/cli-tools/02-formspec-ctl.md §5\n", os.Args[1])
		os.Exit(1)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: formspec-ctl <command> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  serve    Run the Control Plane server (region/cluster/standalone)\n")
	fmt.Fprintf(os.Stderr, "\nNot yet implemented (see docs/cli-tools/02-formspec-ctl.md):\n")
	fmt.Fprintf(os.Stderr, "  freeze, revoke sessions, key rotate, policy test, log verify\n")
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	devMode := fs.Bool("dev", false, "Run in development mode")
	port := fs.Int("port", 8443, "HTTP listen port")
	controlDB := fs.String("control-db", ".formspec/control.db", "Path to control database")
	fs.Parse(args)

	if *devMode {
		log.Println("[formspec-ctl] Starting in DEVELOPMENT mode")
		log.Println("[formspec-ctl]  - HTTP (no mTLS)")
		log.Println("[formspec-ctl]  - Self-signed artifact signatures")
		log.Println("[formspec-ctl]  - No approval gates")
		log.Println("[formspec-ctl]  - Dev poll endpoint enabled")
	}

	// Ensure data directory exists
	if err := os.MkdirAll(".formspec", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .formspec directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize artifact store (in-memory for dev, SQLite/Postgres for prod)
	store := artifact.NewMemStore()
	_ = controlDB // TODO: wire SQLite/Postgres store

	// Initialize signer
	signer, err := artifact.NewDevSigner()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating signer: %v\n", err)
		os.Exit(1)
	}
	log.Printf("[formspec-ctl] Signing key: %s (public: %x)",
		signer.KeyID(), signer.PublicKey()[:8])

	// Create and start server
	svr := control.NewServer(store, signer, *port, *devMode)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("[formspec-ctl] Shutting down...")
		svr.Stop(context.Background())
	}()

	log.Printf("[formspec-ctl] Listening on :%d", *port)

	if err := svr.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
