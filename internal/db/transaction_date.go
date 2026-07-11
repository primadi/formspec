package db

import (
	"fmt"
	"time"
)

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
			Code:      "FORMA.TXN.BACKDATE_EXCEEDED",
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
				Code:      "FORMA.TXN.FORWARD_DATE_EXCEEDED",
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
func (s *EntityStore) validateTransactionDatePolicy(data map[string]any) error {
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

	return ValidateTransactionDate(dateStr, maxDaysBack, maxDaysForward)
}
