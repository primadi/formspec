// Delivery-channel honesty checks (todo 7.7.6).
//
// A channel that is declared, accepted by the schema, and then never delivered
// is worse than a rejected one: the manifest looks configured and `formspec
// validate` stays green. Measured before this existed — `events[].deliver:
// [{channel: queue}]` validated 0 problems while the runtime logged
// `event.channel_not_implemented` and marked the delivery COMPLETED, so nothing
// in the data showed the consequence never happened.
//
// These checks are warnings, not errors: `queue` is used by `verticals/*` and
// the guides' tutorials, which are not migrated yet. A warning still removes the
// failure mode that matters — an author can no longer believe the channel works
// just because validate passed.
package main

import (
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// scanDeliveryChannels reports every delivery channel a manifest declares that
// the runtime does not deliver, plus the inert Tier-2 Subscription `delivery:`
// block. Returns honesty issues (severity: warning) so they ride the existing
// advisory channel and cannot be mistaken for shape errors.
func scanDeliveryChannels(manifests []manifest.RawManifest) []honestyIssue {
	var issues []honestyIssue

	for _, m := range manifests {
		if m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}

		switch spec.Kind(m.Kind) {
		case spec.KindEntity:
			es, err := manifest.RawSpecToEntitySpec(specMap)
			if err != nil || es == nil {
				continue
			}
			for _, e := range es.Events {
				for _, d := range e.Deliver {
					reason, unsupported := spec.ChannelUnsupported(string(d.Channel))
					if !unsupported {
						continue
					}
					issues = append(issues, honestyIssue{
						Source:   m.Source,
						Severity: "warning",
						Message: fmt.Sprintf(
							"event %q delivers on channel %q, which the runtime does NOT deliver — %s. The delivery will fail (outbox retry → dead-letter) rather than happen",
							e.Name, d.Channel, reason),
					})
				}
			}

		case spec.KindSubscription:
			sub, err := manifest.RawSpecTo[spec.SubscriptionSpec](specMap)
			if err != nil || sub == nil || sub.Delivery == nil {
				continue
			}
			if reason, inert := spec.SubscriptionDeliveryIsInert(); inert {
				ch := strings.TrimSpace(sub.Delivery.Channel)
				msg := fmt.Sprintf("`delivery:` (channel %q) is inert — %s", ch, reason)
				if reason, unsupported := spec.ChannelUnsupported(ch); unsupported {
					msg += "; the channel itself is also undelivered — " + reason
				}
				issues = append(issues, honestyIssue{Source: m.Source, Severity: "warning", Message: msg})
			}
		}
	}

	return issues
}
