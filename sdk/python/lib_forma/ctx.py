"""App -> Sidecar direction: the ctx.* primitive client.

Every method is an HTTP call to forma-sidecar's /ctx/{primitive}/{operation}
endpoint — the same primitive contract Starlark scripts use. Stdlib only.
"""

from __future__ import annotations

import http.client
import json
import os
import socket
from typing import Any, Optional


class FormaError(RuntimeError):
    """Transport failure or sidecar-reported error."""


class _UnixHTTPConnection(http.client.HTTPConnection):
    """HTTPConnection that dials a unix domain socket."""

    def __init__(self, socket_path: str, timeout: float) -> None:
        super().__init__("forma-sidecar", timeout=timeout)  # host is ignored
        self._socket_path = socket_path

    def connect(self) -> None:
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.settimeout(self.timeout)
        sock.connect(self._socket_path)
        self.sock = sock


class SidecarClient:
    """HTTP client to the sidecar listener (unix:// or http://)."""

    def __init__(self, endpoint: Optional[str] = None, timeout: float = 30.0) -> None:
        if endpoint is None:
            endpoint = "unix://" + os.environ.get(
                "FORMA_SIDECAR_SOCKET", "/tmp/forma/sidecar.sock"
            )
        self._timeout = timeout
        if endpoint.startswith("unix://"):
            self._socket_path: Optional[str] = endpoint[len("unix://"):]
            self._netloc = ""
        elif endpoint.startswith("http://"):
            self._socket_path = None
            self._netloc = endpoint[len("http://"):].rstrip("/")
        else:
            raise FormaError(
                f"sidecar endpoint {endpoint}: unsupported scheme (want unix:// or http://)"
            )

    def post(self, path: str, body: dict) -> dict:
        if self._socket_path is not None:
            conn: http.client.HTTPConnection = _UnixHTTPConnection(
                self._socket_path, self._timeout
            )
        else:
            conn = http.client.HTTPConnection(self._netloc, timeout=self._timeout)

        try:
            conn.request(
                "POST",
                path,
                body=json.dumps(body),
                headers={"Content-Type": "application/json"},
            )
            resp = conn.getresponse()
            raw = resp.read()
        except OSError as exc:
            raise FormaError(f"sidecar call {path}: {exc}") from exc
        finally:
            conn.close()

        try:
            decoded = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            decoded = {}
        if resp.status != 200:
            raise FormaError(
                f"sidecar call {path}: {decoded.get('error', f'HTTP {resp.status}')}"
            )
        return decoded


class CtxPrimitive:
    """One primitive handle (db/cache/lock/...); .named() binds a datastore."""

    def __init__(self, client: SidecarClient, prim_type: str, named: str = "") -> None:
        self._client = client
        self._type = prim_type
        self._named = named

    def named(self, name: str) -> "CtxPrimitive":
        return CtxPrimitive(self._client, self._type, name)

    def query(self, sql: str, args: Optional[list] = None) -> list:
        body: dict = {"sql": sql}
        if args:
            body["args"] = args
        return self._call("query", body).get("data") or []

    def get(self, key: str) -> Any:
        return self._call("get", {"key": key}).get("data")

    def set(self, key: str, value: Any, ttl_seconds: int = 0) -> None:
        body: dict = {"key": key, "value": value}
        if ttl_seconds > 0:
            body["ttl_seconds"] = ttl_seconds
        self._call("set", body)

    def delete(self, key: str) -> None:
        self._call("delete", {"key": key})

    # ---- entity atomic operations ----

    def update(self, id: str, fields: dict) -> None:
        """Atomically merge fields into an entity record (entity/update)."""
        body: dict = {"key": id, "fields": fields}
        self._call("update", body)

    def increment(self, id: str, field: str, amount: float) -> None:
        """Atomically increment a numeric field on an entity record."""
        body: dict = {"key": id, "field": field, "amount": amount}
        self._call("increment", body)

    def decrement(self, id: str, field: str, amount: float) -> Any:
        """Atomically decrement a numeric field on an entity record.
        Includes a guard against negative values. Returns the new field value."""
        body: dict = {"key": id, "field": field, "amount": amount}
        return self._call("decrement", body).get("data")

    def acquire(self, key: str, ttl_seconds: int = 30) -> bool:
        return bool(
            self._call("acquire", {"key": key, "ttl_seconds": ttl_seconds}).get("ok")
        )

    def release(self, key: str) -> None:
        self._call("release", {"key": key})

    def _call(self, op: str, body: dict) -> dict:
        if self._named:
            body["named"] = self._named
        return self._client.post(f"/ctx/{self._type}/{op}", body)


class Ctx:
    """The ctx.* surface handed to handlers.

    Usage::

        rows = ctx.db().query("SELECT ...")
        ctx.db().named("analytics-db").query("SELECT ...")
        ctx.lock().acquire("workspace:X", ttl_seconds=30)
    """

    def __init__(self, client: SidecarClient) -> None:
        self._client = client

    def db(self) -> CtxPrimitive:
        return CtxPrimitive(self._client, "db")

    def cache(self) -> CtxPrimitive:
        return CtxPrimitive(self._client, "cache")

    def lock(self) -> CtxPrimitive:
        return CtxPrimitive(self._client, "lock")

    def queue(self) -> CtxPrimitive:
        return CtxPrimitive(self._client, "queue")

    def pubsub(self) -> CtxPrimitive:
        return CtxPrimitive(self._client, "pubsub")

    def storage(self) -> CtxPrimitive:
        return CtxPrimitive(self._client, "storage")

    def kvstore(self) -> CtxPrimitive:
        return CtxPrimitive(self._client, "kvstore")

    def entity(self) -> CtxPrimitive:
        """Entity primitive — access entity records via named('module/entity')."""
        return CtxPrimitive(self._client, "entity")
