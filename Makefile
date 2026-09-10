.PHONY: all build clean test lint run-example registry-dev dev web-deps web-dev web-build web-typecheck apply build-spa build-registry install release release-upload

# Build all binaries
all: build

build: build-spa build-ctl build-formspec build-registry build-operator

build-ctl:
	go build -o bin/formspec-ctl ./cmd/formspec-ctl

# Build the formspec CLI with embedded SPA.
# build-spa builds renderers/react-shadcn/dist/, then we copy it to cmd/formspec/dist/ for go:embed.
build-formspec: build-spa
	@mkdir -p cmd/formspec/dist
	cp -r renderers/react-shadcn/dist/* cmd/formspec/dist/
	go build -o bin/formspec ./cmd/formspec

# Build the registry binary with embedded SPA.
# Syncs renderers/react-shadcn/dist → cmd/formspec-registry/web/dist for
# go:embed, so formspec-registry serves the admin panel/portal without
# --web-dir.
build-registry: build-spa
	@mkdir -p cmd/formspec-registry/web/dist
	cp -r renderers/react-shadcn/dist/* cmd/formspec-registry/web/dist/
	go build -o bin/formspec-registry ./cmd/formspec-registry

build-sidecar:
	@echo "✅ Build complete: bin/formspec"

build-operator:
	go build -o bin/formspec-operator ./cmd/formspec-operator

# Build the frontend SPA. Requires npm dependencies installed (make web-deps).
build-spa: web-deps
	cd renderers/react-shadcn && npm run build

# formspec-resource is a Go library (import "github.com/formspec/formspec"), not a
# binary — see docs/runtimes/02-formspec-resource.md. examples/reference-app
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
	@echo "🔧 FormSpec Dev Environment"
	@echo "========================"
	@echo ""
	@echo "Starting Control Plane on :8443 ..."
	go run ./cmd/formspec-ctl serve --dev &
	@echo "Starting reference app on :8080 ..."
	@sleep 1
	go run ./examples/reference-app --spec verticals/billing/spec &
	@echo ""
	@echo "📝 To register specs, run:"
	@echo "   go run ./cmd/formspec apply --control http://localhost:8443 verticals/billing/spec"
	@echo ""
	@echo "🌐 Reference app:  http://localhost:8080"
	@echo "🛂 Control Plane:  http://localhost:8443"
	@echo ""
	@echo "Press Ctrl+C to stop all processes"
	wait

# Run the Module Registry in dev mode (config: registry/formspec-app.yaml)
registry-dev:
	scripts/run-registry.sh

# Run specific example (legacy — use formspec apply instead)
run-example:
	@echo "⚠️  Deprecated: use 'formspec apply' to register specs to Control Plane"
	@echo "   Example: go run ./cmd/formspec apply --control http://localhost:8443 verticals/billing/spec"

# Clean build artifacts
clean:
	rm -rf bin/ .formspec/

# Generate TypeScript types from spec (not implemented yet — see docs/cli-tools/01-formspec-cli.md §5)
generate:
	@echo "⏳ 'formspec generate' is not implemented yet — see docs/cli-tools/01-formspec-cli.md §5"

# Generate JSON Schema files from Go struct types in pkg/spec/
# Output goes to schemas/ — register in VS Code via yaml.schemas
generate-schema:
	@echo "📐 Generating JSON Schema from Go types..."
	@go run ./cmd/formspec-gen-schema/ --out schemas
	@echo "✅ JSON Schema files generated in schemas/"

# Stage schema versi ke schemas/dist/ untuk schemas.formspec.dev.
# Jalur deploy: git-based — commit schemas/dist lalu push (Cloudflare
# auto-build). Default: stage v1 lokal. Upload R2 (cadangan): tambah --upload.
#   make publish-schemas                         # stage v1
#   make publish-schemas ARGS="--version v1 --upload --bucket formspec-schemas"
publish-schemas:
	@test -f scripts/publish-schemas.sh || (echo "❌ scripts/publish-schemas.sh tidak ditemukan" && exit 1)
	@./scripts/publish-schemas.sh $(ARGS)

# Generate per-kind reference docs from Go struct types in pkg/spec/
# Output goes to docs/kind/<group>/<Kind>.md — one file per kind, split in 4
# groups (curation/data/ui/infra). Attribute tables are generated (zero drift);
# narrative sections between <!-- generated:... --> markers are preserved.
generate-kind-docs:
	@echo "📄 Generating kind reference docs from Go types..."
	@go run ./cmd/formspec-gen-kind-docs/ --out docs/kind
	@echo "✅ Kind reference docs generated in docs/kind/"

# Validate a spec tree against the engine loader AND the JSON Schema contract.
# Usage: make validate-spec SPEC=examples/crc-management/spec
validate-spec:
	@test -n "$(SPEC)" || (echo "Usage: make validate-spec SPEC=<path to spec dir>" && exit 1)
	@go run ./cmd/formspec validate --spec $(SPEC)

# Download dependencies
deps:
	go mod tidy
	go mod download

# ---------------------------------------------------------------------------
# Release — cross-compile prebuilt binaries untuk semua OS/arch.
#
# SPA dibangun SEKALI (identik untuk semua target), lalu binary dicompile untuk
# {linux,darwin,windows} × {amd64,arm64} dengan CGO_ENABLED=0 (SQLite pakai
# driver pure-Go modernc.org/sqlite).
#
#   make release                        # VERSION otomatis dari git describe
#   make release VERSION=v1.2.3         # override manual
#   make release-upload VERSION=v1.2.3  # upload dist/release/* ke GitHub Releases (draft)
#
# Output: dist/release/formspec-<os>-<arch>.tar.gz|.zip + SHA256SUMS.txt
# Prosedur lengkap (prasyarat, tag, upload, verifikasi): docs/guides/releasing.md
# ---------------------------------------------------------------------------

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
RELEASE_DIR := dist/release

# Pairs "<os>/<arch>:<ext>" — ext = packaging (tar.gz | zip)
RELEASE_TARGETS := \
	linux/amd64:tar.gz \
	linux/arm64:tar.gz \
	darwin/amd64:tar.gz \
	darwin/arm64:tar.gz \
	windows/amd64:zip \
	windows/arm64:zip

release: build-spa
	@mkdir -p cmd/formspec/dist
	cp -r renderers/react-shadcn/dist/* cmd/formspec/dist/
	@rm -rf $(RELEASE_DIR)
	@mkdir -p $(RELEASE_DIR)
	@echo "🚀 Building formspec $(VERSION) untuk semua platform..."
	@for t in $(RELEASE_TARGETS); do \
		os=$${t%%/*}; rest=$${t#*/}; arch=$${rest%%:*}; ext=$${rest#*:}; \
		echo "  → $$os/$$arch"; \
		out="$(RELEASE_DIR)/formspec-$$os-$$arch"; mkdir -p "$$out"; \
		name=formspec; if [ "$$os" = "windows" ]; then name="formspec.exe"; fi; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath \
			-ldflags "-s -w -X main.version=$(VERSION)" \
			-o "$$out/$$name" ./cmd/formspec; \
		if [ "$$ext" = "tar.gz" ]; then \
			tar -czf "$(RELEASE_DIR)/formspec-$$os-$$arch.tar.gz" -C "$$out" $$name; \
		else \
			zip -q -j "$(RELEASE_DIR)/formspec-$$os-$$arch.zip" "$$out/$$name"; \
		fi; \
		rm -rf "$$out"; \
	done
	@cd $(RELEASE_DIR) && shasum -a 256 *.tar.gz *.zip > SHA256SUMS.txt
	@echo "✅ Release artifacts siap di $(RELEASE_DIR)/"
	@ls -lh $(RELEASE_DIR)

release-upload:
	@test -n "$(VERSION)" || (echo "❌ Set VERSION, mis. make release-upload VERSION=v0.1.0 (jangan mengandalkan git describe — lihat docs/guides/releasing.md)" && exit 1)
	@printf '%s' "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+' || (echo "❌ VERSION='$(VERSION)' bukan semver (format: v<major>.<minor>.patch>). Rilis pertama juga harus tag semver yang dipilih sadar." && exit 1)
	@git rev-parse "$(VERSION)^{commit}" >/dev/null 2>&1 || (echo "❌ Tag $(VERSION) belum ada lokal — buat dulu: git tag $(VERSION) && git push origin $(VERSION)" && exit 1)
	@if git ls-remote --tags origin | grep -q "refs/tags/$(VERSION)$$"; then \
		echo "❌ Tag $(VERSION) sudah ada di remote — satu tag = satu release. Pakai versi baru."; exit 1; \
	fi
	@command -v gh >/dev/null 2>&1 || (echo "❌ 'gh' CLI tidak ditemukan — upload manual via https://github.com/primadi/formspec/releases/new" && exit 1)
	@gh release create $(VERSION) $(RELEASE_DIR)/* --draft --generate-notes

# Frontend
web-deps:
	cd renderers/react-shadcn && npm install

web-dev:
	cd renderers/react-shadcn && npm run dev

# Typecheck frontend dengan tsconfig.app.json (config yang dipakai editor —
# `tsc --noEmit` tanpa -p memakai tsconfig.json solution-style dan tidak
# mengecek apa pun). Gate sebelum build agar type drift Go->TS tertangkap.
web-typecheck:
	cd renderers/react-shadcn && npx tsc --noEmit -p tsconfig.app.json

web-build: web-typecheck
	cd renderers/react-shadcn && npm run build

# Format code
fmt:
	go fmt ./...

# Docs server — tampilkan docs/ di browser (http://localhost:8000/docs/)
# Usage:
#   make docs-serve          # port 8000
#   make docs-serve PORT=9000
docs-serve:
	@test -f scripts/docs-serve.py || (echo "❌ scripts/docs-serve.py tidak ditemukan" && exit 1)
	@python3 -c "import markdown" 2>/dev/null || pip install markdown Pygments
	@python3 scripts/docs-serve.py --port $(if $(PORT),$(PORT),8000)

.PHONY: docs-serve
