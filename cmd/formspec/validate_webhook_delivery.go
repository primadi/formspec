// Cross-manifest and shape validation for the `webhook` delivery channel
// (todo 7.7.6, unsigned version).
//
// A webhook entry needs an ENDPOINT, and there is exactly one of two places it
// may come from: a literal `url:` in the manifest, or `url_from: {config: ...}`
// when the address belongs to the deployment rather than to the module. An entry
// with neither validates green and then has nowhere to post — the consequence
// silently never happens, which is the failure this whole file exists to catch.
//
// The config KEY is checked for SHAPE, not for existence: which module declares
// it is a deployment question (the key namespace is shared), so a missing key is
// reported at delivery time where it can name the key and the event.
package main

import (
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// validateWebhookDelivery checks one `webhook` entry, returning a
// ready-to-append message or "".
func validateWebhookDelivery(wh *spec.WebhookDeliveryDecl) string {
	if wh == nil {
		return "`webhook` entry has no `webhook:` block — there is no endpoint to post to"
	}
	hasURL := strings.TrimSpace(wh.URL) != ""
	hasRef := wh.URLFrom != nil && strings.TrimSpace(wh.URLFrom.Config) != ""
	switch {
	case hasURL && hasRef:
		return "`webhook` declares both `url:` and `url_from:` — pick one; with both, the runtime would silently prefer one and the intent would be unreadable"
	case !hasURL && !hasRef:
		return "`webhook` declares neither `url:` nor `url_from: {config: ...}` — the delivery would have nowhere to post"
	case hasURL:
		// A literal must be an absolute http(s) address: a relative or scheme-less
		// value would fail at request time, where the error is far from the manifest.
		u := strings.TrimSpace(wh.URL)
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return fmt.Sprintf("`webhook.url` %q must be an absolute http(s) URL — a relative or scheme-less address cannot be posted to", u)
		}
	}
	return ""
}

// scanWebhookDeliveries reports webhook entries whose endpoint is undeclared or
// malformed, across every entity's events. Warnings, not errors: an entry may be
// mid-migration, and the runtime already fails the delivery loudly (outbox retry
// → dead-letter), so the manifest does not need to be rejected as well.
func scanWebhookDeliveries(manifests []manifest.RawManifest) []honestyIssue {
	var issues []honestyIssue
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || es == nil {
			continue
		}
		for _, e := range es.Events {
			for _, d := range e.Deliver {
				if d.Channel != spec.ChannelWebhook {
					continue
				}
				if msg := validateWebhookDelivery(d.Webhook); msg != "" {
					issues = append(issues, honestyIssue{
						Source:   m.Source,
						Severity: "warning",
						Message:  fmt.Sprintf("event %q %s (todo 7.7.6)", e.Name, msg),
					})
				}
			}
		}
	}
	return issues
}
