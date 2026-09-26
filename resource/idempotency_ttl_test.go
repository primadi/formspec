package formspec

import (
	"testing"
	"time"

	"github.com/primadi/formspec/internal/config"
	"github.com/primadi/formspec/pkg/spec"
)

// TestResolveIdempotencyTTL pins the manifest→TTL mapping added for todo 2.1.6
// (01-core-basic.md §5: retention is read from `core.idempotency_retention`).
//
// Before this, the key was declared in the spec and documented in three places
// but read by nothing — `grep idempotency_retention` over Go found only
// comments. The TTL was therefore only settable in Go code, so a manifest that
// appeared to configure retention did nothing.
func TestResolveIdempotencyTTL(t *testing.T) {
	const fallback = 24 * time.Hour

	newReg := func(declared any) *config.Registry {
		reg := config.NewRegistry()
		if declared != nil {
			reg.Add("core", &spec.ConfigSpec{
				Keys: map[string]spec.ConfigKey{
					"idempotency_retention": {Type: "string", Default: declared},
				},
			})
		}
		return reg
	}

	t.Run("nil registry falls back", func(t *testing.T) {
		if got := resolveIdempotencyTTL(nil, fallback); got != fallback {
			t.Errorf("nil registry: got %s, want %s", got, fallback)
		}
	})

	t.Run("absent key falls back", func(t *testing.T) {
		if got := resolveIdempotencyTTL(newReg(nil), fallback); got != fallback {
			t.Errorf("absent key: got %s, want %s", got, fallback)
		}
	})

	t.Run("days parse", func(t *testing.T) {
		if got := resolveIdempotencyTTL(newReg("7d"), fallback); got != 7*24*time.Hour {
			t.Errorf("7d: got %s, want 168h", got)
		}
	})

	t.Run("hours parse", func(t *testing.T) {
		if got := resolveIdempotencyTTL(newReg("30m"), fallback); got != 30*time.Minute {
			t.Errorf("30m: got %s, want 30m", got)
		}
	})

	t.Run("zero means no expiry and is honored", func(t *testing.T) {
		// Declared explicitly as 0 → retention disabled, not "unset". This
		// mirrors BackdatePolicy max_days_back=0 ("unlimited").
		if got := resolveIdempotencyTTL(newReg(0), fallback); got != 0 {
			t.Errorf("0: got %s, want 0 (no expiry)", got)
		}
	})

	t.Run("garbage falls back rather than collapsing to zero", func(t *testing.T) {
		// The dangerous failure mode: a typo silently disabling retention.
		if got := resolveIdempotencyTTL(newReg("banana"), fallback); got != fallback {
			t.Errorf("garbage: got %s, want fallback %s", got, fallback)
		}
	})

	t.Run("bare integer is a count, not a duration", func(t *testing.T) {
		// "7" parses as a stream retention count; treating it as 7ns would be
		// a silent near-zero TTL.
		if got := resolveIdempotencyTTL(newReg("7"), fallback); got != fallback {
			t.Errorf("count-like value: got %s, want fallback %s", got, fallback)
		}
	})
}
