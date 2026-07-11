import io.forma.*;

import java.time.Instant;
import java.util.Map;

/**
 * Example lib-forma-java app: the business-logic side of an
 * {@code impl: {type: sidecar}} action. Run inside a pod next to forma-sidecar:
 *
 * <pre>{@code
 * FORMA_APP_SOCKET=http://localhost:9801 \
 * FORMA_SIDECAR_SOCKET=unix:///var/run/forma/sidecar.sock \
 * mvn exec:java -Dexec.mainClass=AppExample
 * }</pre>
 */
public class AppExample {

    public static void main(String[] args) {
        try (var app = new App()) {

        app.handle("billing.invoice.approve", (Invocation inv, Ctx ctx) -> {
            var lockKey = "invoice:" + inv.resourceId();
            if (!ctx.lock().acquire(lockKey, 30)) {
                throw new RuntimeException("invoice is being processed by someone else");
            }

            try {
                var status = inv.resource().get("status");
                if (!"draft".equals(status)) {
                    throw new RuntimeException("only draft invoices can be approved");
                }

                return new ActionResult(
                        Map.of(
                                "approved_at", Instant.now().toString(),
                                "note", inv.params().getOrDefault("note", "")),
                        "approved")
                        .withEvent("invoice.approved",
                                Map.of("id", inv.resourceId()), true);
            } finally {
                ctx.lock().release(lockKey);
            }
        });

        app.run();
        } catch (Exception e) {
            System.err.println("Error: " + e.getMessage());
        }
    }
}
