package db

import (
	"testing"
	"time"
)

func TestValidateTransactionDate_NoPolicy(t *testing.T) {
	// No policy = default 3 days back, 0 forward
	today := timeNow().Format("2006-01-02")
	if err := ValidateTransactionDate(today, 3, 0); err != nil {
		t.Errorf("today should be valid: %v", err)
	}
}

func TestValidateTransactionDate_BackdateWithinLimit(t *testing.T) {
	yesterday := timeNow().Add(-24 * time.Hour).Format("2006-01-02")
	if err := ValidateTransactionDate(yesterday, 3, 0); err != nil {
		t.Errorf("yesterday within 3-day limit should be valid: %v", err)
	}
}

func TestValidateTransactionDate_BackdateExceeded(t *testing.T) {
	oldDate := timeNow().Add(-10 * 24 * time.Hour).Format("2006-01-02")
	err := ValidateTransactionDate(oldDate, 3, 0)
	if err == nil {
		t.Error("10 days back with 3-day limit should be rejected")
	}
	if err != nil {
		pe, ok := err.(*TransactionDatePolicyError)
		if !ok {
			t.Errorf("expected TransactionDatePolicyError, got %T", err)
		}
		if pe.Code != "FORMA.TXN.BACKDATE_EXCEEDED" {
			t.Errorf("expected code BACKDATE_EXCEEDED, got %s", pe.Code)
		}
	}
}

func TestValidateTransactionDate_ForwardDateBlocked(t *testing.T) {
	tomorrow := timeNow().Add(24 * time.Hour).Format("2006-01-02")
	err := ValidateTransactionDate(tomorrow, 3, 0)
	if err == nil {
		t.Error("tomorrow with max_days_forward=0 should be rejected")
	}
}

func TestValidateTransactionDate_ForwardDateWithinLimit(t *testing.T) {
	tomorrow := timeNow().Add(24 * time.Hour).Format("2006-01-02")
	if err := ValidateTransactionDate(tomorrow, 3, 5); err != nil {
		t.Errorf("tomorrow within 5-day forward limit should be valid: %v", err)
	}
}

func TestValidateTransactionDate_ForwardDateExceeded(t *testing.T) {
	farFuture := timeNow().Add(30 * 24 * time.Hour).Format("2006-01-02")
	err := ValidateTransactionDate(farFuture, 3, 7)
	if err == nil {
		t.Error("30 days forward with 7-day limit should be rejected")
	}
	if err != nil {
		pe, ok := err.(*TransactionDatePolicyError)
		if !ok {
			t.Errorf("expected TransactionDatePolicyError, got %T", err)
		}
		if pe.Code != "FORMA.TXN.FORWARD_DATE_EXCEEDED" {
			t.Errorf("expected code FORWARD_DATE_EXCEEDED, got %s", pe.Code)
		}
	}
}

func TestValidateTransactionDate_EmptyDate(t *testing.T) {
	if err := ValidateTransactionDate("", 3, 0); err != nil {
		t.Errorf("empty date should be skipped: %v", err)
	}
}

func TestValidateTransactionDate_BackdateUnlimited(t *testing.T) {
	oldDate := timeNow().Add(-365 * 24 * time.Hour).Format("2006-01-02")
	// maxDaysBack=0 means no limit
	if err := ValidateTransactionDate(oldDate, 0, 0); err != nil {
		t.Errorf("backdate unlimited (max_days_back=0) should pass: %v", err)
	}
}
