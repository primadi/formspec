# FormSpec Module Registry — production image (Plan C batch 2).
# Build:  docker build -t formspec-registry:latest -f registry/deploy/Dockerfile .
# Run:    see registry/deploy/k8s/
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# SPA renderer is optional at build time; without it the registry serves API
# only (portal/admin UI can be served separately or via --web-dir mount).
RUN CGO_ENABLED=0 go build -o /out/formspec-registry ./cmd/formspec-registry

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/formspec-registry /formspec-registry
# Spec is embedded in the binary (registry/embed.go) — no spec mount needed.
EXPOSE 8080 9102
ENTRYPOINT ["/formspec-registry"]
# Default flags; override via container args (see deployment.yaml):
#   --dsn postgres://... --prod --jwt-public-key /etc/formspec/jwt.pub
CMD ["--dsn", "postgres://formspec@postgres:5432/formspec_registry?sslmode=require", "--prod", "--addr", ":8080"]
