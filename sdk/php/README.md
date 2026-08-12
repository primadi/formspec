# lib-formspec-php

Thin PHP client for `formspec-sidecar` (docs/runtimes/04-formspec-sidecar.md).
Pure PHP ≥ 8.1, no dependencies beyond ext-curl/ext-json.

```bash
composer require formspec/lib-formspec
```

```php
use FormSpec\{App, ActionResult, Ctx, Invocation};

$app = new App(); // sockets from FORMA_APP_SOCKET / FORMA_SIDECAR_SOCKET

$app->handle('billing.invoice.approve', function (Invocation $inv, Ctx $ctx) {
    $rows = $ctx->db()->query('SELECT ...');            // proxied to the sidecar engine
    $ctx->cache()->named('session-cache')->get('key');  // named datastore

    return (new ActionResult(['approved_at' => date(DATE_ATOM)], newState: 'approved'))
        ->withEvent('invoice.approved', ['id' => $inv->resourceId]);
});

$app->run(); // blocks; sidecar calls POST /invoke/billing/invoice/approve
```

Handlers may return an `ActionResult`, a plain array (becomes `data`), or
throw — exceptions surface to the sidecar as HTTP 500 with the message.

See [examples/app.php](examples/app.php) for a runnable app, and
[../README.md](../README.md) for the wire contract shared by all SDKs.
