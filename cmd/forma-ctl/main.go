// forma-ctl is the Control Plane binary — governance, policy, keys,
// environments, contracts, transparency log, artifact registration,
// snapshot serving, and evidence collection.
//
// It never reads business data and never executes business logic.
//
// forma-ctl also hosts the Cloud Owner emergency CLI (freeze, revoke
// sessions, key rotate, policy test, log verify) as conventional code
// inside this same binary — never a separate process with its own
// dependencies, so it keeps working when the platform it is repairing does
// not (see docs/spec/11-reference.md D43, docs/cli-tools/02-forma-ctl.md).
// The emergency subcommands are not implemented yet — see
// docs/cli-tools/02-forma-ctl.md §5.
//
// Usage:
//
//	forma-ctl serve [flags]
//
// Flags:
//
//	--dev          Run in development mode (HTTP, self-signed, relaxed policy)
//	--port <port>  HTTP listen port (default: 8443)
//	--control-db  Database path for control state (dev default: .forma/control.db)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/primadi/forma/internal/artifact"
	"github.com/primadi/forma/internal/control"
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
		fmt.Fprintf(os.Stderr, "forma-ctl %s: not implemented yet — see docs/cli-tools/02-forma-ctl.md §5\n", os.Args[1])
		os.Exit(1)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: forma-ctl <command> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  serve    Run the Control Plane server (region/cluster/standalone)\n")
	fmt.Fprintf(os.Stderr, "\nNot yet implemented (see docs/cli-tools/02-forma-ctl.md):\n")
	fmt.Fprintf(os.Stderr, "  freeze, revoke sessions, key rotate, policy test, log verify\n")
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	devMode := fs.Bool("dev", false, "Run in development mode")
	port := fs.Int("port", 8443, "HTTP listen port")
	controlDB := fs.String("control-db", ".forma/control.db", "Path to control database")
	fs.Parse(args)

	if *devMode {
		log.Println("[forma-ctl] Starting in DEVELOPMENT mode")
		log.Println("[forma-ctl]  - HTTP (no mTLS)")
		log.Println("[forma-ctl]  - Self-signed artifact signatures")
		log.Println("[forma-ctl]  - No approval gates")
		log.Println("[forma-ctl]  - Dev poll endpoint enabled")
	}

	// Ensure data directory exists
	if err := os.MkdirAll(".forma", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .forma directory: %v\n", err)
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
	log.Printf("[forma-ctl] Signing key: %s (public: %x)",
		signer.KeyID(), signer.PublicKey()[:8])

	// Create and start server
	svr := control.NewServer(store, signer, *port, *devMode)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("[forma-ctl] Shutting down...")
		svr.Stop(context.Background())
	}()

	log.Printf("[forma-ctl] Listening on :%d", *port)

	if err := svr.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
