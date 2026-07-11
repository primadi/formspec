// Command reference-app is a minimal example of embedding the Forma
// Resource engine into a Go application — see docs/runtimes/02-forma-resource.md
// for the full embedding contract.
//
// Usage:
//
//	go run ./examples/reference-app [--dsn sqlite:.forma/data.db] [--spec ./examples/Customer/spec] [--addr :8080]
package main

import (
	"flag"
	"fmt"
	"log"

	forma "github.com/primadi/forma/resource"
)

func main() {
	dsn := flag.String("dsn", "sqlite:.forma/data.db", "Database DSN")
	specPath := flag.String("spec", "./examples/Customer/spec", "Path to spec directory")
	addr := flag.String("addr", ":8080", "Listen address")
	prodMode := flag.Bool("prod", false, "Enable production mode (JWT auth)")
	jwtSecret := flag.String("jwt-secret", "", "JWT signing secret (required in prod mode for HMAC)")
	jwtIssuer := flag.String("jwt-issuer", "forma", "JWT issuer")
	jwtPublicKey := flag.String("jwt-public-key", "", "Path to RSA/ECDSA public key file (PEM) for asymmetric JWT validation")
	strictMode := flag.Bool("strict", false, "Enable strict enforcement of uses declarations")
	webDir := flag.String("web-dir", "", "Renderer SPA root (web/dist) — serves /{ws}/_admin and /{ws}/app")
	flag.Parse()

	fmt.Println("🚀 Forma Reference App")
	fmt.Println("======================")

	app, err := forma.New(forma.Config{
		DSN:              *dsn,
		SpecPath:         *specPath,
		Addr:             *addr,
		ProdMode:         *prodMode,
		JWTSecret:        *jwtSecret,
		JWTIssuer:        *jwtIssuer,
		JWTPublicKeyPath: *jwtPublicKey,
		StrictMode:       *strictMode,
		WebDir:           *webDir,
	})
	if err != nil {
		log.Fatalf("❌ %v", err)
	}

	fmt.Printf("✓ Entities loaded: %d\n", app.Registry().Count())
	fmt.Printf("✓ Routes generated: %d\n", app.RouteCount())
	for _, rd := range app.Routes() {
		fmt.Printf("  %s %s/%s/%s\n", rd.Method, rd.Module, rd.Plural, rd.Action)
	}

	fmt.Printf("\n✓ Server starting on http://localhost%s\n", *addr)
	fmt.Println("  Try:")
	fmt.Println("    curl http://localhost" + *addr + "/demo/api/v1/billing/customers")
	fmt.Println("    curl http://localhost" + *addr + "/health")

	log.Fatal(app.ListenAndServe())
}
