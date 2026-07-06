package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NaturalKeyCounter manages sequential natural key generation.
// Uses the forma_natural_key_counters system table with atomic increments.
//
// Each counter is scoped by (tenant_id, resource, field, scope, period).
// The period is computed from the current time based on the Reset strategy:
//   - yearly  → "2026"
//   - monthly → "2026-07"
//   - daily   → "2026-07-05"
//   - never   → "" (global counter)
type NaturalKeyCounter struct {
	db     DB
	driver DriverType
}

// NewNaturalKeyCounter creates a new counter manager.
func NewNaturalKeyCounter(db DB, driver DriverType) *NaturalKeyCounter {
	return &NaturalKeyCounter{db: db, driver: driver}
}

// NextSequence atomically increments and returns the next counter value.
// Parameters:
//   - tenantID:  tenant scope
//   - resource:  entity name (e.g. "invoice")
//   - field:     field name (e.g. "number")
//   - scope:     additional scoping (e.g. "sales-channel-1")
//   - reset:     reset strategy: "never", "yearly", "monthly", "daily"
//
// Returns the next counter value (1-based).
func (c *NaturalKeyCounter) NextSequence(ctx context.Context, tenantID, resource, field, scope, reset string) (int64, string, error) {
	period := computePeriod(reset)

	// Use INSERT ... ON CONFLICT DO UPDATE (UPSERT) for atomicity
	query := `
		INSERT INTO forma_natural_key_counters (tenant_id, resource, field, scope, period, counter)
		VALUES (?, ?, ?, ?, ?, 1)
		ON CONFLICT(tenant_id, resource, field, scope, period) DO UPDATE
		SET counter = forma_natural_key_counters.counter + 1
		RETURNING counter
	`

	var counter int64
	err := c.db.QueryRowContext(ctx, query,
		tenantID, resource, field, scope, period,
	).Scan(&counter)
	if err != nil {
		// SQLite compatibility: ON CONFLICT ... RETURNING may not work
		// Fallback: explicit insert-or-increment
		return c.nextSequenceFallback(ctx, tenantID, resource, field, scope, period)
	}

	return counter, period, nil
}

// nextSequenceFallback uses a two-step approach for SQLite compatibility.
func (c *NaturalKeyCounter) nextSequenceFallback(ctx context.Context, tenantID, resource, field, scope, period string) (int64, string, error) {
	// Try insert first
	_, err := c.db.ExecContext(ctx, `
		INSERT INTO forma_natural_key_counters (tenant_id, resource, field, scope, period, counter)
		VALUES (?, ?, ?, ?, ?, 1)
	`, tenantID, resource, field, scope, period)

	if err != nil && strings.Contains(err.Error(), "UNIQUE") ||
		err != nil && strings.Contains(err.Error(), "PRIMARY KEY") {
		// Row exists: increment
		var counter int64
		err2 := c.db.QueryRowContext(ctx, `
			UPDATE forma_natural_key_counters
			SET counter = counter + 1
			WHERE tenant_id = ? AND resource = ? AND field = ? AND scope = ? AND period = ?
			RETURNING counter
		`, tenantID, resource, field, scope, period).Scan(&counter)
		if err2 != nil {
			return 0, period, fmt.Errorf("counter increment: %w", err2)
		}
		return counter, period, nil
	}

	if err != nil {
		return 0, period, fmt.Errorf("counter insert: %w", err)
	}

	// First row was inserted successfully
	return 1, period, nil
}

// GenerateNaturalKey generates a formatted natural key using the counter.
// The format string supports:
//   - {counter}       → raw counter value (e.g. "1", "42")
//   - {counter:05d}   → zero-padded counter (e.g. "00001", "00042")
//   - {counter:04d}   → zero-padded (e.g. "0001")
//   - {period}        → computed period string
//   - {year}          → current year "2026"
//   - {month}         → current month "07"
//   - {day}           → current day "05"
//   - {resource}      → entity name
//   - {field}         → field name
func (c *NaturalKeyCounter) GenerateNaturalKey(ctx context.Context, tenantID, resource, field, scope, reset, format string) (string, error) {
	if format == "" {
		format = "{prefix}-{period}-{counter:05d}"
	}

	counter, period, err := c.NextSequence(ctx, tenantID, resource, field, scope, reset)
	if err != nil {
		return "", fmt.Errorf("generate natural key for %s.%s: %w", resource, field, err)
	}

	return renderFormat(format, counter, period, resource, field, ""), nil
}

// PeekCounter returns the current counter value WITHOUT incrementing.
// Useful for display purposes. Returns 0 if no counter exists yet.
func (c *NaturalKeyCounter) PeekCounter(ctx context.Context, tenantID, resource, field, scope, reset string) (int64, string, error) {
	period := computePeriod(reset)

	var counter int64
	err := c.db.QueryRowContext(ctx, `
		SELECT counter FROM forma_natural_key_counters
		WHERE tenant_id = ? AND resource = ? AND field = ? AND scope = ? AND period = ?
	`, tenantID, resource, field, scope, period).Scan(&counter)
	if err != nil {
		return 0, period, nil // no counter yet
	}
	return counter, period, nil
}

// ResetCounter resets the counter for a given scope to 0.
// This is used for testing or manual correction.
func (c *NaturalKeyCounter) ResetCounter(ctx context.Context, tenantID, resource, field, scope, reset string) error {
	period := computePeriod(reset)
	_, err := c.db.ExecContext(ctx, `
		UPDATE forma_natural_key_counters
		SET counter = 0
		WHERE tenant_id = ? AND resource = ? AND field = ? AND scope = ? AND period = ?
	`, tenantID, resource, field, scope, period)
	return err
}

// computePeriod returns the period string based on reset strategy and current time.
func computePeriod(reset string) string {
	now := time.Now().UTC()
	switch reset {
	case "yearly":
		return fmt.Sprintf("%d", now.Year())
	case "monthly":
		return fmt.Sprintf("%d-%02d", now.Year(), now.Month())
	case "daily":
		return fmt.Sprintf("%d-%02d-%02d", now.Year(), now.Month(), now.Day())
	case "never", "":
		return ""
	default:
		return ""
	}
}

// renderFormat replaces placeholders in the format string.
func renderFormat(format string, counter int64, period, resource, field, prefix string) string {
	result := format

	// Replace {counter} variants
	result = strings.ReplaceAll(result, "{counter}", fmt.Sprintf("%d", counter))

	// Find and replace {counter:WIDTHd} patterns
	for i := 1; i <= 20; i++ {
		placeholder := fmt.Sprintf("{counter:0%dd}", i)
		replacement := fmt.Sprintf(fmt.Sprintf("%%0%dd", i), counter)
		result = strings.ReplaceAll(result, placeholder, replacement)
	}

	result = strings.ReplaceAll(result, "{period}", period)
	result = strings.ReplaceAll(result, "{year}", fmt.Sprintf("%d", time.Now().UTC().Year()))
	result = strings.ReplaceAll(result, "{month}", fmt.Sprintf("%02d", time.Now().UTC().Month()))
	result = strings.ReplaceAll(result, "{day}", fmt.Sprintf("%02d", time.Now().UTC().Day()))
	result = strings.ReplaceAll(result, "{resource}", resource)
	result = strings.ReplaceAll(result, "{field}", field)
	result = strings.ReplaceAll(result, "{prefix}", prefix)

	return result
}
