// forma-control is the Control Plane binary — governance, policy, keys,
// environments, contracts, and transparency log.
//
// It never reads business data and never executes business logic.
//
// Usage:
//
//	forma-control [flags]
//
// Flags:
//
//	--dev          Run in development mode (self-signed, relaxed policy)
//	--port <port>  gRPC listen port (default: 8443)
//	--db <dsn>     Database connection string (dev default: file:.forma/control.db)
package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	devMode := flag.Bool("dev", false, "Run in development mode")
	port := flag.Int("port", 8443, "gRPC listen port")
	flag.Parse()

	if *devMode {
		log.Println("[forma-control] Starting in DEVELOPMENT mode")
		log.Println("[forma-control]  - Self-signed certificates")
		log.Println("[forma-control]  - Relaxed policy (no signing required)")
		log.Println("[forma-control]  - No approval gates")
	}

	log.Printf("[forma-control] Listening on :%d (gRPC + mTLS)", *port)

	// TODO: Initialize control plane
	// 1. Load/initialize policy store
	// 2. Start OPA engine
	// 3. Start gRPC server with mTLS
	// 4. Serve desired-state channel (snapshot)
	// 5. Accept evidence channel (append-only)
	// 6. Initialize transparency log

	fmt.Println("forma-control v0.1.0 — ready (no-op)")
	fmt.Println("(Control plane not yet implemented — see Fase 5)")
}
