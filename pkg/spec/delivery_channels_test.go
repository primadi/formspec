package spec

import "testing"

// TestChannelUnsupported pins the single source of truth for 7.7.6: the
// channels the runtime does NOT deliver, so the validator, the runtime, and the
// docs cannot drift apart about which ones work.
func TestChannelUnsupported(t *testing.T) {
	// Every declared channel is delivered as of 2026-10-06: `queue` (outbox job
	// runner), `notification` (notify module), `webhook` (unsigned outbound
	// sender). None may be reported unsupported.
	for _, ch := range EventChannelValues() {
		if reason, unsupported := ChannelUnsupported(ch); unsupported {
			t.Errorf("%q is declared and delivered but is reported unsupported: %s", ch, reason)
		}
	}

	// An unknown name is not "unsupported" either — it is not a channel at all,
	// and the runtime's default branch is what refuses it.
	if _, unsupported := ChannelUnsupported("carrier-pigeon"); unsupported {
		t.Error("an unknown name is not a declared-but-undelivered channel")
	}
}

// TestUnsupportedChannelNames_IsEmpty documents the state: nothing is declared
// and undelivered. The mechanism is kept so the next channel added without a
// delivery branch is reported rather than silently succeeding.
func TestUnsupportedChannelNames_IsEmpty(t *testing.T) {
	if names := UnsupportedChannelNames(); len(names) != 0 {
		t.Fatalf("names = %v, want none — every declared channel is delivered", names)
	}
}

// TestResolveJobRef pins the contract for `deliver: {channel: queue, job: ...}`:
// a job names a Service action, and an ambiguous bare name is REFUSED rather
// than guessed.
func TestResolveJobRef(t *testing.T) {
	t.Run("service.action defaults to the publisher's module", func(t *testing.T) {
		module, svc, action, err := ResolveJobRef("receipt-jobs.generate-receipt", "billing")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if module != "billing" || svc != "receipt-jobs" || action != "generate-receipt" {
			t.Fatalf("got %s.%s.%s, want billing.receipt-jobs.generate-receipt", module, svc, action)
		}
	})

	t.Run("module.service.action is taken as written", func(t *testing.T) {
		module, svc, action, err := ResolveJobRef("gl.journal-jobs.post", "billing")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if module != "gl" || svc != "journal-jobs" || action != "post" {
			t.Fatalf("got %s.%s.%s, want gl.journal-jobs.post", module, svc, action)
		}
	})

	// A bare name has no Service to belong to, so it must be refused — resolving
	// it against some convention would be inventing one.
	for _, bad := range []string{"", "generate-receipt", "a.b.c.d", "a..b", "a.b."} {
		if _, _, _, err := ResolveJobRef(bad, "billing"); err == nil {
			t.Errorf("job %q must be refused", bad)
		}
	}
}

// TestSubscriptionDeliveryIsInert documents WHY the Tier-2 block is reported: a
// search for reads of SubscriptionSpec.Delivery finds none, so the whole block
// changes nothing — which includes the retry/dead-letter settings a reader
// would reasonably expect it to imply.
func TestSubscriptionDeliveryIsInert(t *testing.T) {
	reason, inert := SubscriptionDeliveryIsInert()
	if !inert {
		t.Fatal("the Tier-2 delivery block is not consumed; it must be reported")
	}
	if !contains(reason, "not read") {
		t.Errorf("the reason should say the field is unread, got %q", reason)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || len(haystack) >= len(needle) && indexOfStr(haystack, needle) >= 0
}

func indexOfStr(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
