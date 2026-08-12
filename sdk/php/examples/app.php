<?php

// Example lib-formspec-php app: the business-logic side of an
// impl: {type: sidecar} action. Run inside a pod next to formspec-sidecar:
//
//   FORMA_APP_SOCKET=/tmp/formspec/app.sock \
//   FORMA_SIDECAR_SOCKET=/tmp/formspec/sidecar.sock \
//   php examples/app.php

declare(strict_types=1);

require __DIR__ . '/../vendor/autoload.php';

use FormSpec\ActionResult;
use FormSpec\App;
use FormSpec\Ctx;
use FormSpec\Invocation;

$app = new App();

$app->handle('billing.invoice.approve', function (Invocation $inv, Ctx $ctx): ActionResult {
    if (!$ctx->lock()->acquire('invoice:' . $inv->resourceId, 30)) {
        throw new RuntimeException('invoice is being processed by someone else');
    }

    try {
        if (($inv->resource['status'] ?? '') !== 'draft') {
            throw new RuntimeException('only draft invoices can be approved');
        }

        return (new ActionResult(
            data: ['approved_at' => date(DATE_ATOM), 'note' => $inv->params['note'] ?? ''],
            newState: 'approved',
        ))->withEvent('invoice.approved', ['id' => $inv->resourceId], durable: true);
    } finally {
        $ctx->lock()->release('invoice:' . $inv->resourceId);
    }
});

$app->run();
