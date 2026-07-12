.PHONY: all build clean test lint run-example dev web-deps web-dev web-build apply build-spa

# Build all binaries
all: build

build: build-spa build-ctl build-forma build-operator

build-ctl:
	go build -o bin/forma-ctl ./cmd/forma-ctl

# Build the forma CLI with embedded SPA.
# build-spa ensures web/dist/ is built and copied to cmd/forma/dist/.
build-forma: build-spa
	go build -o bin/forma ./cmd/forma

build-sidecar:
	@echo "⚠️  cmd/forma-sidecar is deprecated — use 'forma dev' instead"
	go build -o bin/forma-sidecar ./cmd/forma-sidecar

build-operator:
	go build -o bin/forma-operator ./cmd/forma-operator

# Build the frontend SPA. Requires npm dependencies installed (make web-deps).
build-spa: web-deps
	cd web && npm run build
	@mkdir -p cmd/forma/dist
	cp -r web/dist/* cmd/forma/dist/

# forma-resource is a Go library (import "github.com/forma/forma"), not a
# binary — see docs/runtimes/02-forma-resource.md. examples/reference-app
# demonstrates embedding it.

# Run tests
test:
	go test ./...

test-verbose:
	go test -v ./...

# Lint
lint:
	golangci-lint run ./...

# Run dev environment — starts both planes + watcher
# Usage:
#   make dev              # start with default vertical (billing)
#   make dev EXAMPLE=gl   # start with the gl vertical instead
dev:
	@echo "🔧 Forma Dev Environment"
	@echo "========================"
	@echo ""
	@echo "Starting Control Plane on :8443 ..."
	go run ./cmd/forma-ctl serve --dev &
	@echo "Starting reference app on :8080 ..."
	@sleep 1
	go run ./examples/reference-app --spec verticals/billing/spec &
	@echo ""
	@echo "📝 To register specs, run:"
	@echo "   go run ./cmd/forma apply --control http://localhost:8443 verticals/billing/spec"
	@echo ""
	@echo "🌐 Reference app:  http://localhost:8080"
	@echo "🛂 Control Plane:  http://localhost:8443"
	@echo ""
	@echo "Press Ctrl+C to stop all processes"
	wait

# Run specific example (legacy — use forma apply instead)
run-example:
	@echo "⚠️  Deprecated: use 'forma apply' to register specs to Control Plane"
	@echo "   Example: go run ./cmd/forma apply --control http://localhost:8443 verticals/billing/spec"

# Clean build artifacts
clean:
	rm -rf bin/ .forma/

# Generate TypeScript types from spec (not implemented yet — see docs/cli-tools/01-forma-cli.md §5)
generate:
	@echo "⏳ 'forma generate' is not implemented yet — see docs/cli-tools/01-forma-cli.md §5"

# Download dependencies
deps:
	go mod tidy
	go mod download

# Frontend
web-deps:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

# Format code
fmt:
	go fmt ./...
