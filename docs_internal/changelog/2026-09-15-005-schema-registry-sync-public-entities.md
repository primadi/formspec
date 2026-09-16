# 2026-09-15-005 schema registry sync for public entities

`formspec validate` against the default registry was rejecting `App.spec.public_entities`, while the local override (`--schema schemas`) passed. The mismatch was caused by stale generated JSON Schema in `schemas/` relative to the current Go contract. I regenerated the schema bundle from `pkg/spec` and re-ran the Kafe example validation to confirm the default registry-backed path is now aligned.

This update affects the generated schema outputs under `schemas/` and the public QR app manifest at `examples/kafe/spec/apps/kafe-qr.yaml`. It closes the validation drift without changing runtime logic.
