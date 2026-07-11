# lib-forma-java

Thin Java client for `forma-sidecar` (docs/runtimes/04-forma-sidecar.md).
Java 17+, stdlib only — no dependencies beyond the JDK.

```bash
# Maven
<dependency>
    <groupId>io.forma</groupId>
    <artifactId>lib-forma</artifactId>
    <version>0.1.0</version>
</dependency>
```

```java
import io.forma.*;

var app = new App();  // sockets from FORMA_APP_SOCKET / FORMA_SIDECAR_SOCKET

app.handle("billing.invoice.approve", (Invocation inv, Ctx ctx) -> {
    var rows = ctx.db().query("SELECT ...");                 // proxied to the sidecar engine
    ctx.cache().named("session-cache").get("key");           // named datastore

    return new ActionResult(Map.of("approved_at", Instant.now().toString()))
            .withNewState("approved")
            .withEvent("invoice.approved", Map.of("id", inv.resourceId()));
});

app.run();  // blocks; sidecar calls POST /invoke/billing/invoice/approve
```

Handlers may return an `ActionResult`, a plain `Map`/`List` (becomes `data`),
or throw — exceptions surface to the sidecar as HTTP 500 with the message.

See [examples/App.java](examples/App.java) for a runnable app, and
[../README.md](../README.md) for the wire contract shared by all SDKs.
