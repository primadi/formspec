// ─── Approval inbox call path ───
//
// Builds the API-client path for the pending-approval source (kind:
// ApprovalInbox, frontend/06-page-kinds.md §11, todo 5.13.6).
//
// The `../` is load-bearing and easy to get wrong, exactly as in
// `serviceCall.ts`: the client is created with `prefix: "/{ws}/_ui/entity"`, so
// a relative path starting with `../` climbs out of the `entity` segment and
// lands on `/{ws}/_ui/workflow/approvals` — where the runtime registers the
// approval surface. An absolute `/_ui/workflow/...` would be resolved against
// the ORIGIN instead of the prefix and miss the workspace slug entirely, and
// `/workflow/...` would 404. Both failures are silent: the inbox would simply
// render "no pending approvals" forever, which is indistinguishable from an
// empty queue.
//
// These are named functions rather than inline templates so the assumption is
// testable: `approvalInbox.test.ts` proves, against a real HTTP server, which
// path actually arrives.

/** Path (relative to the API client's prefix) listing the caller's tasks. */
export function approvalListPath(app?: string): string {
  const qs = app ? `?app=${encodeURIComponent(app)}` : ""
  return `../workflow/approvals${qs}`
}

/** Path (relative to the API client's prefix) deciding one task. */
export function approvalDecisionPath(id: string): string {
  return `../workflow/approvals/${encodeURIComponent(id)}`
}
