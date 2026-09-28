// ─── Service action call path ───
//
// Builds the API-client path for a `submit.call` (kafe P3).
//
// The `../` is load-bearing and easy to get wrong. The client is created with
// `prefix: "/{ws}/_ui/entity"`, so a relative path starting with `../` climbs
// out of the `entity` segment and lands on `/{ws}/_ui/service/...` — which is
// where the runtime registers service actions
// (`GenerateUIServiceRoutes`). Writing the path any other way (an absolute
// `/_ui/service/...`, say) would be resolved against the ORIGIN instead of the
// prefix and miss the workspace slug entirely.
//
// This is a named function rather than an inline template so the assumption is
// testable: `serviceCall.test.ts` proves, against a real HTTP server, which path
// actually arrives.

/**
 * @param call A `"module.service.action"` ref, as authored on `submit.call`.
 *             Its shape is validated at deploy time (`internal/ui/validate.go`);
 *             a malformed ref cannot reach here from a validated manifest.
 */
export function serviceCallPath(call: string): string {
  const [serviceModule, serviceName, action] = call.split(".")
  return `../service/${serviceModule}/${serviceName}/${action}`
}
