// forma-resource is the Resource Plane binary — the runtime that reads
// Forma YAML manifests and serves the REST API, WebSocket, and admin panel.
//
// Usage:
//
//	forma-resource [flags]
//
// Flags:
//
//	--dev          Run in development mode (single-tenant, SQLite, relaxed policy)
//	--spec <path>  Path to the project spec directory (default: ".")
//	--port <port>  HTTP listen port (default: 8080)
//	--db <dsn>     Database connection string (dev default: file:.forma/data.db)
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	devMode := flag.Bool("dev", false, "Run in development mode")
	specPath := flag.String("spec", ".", "Path to project spec directory")
	port := flag.Int("port", 8080, "HTTP listen port")
	flag.Parse()

	if *devMode {
		log.Println("[forma-resource] Starting in DEVELOPMENT mode")
		log.Println("[forma-resource]  - Single-tenant (no tenant isolation)")
		log.Println("[forma-resource]  - SQLite (file:.forma/data.db)")
		log.Println("[forma-resource]  - Relaxed policy (self-signed)")
		log.Println("[forma-resource]  - Hot-reload enabled")
	}

	log.Printf("[forma-resource] Spec path: %s", *specPath)
	log.Printf("[forma-resource] Listening on :%d", *port)

	// TODO: Initialize runtime
	// 1. Load and parse all YAML manifests from specPath
	// 2. Initialize database (SQLite for dev, PostgreSQL for prod)
	// 3. Register entities, services, configs
	// 4. Start HTTP server with API routes
	// 5. Start admin panel SPA server
	// 6. Connect to forma-control (if available)

	if _, err := os.Stat(*specPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: spec path %q does not exist\n", *specPath)
		os.Exit(1)
	}

	fmt.Println("forma-resource v0.1.0 — ready (no-op)")
	fmt.Println("(Runtime engine not yet implemented — see Fase 3)")
}
