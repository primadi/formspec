// Command forma-serve starts the Forma API server.
//
// Usage:
//
//	go run ./cmd/forma-serve/ [--dsn sqlite:.forma/data.db] [--spec ./examples/Customer/spec] [--addr :8080]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forma/forma/internal/action"
	"github.com/forma/forma/internal/api"
	"github.com/forma/forma/internal/auth"
	"github.com/forma/forma/internal/db"
	"github.com/forma/forma/internal/entity"
	"github.com/forma/forma/internal/permission"
	"github.com/forma/forma/internal/validation"
	"github.com/forma/forma/pkg/spec"

	"github.com/golang-jwt/jwt/v5"
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
	idempotencyTTL := flag.Duration("idempotency-ttl", db.DefaultIdempotencyTTL, "TTL for idempotency keys (default 24h; config key: core.idempotency_retention)")
	flag.Parse()

	ctx := context.Background()

	fmt.Println("🚀 Forma API Server")
	fmt.Println("==================")

	// 0. Auth: configure token validator
	if *prodMode {
		if *jwtSecret == "" && *jwtPublicKey == "" {
			log.Fatal("❌ --jwt-secret or --jwt-public-key is required in --prod mode")
		}
		if *jwtPublicKey != "" {
			// Read PEM file and parse public key (RSA or ECDSA)
			pemData, err := os.ReadFile(*jwtPublicKey)
			if err != nil {
				log.Fatalf("❌ reading public key file: %v", err)
			}
			var key any
			key, err = jwt.ParseECPublicKeyFromPEM(pemData)
			if err != nil {
				key, err = jwt.ParseRSAPublicKeyFromPEM(pemData)
				if err != nil {
					log.Fatalf("❌ parsing public key (tried ECDSA and RSA): %v", err)
				}
			}
			api.SetAuthValidator(auth.NewJWTValidatorWithKey(key, *jwtIssuer, ""))
			fmt.Printf("🔐 Auth: JWT (prod mode, asymmetric key from %s, issuer=%s)\n", *jwtPublicKey, *jwtIssuer)
		} else {
			api.SetAuthValidator(auth.NewJWTValidator(*jwtSecret, *jwtIssuer, ""))
			fmt.Printf("🔐 Auth: JWT (prod mode, HMAC, issuer=%s)\n", *jwtIssuer)
		}
	} else {
		api.SetAuthValidator(auth.NewDevValidator())
		fmt.Println("🔓 Auth: dev mode (all requests pass through)")
	}

	// Set strict mode
	if *strictMode || *prodMode {
		api.SetStrictMode(true)
		fmt.Println("🔒 Enforcement: strict")
	} else {
		fmt.Println("🔓 Enforcement: relaxed (dev)")
	}

	// 1. Open database
	database, err := db.Open(*dsn)
	if err != nil {
		log.Fatalf("❌ Open database: %v", err)
	}
	defer database.Close()

	driver := db.DriverSQLite
	if database.DriverName() == "postgres" {
		driver = db.DriverPostgres
	}
	fmt.Printf("✓ Database: %s (%s)\n", *dsn, driver)
	fmt.Printf("✓ Idempotency TTL: %v (config key: core.idempotency_retention)\n", *idempotencyTTL)

	// 2. Registry → load → sync
	reg := entity.NewRegistry(database, driver, *specPath)

	errs := reg.LoadEntities()
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "  ⚠ %v\n", e)
	}
	fmt.Printf("✓ Entities loaded: %d\n", reg.Count())

	// Wire permission registry into auth package (for ctx.auth.has())
	permReg := reg.GetPermissionRegistry()
	auth.SetPermissionChecker(permission.NewAuthChecker(permReg))

	// Print module permission footprints
	footprints := permReg.AllFootprints()
	if len(footprints) > 0 {
		fmt.Println("\n📋 Module Permission Footprints:")
		for _, fp := range footprints {
			fmt.Println(fp.String())
		}
		fmt.Printf("✓ Total permissions registered: %d\n", permReg.TotalPermissions())
	} else {
		fmt.Println("📋 No permission footprints (entities without expose)")
	}

	applied, err := reg.SyncSchema(ctx)
	if err != nil {
		log.Fatalf("❌ SyncSchema: %v", err)
	}
	if applied > 0 {
		fmt.Printf("✓ Migrations applied: %d\n", applied)
	} else {
		fmt.Println("✓ Schema: up to date")
	}

	// 3. Build router
	rb := api.NewRouterBuilder(reg)
	rb.BuildRoutes()

	// 3a. Wire action dispatcher with executors
	disp := action.NewDispatcher()

	// Script executor (script and script_ref)
	scriptEx := action.NewScriptExecutor(*specPath)
	scriptEx.SetSaveHandler(func(module, entity, id string, data map[string]any) error {
		store, err := reg.GetEntityStore(module, entity)
		if err != nil {
			return fmt.Errorf("get store: %w", err)
		}
		_, err = store.Update(ctx, db.UpdateParams{
			TenantID:  "demo",
			ID:        id,
			Version:   0, // TODO: fetch current version for CAS
			UpdatedBy: "script",
			Data:      data,
		})
		return err
	})
	scriptEx.SetCallHandler(func(fromModule, targetModule, targetEntity, actionName string, p map[string]any) (any, error) {
		if targetModule == "" {
			targetModule = fromModule
		}
		// TODO: dispatch to action dispatcher for cross-resource calls
		return map[string]any{"status": "called", "target": fmt.Sprintf("%s.%s.%s", targetModule, targetEntity, actionName)}, nil
	})
	scriptEx.SetLoadHandler(func(module, entity, id string) (map[string]any, error) {
		store, err := reg.GetEntityStore(module, entity)
		if err != nil {
			return nil, fmt.Errorf("get store: %w", err)
		}
		rec, err := store.GetByID(ctx, db.GetByIDParams{TenantID: "demo", ID: id})
		if err != nil {
			return nil, err
		}
		if rec == nil {
			return nil, fmt.Errorf("record not found")
		}
		return rec.Data, nil
	})
	scriptEx.SetNextKeyHandler(func(fieldName string) (string, error) {
		// Delegate to the counter store — the script doesn't know which entity,
		// so we generate a generic key for dev mode.
		return fmt.Sprintf("KEY-%d", time.Now().UnixNano()), nil
	})
	disp.RegisterExecutor(spec.ImplScript, scriptEx)
	disp.RegisterExecutor(spec.ImplScriptRef, scriptEx)

	// Native executor
	nativeEx := action.NewNativeExecutor()
	disp.RegisterExecutor(spec.ImplNative, nativeEx)

	// Sidecar executor (stub)
	disp.RegisterExecutor(spec.ImplSidecar, action.NewSidecarExecutor())

	rb.SetDispatcher(disp)

	// 3b. Wire exists:<resource> validator
	validation.SetEntityLookup(func(module, entityName, id string) (bool, error) {
		store, err := reg.GetEntityStore(module, entityName)
		if err != nil {
			return false, err
		}
		_, err = store.GetByID(ctx, db.GetByIDParams{TenantID: "demo", ID: id})
		if err != nil {
			return false, nil // not found = exists check fails
		}
		return true, nil
	})

	fmt.Printf("✓ Routes generated: %d\n", rb.RouteCount())

	for _, rd := range rb.Routes() {
		fmt.Printf("  %s %s/%s/%s\n", rd.Method, rd.Module, rd.Plural, rd.Action)
	}

	handler := rb.BuildHTTP()

	fmt.Printf("\n✓ Server starting on http://localhost%s\n", *addr)
	fmt.Println("  Endpoints:")
	fmt.Println("    GET  /health")
	for _, rd := range rb.Routes() {
		fmt.Printf("    %s   /{workspace}/api/v1/%s/%s%s\n", rd.Method, rd.Module, rd.Plural, rd.PathSuffix())
	}
	fmt.Println("\n  Try:")
	fmt.Println("    curl http://localhost" + *addr + "/demo/api/v1/billing/customers")
	fmt.Println("    curl http://localhost" + *addr + "/demo/api/v1/billing/addresses")
	fmt.Println("    curl http://localhost" + *addr + "/health")

	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatalf("❌ Server error: %v", err)
	}
}
