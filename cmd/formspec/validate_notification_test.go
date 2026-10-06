package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
)

// deliveryEntityWith builds an Entity whose `paid` event has one delivery entry,
// so a test states only the entry it is about.
func deliveryEntityWith(source string, entry map[string]any) manifest.RawManifest {
	return manifest.RawManifest{
		APIVersion: "formspec.dev/v1",
		Kind:       "Entity",
		Source:     source,
		Metadata:   manifest.RawMetadata{Name: "order", Module: "billing"},
		Spec: map[string]any{
			"version":        "v1",
			"characteristic": "transaction",
			"fields":         []any{map[string]any{"name": "status", "type": "string"}},
			"events": []any{map[string]any{
				"name":    "paid",
				"type":    "async",
				"publish": map[string]any{"durable": true},
				"deliver": []any{entry},
			}},
		},
	}
}

// TestValidateEventTargets_NotificationNeedsContent: an entry that declares
// neither the in-app block nor a handler would deliver nothing — the failure
// this whole feature exists to remove.
func TestValidateEventTargets_NotificationNeedsContent(t *testing.T) {
	msg, ok := validateEventTargets([]manifest.RawManifest{
		deliveryEntityWith("order.yaml", map[string]any{"channel": "notification"}),
	})["order.yaml"]
	if !ok {
		t.Fatal("an empty notification entry must be rejected")
	}
	if !strings.Contains(msg, "would deliver nothing") {
		t.Errorf("error %q should say it plainly", msg)
	}
}

// TestValidateEventTargets_NotificationNeedsRecipient: `recipient_id` is what
// the entity's row_scope matches, so a missing recipient is a row nobody can
// read — refused at validate time rather than written and invisible.
func TestValidateEventTargets_NotificationNeedsRecipient(t *testing.T) {
	msg, ok := validateEventTargets([]manifest.RawManifest{
		deliveryEntityWith("order.yaml", map[string]any{
			"channel":      "notification",
			"notification": map[string]any{"title": "Paid"},
		}),
	})["order.yaml"]
	if !ok {
		t.Fatal("a notification without `recipient` must be rejected")
	}
	if !strings.Contains(msg, "recipient") {
		t.Errorf("error %q should name the missing field", msg)
	}
}

// TestValidateEventTargets_NotificationHandlerMustResolve mirrors the `job:`
// rule: a handler naming a Service action that does not exist validates green
// and then dead-letters.
func TestValidateEventTargets_NotificationHandlerMustResolve(t *testing.T) {
	base := map[string]any{
		"channel":      "notification",
		"notification": map[string]any{"recipient": "customer_id", "title": "Paid"},
	}

	ok := append([]manifest.RawManifest{
		deliveryEntityWith("order.yaml", withHandler(base, "notify-jobs.send-email")),
	}, queueService("notify-jobs.yaml", "billing", "notify-jobs", "send-email"))
	if rejects := validateEventTargets(ok); len(rejects) != 0 {
		t.Fatalf("a resolving handler must be accepted, got %v", rejects)
	}

	bad := append([]manifest.RawManifest{
		deliveryEntityWith("order.yaml", withHandler(base, "notify-jobs.send-email")),
	}, queueService("notify-jobs.yaml", "billing", "notify-jobs", "something-else"))
	msg, found := validateEventTargets(bad)["order.yaml"]
	if !found {
		t.Fatal("a handler naming a non-existent Service action must be rejected")
	}
	for _, want := range []string{"send-email", "does not exist"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should mention %q", msg, want)
		}
	}
}

// withHandler returns a copy of the entry with `handler:` set.
func withHandler(entry map[string]any, handler string) map[string]any {
	out := make(map[string]any, len(entry)+1)
	for k, v := range entry {
		out[k] = v
	}
	out["handler"] = handler
	return out
}
