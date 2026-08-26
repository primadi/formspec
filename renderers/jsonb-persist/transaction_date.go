package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/primadi/formspec/pkg/spec"
)

// ErrPeriodClosed is returned when a transaction_date falls in a closed period
// (02-core-extended.md §9.3, todo 7.11.5).
var ErrPeriodClosed = fmt.Errorf("FORMSPEC.PERIOD.CLOSED")

// periodFromDate extracts the accounting period ("YYYY-MM") from a date
// string. Returns "" when the date cannot be parsed.
func periodFromDate(dateStr string) string {
	t, err := parseDate(dateStr)
	if err != nil {
		return ""
	}
	return t.Format("2006-01")
}

// validatePeriodGuard rejects a transaction whose transaction_date falls in a
// closed period (todo 7.11.5). Only applies to characteristic: transaction
// entities with a wired period guard. Reads the period-closing state via the
// guard callback (wired from resource/formspec.go).
func (s *EntityStore) validatePeriodGuard(ctx context.Context, workspaceID string, data map[string]any) error {
	if s.characteristic != spec.CharTransaction || s.periodGuard == nil {
		return nil
	}
	transactionDate, _ := data["transaction_date"].(string)
	if transactionDate == "" {
		return nil
	}
	period := periodFromDate(transactionDate)
	if period == "" {
		return nil
	}
	closed, err := s.periodGuard(ctx, workspaceID, period)
	if err != nil {
		return fmt.Errorf("period guard: %w", err)
	}
	if closed {
		return fmt.Errorf("%w: transaction_date %s falls in closed period %s", ErrPeriodClosed, transactionDate, period)
	}
	return nil
}

// ValidateTransactionDate checks that a transaction_date value respects the
// backdate/forward-date policy defined on the document spec (§14a).
//
// Rules:
//   - If `policy.MaxDaysBack > 0`, transaction_date must not be more than
//     MaxDaysBack days before today.
//   - If `policy.MaxDaysForward == 0` (default), forward dating is NOT allowed.
//   - If `policy.MaxDaysForward > 0`, transaction_date must not be more than
//     MaxDaysForward days after today.
//
// Returns nil if valid, or a TransactionDateError if the policy is violated.
// The caller (CRUD handler) is responsible for checking override_permission
// before allowing a blocked date through.
func ValidateTransactionDate(transactionDate string, maxDaysBack, maxDaysForward int) error {
	if transactionDate == "" {
		return nil // no value to validate
	}

	t, err := parseDate(transactionDate)
	if err != nil {
		return fmt.Errorf("invalid transaction_date %q: %w", transactionDate, err)
	}

	now := timeNow().Truncate(24 * time.Hour)
	date := t.Truncate(24 * time.Hour)

	daysDiff := int(now.Sub(date).Hours() / 24)

	if maxDaysBack > 0 && daysDiff > maxDaysBack {
		return &TransactionDatePolicyError{
			Date:      transactionDate,
			Direction: "backdate",
			Limit:     maxDaysBack,
			Actual:    daysDiff,
			Code:      "FORMSPEC.TXN.BACKDATE_EXCEEDED",
		}
	}

	if maxDaysForward >= 0 {
		// daysDiff positive = in the past, negative = in the future
		daysForward := -daysDiff
		if daysForward > maxDaysForward {
			return &TransactionDatePolicyError{
				Date:      transactionDate,
				Direction: "forward-date",
				Limit:     maxDaysForward,
				Actual:    daysForward,
				Code:      "FORMSPEC.TXN.FORWARD_DATE_EXCEEDED",
			}
		}
	}

	return nil
}

// DefaultBackdatePolicy returns the default backdate policy values.
func DefaultBackdatePolicy() (int, string) {
	return 3, "" // max 3 days back, no override permission
}

// DefaultForwardDatePolicy returns the default forward-date policy values.
// Default is 0 — no forward dating allowed.
func DefaultForwardDatePolicy() (int, string) {
	return 0, "" // no forward dating, no override permission
}

// parseDate attempts to parse a date string in YYYY-MM-DD or ISO 8601 format.
func parseDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as date", s)
}

// TransactionDatePolicyError is returned when a transaction_date violates the
// backdate or forward-date policy.
type TransactionDatePolicyError struct {
	Date      string
	Direction string // "backdate" or "forward-date"
	Limit     int
	Actual    int
	Code      string
}

func (e *TransactionDatePolicyError) Error() string {
	return fmt.Sprintf("[%s] transaction_date %s exceeds %s limit: max %d days, got %d days",
		e.Code, e.Date, e.Direction, e.Limit, e.Actual)
}

// validateTransactionDatePolicy checks the transaction_date in data against the
// entity's backdate/forward-date policy. Uses defaults if no policy is set.
//
// permissions carries the caller's effective permissions. If the entity's
// backdate/forward-date policy declares an override_permission and the caller
// holds it, the policy is bypassed — the "special path" for stale records,
// so authorized staff can touch them without widening the policy itself.
func (s *EntityStore) validateTransactionDatePolicy(data map[string]any, permissions []string) error {
	transactionDate, exists := data["transaction_date"]
	if !exists || transactionDate == nil || transactionDate == "" {
		return nil // no transaction_date to validate
	}

	dateStr, ok := transactionDate.(string)
	if !ok {
		return nil // wrong type, skip
	}

	maxDaysBack := 3
	maxDaysForward := 0

	if s.backdatePolicy != nil && s.backdatePolicy.MaxDaysBack > 0 {
		maxDaysBack = s.backdatePolicy.MaxDaysBack
	}
	if s.forwardDatePolicy != nil {
		maxDaysForward = s.forwardDatePolicy.MaxDaysForward
	}

	// override_permission: an authorized caller may bypass the policy without
	// widening it (special path for stale records).
	if s.backdatePolicy != nil && s.backdatePolicy.OverridePermission != "" &&
		hasPermission(permissions, s.backdatePolicy.OverridePermission) {
		return nil
	}
	if s.forwardDatePolicy != nil && s.forwardDatePolicy.OverridePermission != "" &&
		hasPermission(permissions, s.forwardDatePolicy.OverridePermission) {
		return nil
	}

	return ValidateTransactionDate(dateStr, maxDaysBack, maxDaysForward)
}

// hasPermission reports whether any held permission grants the required one,
// mirroring auth.Identity.HasPermission wildcard semantics (exact match,
// "a.b.*" prefix, and "*"). Kept local so the renderer stays decoupled from
// internal/auth.
func hasPermission(held []string, required string) bool {
	if required == "" || required == "public" {
		return true
	}
	for _, p := range held {
		if p == "*" || p == required {
			return true
		}
		if before, ok := strings.CutSuffix(p, ".*"); ok {
			if strings.HasPrefix(required, before) {
				rest := strings.TrimPrefix(required, before)
				if strings.HasPrefix(rest, ".") && !strings.Contains(rest[1:], ".") {
					return true
				}
			}
		}
	}
	return false
}
