"""lib-forma-python — thin client SDK for forma-sidecar.

Runs a local /invoke listener (sidecar -> app) and exposes a ``ctx`` object
whose methods call back into the sidecar's /ctx endpoints (app -> sidecar).
No Forma business logic lives here — see docs/runtimes/04-forma-sidecar.md §4.4
and sdk/README.md for the wire contract.
"""

from .app import ActionResult, App, Invocation
from .ctx import Ctx, CtxPrimitive, FormaError

__all__ = [
    "ActionResult",
    "App",
    "Ctx",
    "CtxPrimitive",
    "FormaError",
    "Invocation",
]
