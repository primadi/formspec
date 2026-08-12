# lib-formspec-rust

Thin Rust client for `formspec-sidecar` (docs/runtimes/04-formspec-sidecar.md).
Rust 1.75+, dependencies: `ureq` (HTTP + Unix socket), `serde`/`serde_json`.

```toml
[dependencies]
lib-formspec = { path = "/path/to/sdk/rust" }
```

```rust
use lib_formspec::{App, ActionResult};

let mut app = App::new(); // sockets from FORMA_APP_SOCKET / FORMA_SIDECAR_SOCKET

app.handle("billing.invoice.approve", |inv, ctx| {
    // ctx.db().query("SELECT ...");         // proxied to the sidecar engine
    // ctx.cache().named("session-cache").get("key");

    ActionResult {
        data: serde_json::json!({ "approved_at": chrono::Utc::now().to_rfc3339() }),
        new_state: Some("approved".to_string()),
    }
});

app.run(); // blocks; sidecar calls POST /invoke/billing/invoice/approve
```

See [examples/app.rs](examples/app.rs) for a runnable app, and
[../README.md](../README.md) for the wire contract shared by all SDKs.
