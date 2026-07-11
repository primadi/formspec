"""Sidecar -> App direction: the /invoke listener.

Receives POST /invoke/{module}/{entity}/{action} from forma-sidecar and
dispatches to registered handler functions. Also answers GET /health for
the sidecar's app monitor. Stdlib only.
"""

from __future__ import annotations

import json
import os
import socketserver
import sys
import urllib.parse
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler
from typing import Any, Callable, Dict, List, Optional, Union

from .ctx import Ctx, FormaError, SidecarClient


@dataclass(frozen=True)
class Invocation:
    """One action invocation — the wire form of the engine's ExecuteParams."""

    module: str
    entity: str
    action: str
    resource_id: str
    resource: dict
    params: dict
    tenant_id: str
    user_id: str


@dataclass
class ActionResult:
    """Structured handler result — the wire form of ExecuteResult.

    Handlers may also return a plain dict/list/scalar, which becomes ``data``.
    """

    data: Any = None
    new_state: Optional[str] = None
    events: List[dict] = field(default_factory=list)

    def with_event(self, name: str, payload: Optional[dict] = None, durable: bool = False) -> "ActionResult":
        event: dict = {"name": name}
        if payload:
            event["payload"] = payload
        if durable:
            event["durable"] = True
        self.events.append(event)
        return self

    def to_wire(self) -> dict:
        wire: dict = {"data": self.data}
        if self.new_state is not None:
            wire["new_state"] = self.new_state
        if self.events:
            wire["events"] = self.events
        return wire


Handler = Callable[[Invocation, Ctx], Union[ActionResult, dict, list, None]]


class _UnixHTTPServer(socketserver.ThreadingUnixStreamServer):
    daemon_threads = True

    def get_request(self):
        # BaseHTTPRequestHandler expects a (host, port) client address; a
        # unix socket peer has none.
        request, _ = super().get_request()
        return request, ("local", 0)

    def handle_error(self, request, client_address):
        # A client dropping a keep-alive connection mid-request is normal
        # lifecycle, not a server error worth a traceback.
        exc = sys.exc_info()[1]
        if isinstance(exc, (BrokenPipeError, ConnectionResetError)):
            return
        super().handle_error(request, client_address)


class App:
    """The lib-forma-python listener.

    Usage::

        app = lib_forma.App()  # sockets from FORMA_APP_SOCKET / FORMA_SIDECAR_SOCKET

        @app.action("billing.invoice.approve")
        def approve(inv: Invocation, ctx: Ctx):
            ctx.lock().acquire(f"invoice:{inv.resource_id}")
            ...
            return ActionResult({"ok": True}, new_state="approved")

        app.run()
    """

    def __init__(self, listen: Optional[str] = None, sidecar_endpoint: Optional[str] = None) -> None:
        self._listen = listen or "unix://" + os.environ.get(
            "FORMA_APP_SOCKET", "/var/run/forma/app.sock"
        )
        self._handlers: Dict[str, Handler] = {}
        self.ctx = Ctx(SidecarClient(sidecar_endpoint))

    def action(self, name: str) -> Callable[[Handler], Handler]:
        """Decorator registering a handler for "module.entity.action"."""

        def register(handler: Handler) -> Handler:
            self.handle(name, handler)
            return handler

        return register

    def handle(self, name: str, handler: Handler) -> None:
        if name in self._handlers:
            raise FormaError(f"handler for {name} already registered")
        self._handlers[name] = handler

    def run(self) -> None:
        """Blocks serving requests until the process is terminated."""
        if not self._listen.startswith("unix://"):
            raise FormaError(
                f"listen {self._listen}: only unix:// is supported by lib-forma-python"
            )
        socket_path = self._listen[len("unix://"):]

        os.makedirs(os.path.dirname(socket_path), exist_ok=True)
        if os.path.exists(socket_path):
            os.unlink(socket_path)  # stale socket from a previous run

        app = self

        class RequestHandler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, fmt: str, *args: Any) -> None:
                pass  # invoke traffic is high-volume; the sidecar logs errors

            def do_GET(self) -> None:  # noqa: N802 (http.server API)
                if urllib.parse.urlparse(self.path).path == "/health":
                    self._respond(200, {"status": "healthy", "handlers": len(app._handlers)})
                else:
                    self._respond(404, {"error": "expected GET /health"})

            def do_POST(self) -> None:  # noqa: N802
                path = urllib.parse.urlparse(self.path).path
                parts = [urllib.parse.unquote(p) for p in path.strip("/").split("/")]
                if len(parts) != 4 or parts[0] != "invoke":
                    self._respond(404, {"error": "expected POST /invoke/{module}/{entity}/{action}"})
                    return
                _, module, entity, action = parts

                key = f"{module}.{entity}.{action}"
                handler = app._handlers.get(key)
                if handler is None:
                    self._respond(500, {"error": f"no handler registered for {key}"})
                    return

                length = int(self.headers.get("Content-Length", "0"))
                try:
                    body = json.loads(self.rfile.read(length)) if length else {}
                except json.JSONDecodeError as exc:
                    self._respond(400, {"error": f"invalid JSON body: {exc}"})
                    return

                invocation = Invocation(
                    module=module,
                    entity=entity,
                    action=action,
                    resource_id=str(body.get("resource_id", "")),
                    resource=body.get("resource") or {},
                    params=body.get("params") or {},
                    tenant_id=str(body.get("tenant_id", "")),
                    user_id=str(body.get("user_id", "")),
                )

                try:
                    result = handler(invocation, app.ctx)
                except Exception as exc:  # handler errors surface as HTTP 500
                    self._respond(500, {"error": str(exc)})
                    return

                if isinstance(result, ActionResult):
                    self._respond(200, result.to_wire())
                else:
                    self._respond(200, {"data": result})

            def _respond(self, status: int, payload: dict) -> None:
                raw = json.dumps(payload).encode()
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

        server = _UnixHTTPServer(socket_path, RequestHandler)
        os.chmod(socket_path, 0o666)  # sidecar runs as a different user
        print(f"[lib-forma-python] listening on {socket_path}", file=sys.stderr)
        try:
            server.serve_forever()
        finally:
            server.server_close()
