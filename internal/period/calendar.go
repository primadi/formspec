package period

import (
	"context"
	"time"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Guard checks period-closing state by reading the formspec.core.period-closing
// entity store (todo 7.11). It backs both the FORMSPEC.PERIOD.CLOSED write
// guard and the business-calendar resolution (§9.4).
type Guard struct {
	reg *entity.Registry
}

// NewGuard creates a period guard backed by the entity registry.
func NewGuard(reg *entity.Registry) *Guard {
	return &Guard{reg: reg}
}

// IsClosed reports whether the given period ("YYYY-MM") is closed for the
// workspace — i.e. there is a submitted (finalized) period-closing record for
// it. A cancelled (reopened) record means the period is open again.
func (g *Guard) IsClosed(ctx context.Context, workspaceID, period string) (bool, error) {
	store, err := g.reg.GetEntityStore(CoreModule, "period-closing")
	if err != nil {
		return false, err
	}
	rec, err := store.FindByField(ctx, workspaceID, "period", period)
	if err != nil {
		return false, err
	}
	if rec == nil {
		return false, nil
	}
	return rec.EffectiveDocStatus() == spec.DocStatusSubmitted, nil
}

// BusinessToday returns the business "today" — the day after the last closed
// period's EOD (02-core-extended.md §9.4), NOT the system clock. When no
// period is closed, it falls back to the system date. This keeps period
// calculations correct when EOD processing is delayed (the system may show
// day 5 while the business has not closed day 4 — business "today" is still
// day 4 until EOD completes).
func (g *Guard) BusinessToday(ctx context.Context, workspaceID string) (time.Time, error) {
	store, err := g.reg.GetEntityStore(CoreModule, "period-closing")
	if err != nil {
		return time.Now(), err
	}
	result, err := store.List(ctx, db.ListParams{WorkspaceID: workspaceID, PerPage: 100})
	if err != nil {
		return time.Now(), err
	}
	latest := ""
	for _, rec := range result.Data {
		if rec.EffectiveDocStatus() != spec.DocStatusSubmitted {
			continue
		}
		period, _ := rec.Data["period"].(string)
		if period > latest {
			latest = period
		}
	}
	if latest == "" {
		return time.Now(), nil
	}
	t, err := time.Parse("2006-01", latest)
	if err != nil {
		return time.Now(), nil
	}
	// EOD of the latest closed period = last day of that month; business
	// today = that + 1 day.
	eod := time.Date(t.Year(), t.Month()+1, 0, 23, 59, 59, 0, time.UTC)
	return eod.AddDate(0, 0, 1), nil
}
