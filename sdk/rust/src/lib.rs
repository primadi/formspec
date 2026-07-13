//! Thin Rust SDK for forma-sidecar communication.
//!
//! Provides an [App] that listens on `FORMA_APP_SOCKET` (a Unix socket)
//! for incoming invocations from the forma engine, and a [Ctx] proxy
//! that forwards `ctx.*` primitive calls back to the engine over
//! `FORMA_SIDECAR_SOCKET`.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// ─── Public Types ───

/// Represents an incoming action invocation from the forma engine.
#[derive(Debug, Deserialize)]
pub struct Invocation {
    pub resource_id: String,
    pub tenant_id: String,
    pub resource: serde_json::Value,
    pub params: serde_json::Value,
}

/// Result returned by a handler after processing an invocation.
#[derive(Debug, Serialize)]
pub struct ActionResult {
    pub data: serde_json::Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub new_state: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub events: Option<Vec<Event>>,
}

/// An event emitted by a handler.
#[derive(Debug, Serialize)]
pub struct Event {
    pub name: String,
    pub data: serde_json::Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub durable: Option<bool>,
}

/// Proxy for `ctx.*` primitives — forwards calls to the forma engine.
pub struct Ctx {
    sidecar_socket: String,
}

impl Ctx {
    fn new(sidecar_socket: &str) -> Self {
        Self {
            sidecar_socket: sidecar_socket.to_string(),
        }
    }

    /// Access the database primitive.
    pub fn db(&self) -> DbProxy {
        DbProxy::new(&self.sidecar_socket)
    }

    /// Access the cache primitive.
    pub fn cache(&self) -> CacheProxy {
        CacheProxy::new(&self.sidecar_socket)
    }

    /// Access the lock primitive.
    pub fn lock(&self) -> LockProxy {
        LockProxy::new(&self.sidecar_socket)
    }
}

// ─── Proxy types ───

pub struct DbProxy {
    sidecar_socket: String,
}

impl DbProxy {
    fn new(sidecar_socket: &str) -> Self {
        Self {
            sidecar_socket: sidecar_socket.to_string(),
        }
    }

    pub fn query(&self, _sql: &str) -> Result<Vec<HashMap<String, serde_json::Value>>, String> {
        // TODO: implement actual proxy call via sidecar socket
        Ok(Vec::new())
    }
}

pub struct CacheProxy {
    sidecar_socket: String,
}

impl CacheProxy {
    fn new(sidecar_socket: &str) -> Self {
        Self {
            sidecar_socket: sidecar_socket.to_string(),
        }
    }

    pub fn named(&self, _name: &str) -> CacheInstance {
        CacheInstance
    }
}

pub struct CacheInstance;

impl CacheInstance {
    pub fn get(&self, _key: &str) -> Result<Option<String>, String> {
        Ok(None)
    }
}

pub struct LockProxy {
    sidecar_socket: String,
}

impl LockProxy {
    fn new(sidecar_socket: &str) -> Self {
        Self {
            sidecar_socket: sidecar_socket.to_string(),
        }
    }

    pub fn acquire(&self, _name: &str, _ttl_seconds: u64) -> bool {
        true
    }

    pub fn release(&self, _name: &str) {}
}

// ─── App ───

type HandlerFn = Box<dyn FnMut(Invocation, Ctx) -> ActionResult + Send>;

/// The main application struct. Listens on `FORMA_APP_SOCKET` for incoming
/// invocations from the forma engine.
pub struct App {
    handlers: HashMap<String, HandlerFn>,
    app_socket: String,
    sidecar_socket: String,
}

impl App {
    /// Create a new App. Reads socket paths from environment variables:
    /// - `FORMA_APP_SOCKET` — where to listen for invocations (default: `unix:///tmp/forma-app.sock`)
    /// - `FORMA_SIDECAR_SOCKET` — where to forward ctx.* calls (default: `unix:///tmp/forma-sidecar.sock`)
    pub fn new() -> Self {
        let app_socket = std::env::var("FORMA_APP_SOCKET")
            .unwrap_or_else(|_| "unix:///tmp/forma-app.sock".to_string());
        let sidecar_socket = std::env::var("FORMA_SIDECAR_SOCKET")
            .unwrap_or_else(|_| "unix:///tmp/forma-sidecar.sock".to_string());
        Self {
            handlers: HashMap::new(),
            app_socket,
            sidecar_socket,
        }
    }

    /// Register a handler for the given action ID (format: `module.entity.action`).
    pub fn handle<F>(&mut self, action: &str, handler: F)
    where
        F: FnMut(Invocation, Ctx) -> ActionResult + Send + 'static,
    {
        self.handlers.insert(action.to_string(), Box::new(handler));
    }

    /// Start listening for invocations. Blocks the current thread.
    pub fn run(&mut self) {
        // Strip unix:// prefix to get the socket path
        let socket_path = self
            .app_socket
            .strip_prefix("unix://")
            .unwrap_or(&self.app_socket)
            .to_string();

        // Remove old socket file if it exists
        let _ = std::fs::remove_file(&socket_path);

        let listener = match std::os::unix::net::UnixListener::bind(&socket_path) {
            Ok(l) => l,
            Err(e) => {
                eprintln!("[lib-forma] failed to bind {}: {}", socket_path, e);
                return;
            }
        };

        eprintln!("[lib-forma] listening on {}", socket_path);

        for stream in listener.incoming() {
            match stream {
                Ok(mut stream) => {
                    use std::io::Read;
                    let mut buf = Vec::new();
                    if let Err(e) = stream.read_to_end(&mut buf) {
                        eprintln!("[lib-forma] read error: {}", e);
                        continue;
                    }
                    // Handle invocation (simplified — full HTTP parsing in production)
                    if let Ok(inv) = serde_json::from_slice::<Invocation>(&buf) {
                        let action_id = format!("{}.{}.{}", "", "", ""); // placeholder
                        if let Some(handler) = self.handlers.get_mut(&action_id) {
                            let ctx = Ctx::new(&self.sidecar_socket);
                            let result = handler(inv, ctx);
                            let resp = serde_json::to_string(&result).unwrap_or_default();
                            use std::io::Write;
                            let _ = stream.write_all(resp.as_bytes());
                        }
                    }
                }
                Err(e) => {
                    eprintln!("[lib-forma] accept error: {}", e);
                }
            }
        }
    }
}

impl Default for App {
    fn default() -> Self {
        Self::new()
    }
}
