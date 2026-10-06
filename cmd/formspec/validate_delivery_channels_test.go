package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// deliveryEntity builds an Entity whose single event delivers on the given
// channel, so a test states only the channel it is about.
func deliveryEntity(source, channel, job string) manifest.RawManifest {
	deliver := map[string]any{"channel": channel}
	if job != "" {
		deliver["job"] = job
	}
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: "order", Module: "demo"},
		Spec: map[string]any{
			"version":        "v1",
			"characteristic": "transaction",
			"fields":         []any{map[string]any{"name": "status", "type": "string"}},
			"events": []any{map[string]any{
				"name":    "completed",
				"type":    "async",
				"publish": map[string]any{"durable": true},
				"deliver": []any{deliver},
			}},
		},
	}
}

// TestScanDeliveryChannels_NothingIsUndelivered documents the state after 7.7.6:
// every declared channel is delivered, so the scan reports nothing. It stayed in
// place as the guard that will report the next channel added without a delivery
// branch.
func TestScanDeliveryChannels_NothingIsUndelivered(t *testing.T) {
	for _, ch := range []string{"audit_log", "websocket", "pubsub", "reliable_event", "queue", "notification", "webhook"} {
		issues := scanDeliveryChannels([]manifest.RawManifest{deliveryEntity("order.yaml", ch, "receipt-jobs.generate")})
		if len(issues) != 0 {
			t.Errorf("channel %q is delivered but was reported: %+v", ch, issues)
		}
	}
}

// TestScanDeliveryChannels_InertSubscriptionDelivery covers the second shape:
// the Tier-2 `delivery:` block on a Subscription is not read by the runtime at
// all, so declaring it — channel and all — changes nothing.
func TestScanDeliveryChannels_InertSubscriptionDelivery(t *testing.T) {
	raw := manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Subscription",
		Source:     "sub.yaml",
		Metadata:   manifest.RawMetadata{Name: "on-completed", Module: "demo"},
		Spec: map[string]any{
			"events":   []any{"demo.order.completed"},
			"handler":  map[string]any{"type": "native", "ref": "Demo.Handle"},
			"delivery": map[string]any{"channel": "webhook", "url_from": "config"},
		},
	}

	issues := scanDeliveryChannels([]manifest.RawManifest{raw})
	if len(issues) != 1 {
		t.Fatalf("issues = %+v, want one for the inert delivery block", issues)
	}
	msg := issues[0].Message
	if !strings.Contains(msg, "inert") {
		t.Errorf("message %q should say the block is inert", msg)
	}
	// It must name the channel the author declared — that is the value they would
	// otherwise believe is in effect.
	if !strings.Contains(msg, "webhook") {
		t.Errorf("message %q should name the declared channel", msg)
	}
}
