/// Example lib-forma-rust app: the business-logic side of an
/// `impl: {type: sidecar}` action. Run next to forma engine:
///
/// ```text
/// FORMA_APP_SOCKET=unix:///tmp/forma-app.sock \
/// FORMA_SIDECAR_SOCKET=unix:///tmp/forma-sidecar.sock \
/// cargo run --example app
/// ```
use lib_forma::{App, ActionResult};

fn main() {
    let mut app = App::new();

    app.handle("billing.invoice.approve", |inv, _ctx| {
        eprintln!("[billing.invoice.approve] invoked: resource_id={}", inv.resource_id);

        ActionResult {
            data: serde_json::json!({
                "approved_at": chrono::Utc::now().to_rfc3339(),
            }),
            new_state: Some("approved".to_string()),
        }
    });

    app.run();
}
