.PHONY: all build clean test lint run-example dev web-deps web-dev web-build

# Build all binaries
all: build

build: build-resource build-control

build-resource:
	go build -o bin/forma-resource ./cmd/forma-resource

build-control:
	go build -o bin/forma-control ./cmd/forma-control

# Run tests
test:
	go test ./...

test-verbose:
	go test -v ./...

# Lint
lint:
	golangci-lint run ./...

# Run dev server (both planes)
dev:
	go run ./cmd/forma-resource --dev

# Run specific example
run-example:
	@echo "Usage: make run-example EXAMPLE=customer"
	go run ./cmd/forma-resource --spec examples/$(EXAMPLE)/spec

# Clean build artifacts
clean:
	rm -rf bin/

# Generate TypeScript types from spec
generate:
	go run ./cmd/forma-resource generate --output web/src/generated

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
