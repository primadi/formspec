package spec

import (
	"fmt"
	"sort"
	"strings"
)

// Unsupported delivery channels — one source of truth for the validator, the
// runtime, and the docs (todo 7.7.6).
//
// A channel that is declared, accepted, and then never delivered is the worst
// of the three states: the manifest looks configured, `formspec validate`
// stays green, and the outbox marks the entry completed — so nothing in the
// data shows the consequence never happened. This list makes that state
// impossible to mistake for a working one: the validator reports it, and the
// runtime fails the delivery instead of swallowing it.

// unsupportedChannels maps a channel name to WHY it is not delivered, so every
// surface can print the same reason instead of inventing its own.
//
// EMPTY as of 2026-10-06: every channel the enum declares is now delivered
// (`queue` via the outbox job runner, `notification` by the notify module,
// `webhook` by the unsigned outbound sender). The mechanism stays because it is
// what made the gap visible in the first place: the moment a channel is added to
// the enum without a delivery branch, it belongs here instead of silently
// reporting success.
var unsupportedChannels = map[string]string{}

// ChannelUnsupported reports whether a delivery channel is declared but not
// delivered, returning the reason. Callers must scope this to the two fields
// that carry delivery channels — `EventDeliveryDecl.Channel` and
// `SubDeliveryDecl.Channel` — never to `CallbackDecl.Channel`, whose `webhook`
// IS implemented.
func ChannelUnsupported(channel string) (string, bool) {
	reason, ok := unsupportedChannels[channel]
	return reason, ok
}

// UnsupportedChannelNames lists the unsupported channel names, sorted, for
// error messages and docs.
func UnsupportedChannelNames() []string {
	out := make([]string, 0, len(unsupportedChannels))
	for name := range unsupportedChannels {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// SubscriptionDeliveryIsInert reports whether a `kind: Subscription`'s Tier-2
// `delivery:` block is consumed by the runtime.
//
// It is not: a search for reads of `SubscriptionSpec.Delivery` finds none, so
// the whole block — channel, and the retry/dead-letter settings a reader would
// expect it to imply — changes nothing. Reported separately from
// ChannelUnsupported because the problem is the BLOCK, not a single channel
// value inside it.
func SubscriptionDeliveryIsInert() (string, bool) {
	return "a Subscription's Tier-2 `delivery:` block is not read by the runtime — declaring it changes nothing (7.7.6)", true
}

// ResolveServiceActionRef resolves a Service-action reference written
// `service.action` (own module) or `module.service.action` (explicit), used by
// both `job:` (queue) and `handler:` (notification) — one reference vocabulary,
// not two.
//
// ownModule is the publishing entity's module, used when the reference omits
// it. A reference that is not 2 or 3 segments is refused rather than guessed: a
// bare name has no Service to belong to.
func ResolveServiceActionRef(ref, ownModule string) (module, service, action string, err error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", "", "", fmt.Errorf("empty Service action reference")
	}
	parts := strings.Split(trimmed, ".")
	switch len(parts) {
	case 2:
		module, service, action = strings.TrimSpace(ownModule), strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	case 3:
		module, service, action = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
	default:
		return "", "", "", fmt.Errorf(
			"%q must name a Service action as `service.action` or `module.service.action` — a bare name is ambiguous about which Service owns it", ref)
	}
	if module == "" || service == "" || action == "" {
		return "", "", "", fmt.Errorf("%q has an empty segment (module/service/action must all be present)", ref)
	}
	return module, service, action, nil
}

// ResolveJobRef resolves a `deliver: {channel: queue, job: <ref>}` reference to
// the Service action it names (todo 7.7.6).
//
// Two forms, and the reason each is allowed:
//
//	receipt-jobs.generate-receipt        # same module as the publisher
//	billing.receipt-jobs.generate-receipt # explicit module
//
// A job is stateless computation, so its home is `kind: Service` — which
// already carries an `impl`, permission/`uses` enforcement, and a dispatcher.
// Naming a Service action reuses that machinery instead of adding a second
// registry for the same work.
//
// eventModule is the publishing entity's module, used when the reference omits
// it (mirroring how `deliver[].target` and `Integrator.call` default their
// module). A reference that does not have 2 or 3 segments is refused rather
// than guessed: a bare `job: generate-receipt` would have to be resolved
// against a Service nobody named.
func ResolveJobRef(job, eventModule string) (module, service, action string, err error) {
	ref := strings.TrimSpace(job)
	if ref == "" {
		return "", "", "", fmt.Errorf("`queue` deliver entry has no `job:` — the worker would have nothing to call")
	}
	module, service, action, err = ResolveServiceActionRef(ref, eventModule)
	if err != nil {
		return "", "", "", fmt.Errorf("`job:` %w", err)
	}
	return module, service, action, nil
}
