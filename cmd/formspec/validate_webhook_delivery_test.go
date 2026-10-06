package main

import (
	"strings"
	"testing"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// webhookEntity builds an Entity whose `paid` event posts to the given webhook
// block, so a test states only what it varies.
func webhookEntity(source string, wh map[string]any) manifest.RawManifest {
	entry := map[string]any{"channel": "webhook"}
	if wh != nil {
		entry["webhook"] = wh
	}
	return deliveryEntityWith(source, entry)
}

// TestScanWebhookDeliveries_RequiresAnEndpoint: with neither `url:` nor
// `url_from:`, the delivery has nowhere to post — the consequence would silently
// never happen.
func TestScanWebhookDeliveries_RequiresAnEndpoint(t *testing.T) {
	for name, wh := range map[string]map[string]any{
		"no block":    nil,
		"empty block": {},
	} {
		issues := scanWebhookDeliveries([]manifest.RawManifest{webhookEntity("order.yaml", wh)})
		if len(issues) != 1 {
			t.Errorf("%s: issues = %+v, want one", name, issues)
			continue
		}
		if !strings.Contains(issues[0].Message, "post") {
			t.Errorf("%s: message %q should say there is nowhere to post", name, issues[0].Message)
		}
		if issues[0].Severity != "warning" {
			t.Errorf("%s: severity = %q — the runtime also fails it, so this is advisory", name, issues[0].Severity)
		}
	}
}

// TestScanWebhookDeliveries_RejectsBothSources: declaring `url:` AND `url_from:`
// leaves the intent unreadable — the runtime would silently prefer one.
func TestScanWebhookDeliveries_RejectsBothSources(t *testing.T) {
	issues := scanWebhookDeliveries([]manifest.RawManifest{webhookEntity("order.yaml", map[string]any{
		"url":      "https://example.com/hook",
		"url_from": map[string]any{"config": "billing.webhook_url"},
	})})
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "both") {
		t.Fatalf("issues = %+v, want a 'pick one' warning", issues)
	}
}

// TestScanWebhookDeliveries_RequiresAbsoluteURL: a relative or scheme-less
// address fails at request time, far from the manifest that caused it.
func TestScanWebhookDeliveries_RequiresAbsoluteURL(t *testing.T) {
	issues := scanWebhookDeliveries([]manifest.RawManifest{webhookEntity("order.yaml", map[string]any{
		"url": "/hooks/order-paid",
	})})
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "absolute") {
		t.Fatalf("issues = %+v, want an absolute-URL warning", issues)
	}
}

// TestScanWebhookDeliveries_ResolvingEntriesAreSilent: a literal absolute URL and
// a config reference both resolve, so neither may be reported.
func TestScanWebhookDeliveries_ResolvingEntriesAreSilent(t *testing.T) {
	for name, wh := range map[string]map[string]any{
		"literal": {"url": "https://example.com/hook"},
		"config":  {"url_from": map[string]any{"config": "billing.webhook_url"}},
	} {
		if issues := scanWebhookDeliveries([]manifest.RawManifest{webhookEntity("order.yaml", wh)}); len(issues) != 0 {
			t.Errorf("%s: reported %+v", name, issues)
		}
	}
}

// TestScanWebhookDeliveries_OtherChannelsAreIgnored: the scan must only look at
// `webhook` entries; applying its rules to a `queue` entry would be wrong (a job
// needs no URL).
func TestScanWebhookDeliveries_OtherChannelsAreIgnored(t *testing.T) {
	raw := deliveryEntityWith("order.yaml", map[string]any{"channel": "queue", "job": "svc.act"})
	if issues := scanWebhookDeliveries([]manifest.RawManifest{raw}); len(issues) != 0 {
		t.Fatalf("a queue entry has no URL to check, got %+v", issues)
	}
}

// TestValidateWebhookDelivery_ShapeUnit pins the three refusals directly, without
// going through a manifest.
func TestValidateWebhookDelivery_ShapeUnit(t *testing.T) {
	cases := []struct {
		name string
		wh   *spec.WebhookDeliveryDecl
		want string
	}{
		{"nil block", nil, "no `webhook:` block"},
		{"neither source", &spec.WebhookDeliveryDecl{}, "neither"},
		{"both sources", &spec.WebhookDeliveryDecl{URL: "https://e.com", URLFrom: &spec.WebhookURLRef{Config: "k"}}, "both"},
		{"relative url", &spec.WebhookDeliveryDecl{URL: "hooks/x"}, "absolute"},
		{"literal ok", &spec.WebhookDeliveryDecl{URL: "https://e.com/x"}, ""},
		{"config ok", &spec.WebhookDeliveryDecl{URLFrom: &spec.WebhookURLRef{Config: "k"}}, ""},
	}
	for _, c := range cases {
		got := validateWebhookDelivery(c.wh)
		if c.want == "" {
			if got != "" {
				t.Errorf("%s: got %q, want no problem", c.name, got)
			}
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q, want it to mention %q", c.name, got, c.want)
		}
	}
}
