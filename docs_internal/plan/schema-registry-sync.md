# Schema registry sync plan

## Context

`formspec validate` against the default registry fails on `App.spec.public_entities`, while the local schema override (`--schema schemas`) succeeds. The root cause is schema drift: generated files under `schemas/` are stale relative to the current Go contract in `pkg/spec`.

## Scope

- Rebuild generated JSON Schema from `pkg/spec`.
- Verify default validator behavior with the cached registry-backed path.
- Confirm the Kafe public app validates without requiring the local override.

## Files involved

- `pkg/spec/resources.go`
- `cmd/formspec-gen-schema/main.go`
- `schemas/**`
- `examples/kafe/spec/apps/kafe-qr.yaml`

## Effort

Small. This is a deterministic regeneration and verification pass.

## Validation

- `make generate-schema`
- `./formspec validate --spec examples/kafe/spec`
