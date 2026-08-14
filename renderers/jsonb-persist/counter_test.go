package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestNaturalKeyCounter_NextSequence(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "counter_seq.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	c := NewNaturalKeyCounter(d, DriverSQLite)

	// First call → counter=1
	val1, period1, err := c.NextSequence(ctx, "tenant-1", "invoice", "number", "", "never")
	if err != nil {
		t.Fatalf("NextSequence #1 failed: %v", err)
	}
	if val1 != 1 {
		t.Errorf("expected counter=1, got %d", val1)
	}
	if period1 != "" {
		t.Errorf("expected empty period for never, got %q", period1)
	}

	// Second call → counter=2
	val2, _, err := c.NextSequence(ctx, "tenant-1", "invoice", "number", "", "never")
	if err != nil {
		t.Fatalf("NextSequence #2 failed: %v", err)
	}
	if val2 != 2 {
		t.Errorf("expected counter=2, got %d", val2)
	}

	// Third call → counter=3
	val3, _, err := c.NextSequence(ctx, "tenant-1", "invoice", "number", "", "never")
	if err != nil {
		t.Fatalf("NextSequence #3 failed: %v", err)
	}
	if val3 != 3 {
		t.Errorf("expected counter=3, got %d", val3)
	}
}

func TestNaturalKeyCounter_TenantIsolation(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "counter_tenant.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	c := NewNaturalKeyCounter(d, DriverSQLite)

	// Tenant A
	v1, _, _ := c.NextSequence(ctx, "tenant-a", "invoice", "number", "", "never")
	v2, _, _ := c.NextSequence(ctx, "tenant-a", "invoice", "number", "", "never")

	// Tenant B (starts at 1)
	v3, _, _ := c.NextSequence(ctx, "tenant-b", "invoice", "number", "", "never")

	if v1 != 1 || v2 != 2 {
		t.Errorf("tenant-a: expected 1,2 got %d,%d", v1, v2)
	}
	if v3 != 1 {
		t.Errorf("tenant-b: expected 1, got %d", v3)
	}
}

func TestNaturalKeyCounter_ScopeIsolation(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "counter_scope.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	c := NewNaturalKeyCounter(d, DriverSQLite)

	// Different resources
	v1, _, _ := c.NextSequence(ctx, "t1", "invoice", "number", "", "never")
	v2, _, _ := c.NextSequence(ctx, "t1", "customer", "code", "", "never")

	if v1 != 1 {
		t.Errorf("invoice: expected 1, got %d", v1)
	}
	if v2 != 1 {
		t.Errorf("customer: expected 1, got %d", v2)
	}
}

func TestNaturalKeyCounter_ResetPeriod(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "counter_period.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	c := NewNaturalKeyCounter(d, DriverSQLite)

	// Yearly — period should be current year
	v1, period, err := c.NextSequence(ctx, "t1", "invoice", "number", "", "yearly")
	if err != nil {
		t.Fatalf("NextSequence yearly failed: %v", err)
	}
	if v1 != 1 {
		t.Errorf("expected counter=1, got %d", v1)
	}
	if want := time.Now().Format("2006"); period != want {
		t.Errorf("expected period=%s, got %q", want, period)
	}

	// Monthly
	v2, period2, err := c.NextSequence(ctx, "t1", "invoice", "number", "", "monthly")
	if err != nil {
		t.Fatalf("NextSequence monthly failed: %v", err)
	}
	if v2 != 1 {
		t.Errorf("expected counter=1, got %d", v2)
	}
	if want := time.Now().Format("2006-01"); period2 != want {
		t.Errorf("expected period=%s, got %q", want, period2)
	}

	// Daily
	v3, period3, err := c.NextSequence(ctx, "t1", "invoice", "number", "", "daily")
	if err != nil {
		t.Fatalf("NextSequence daily failed: %v", err)
	}
	if v3 != 1 {
		t.Errorf("expected counter=1, got %d", v3)
	}
	if want := time.Now().Format("2006-01-02"); period3 != want {
		t.Errorf("expected period=%s, got %q", want, period3)
	}

	// Same period should continue counting
	v4, period4, _ := c.NextSequence(ctx, "t1", "invoice", "number", "", "yearly")
	if v4 != 2 {
		t.Errorf("expected counter=2 for same year, got %d", v4)
	}
	if want := time.Now().Format("2006"); period4 != want {
		t.Errorf("expected period=%s, got %q", want, period4)
	}
}

func TestNaturalKeyCounter_GenerateNaturalKey(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "counter_format.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	c := NewNaturalKeyCounter(d, DriverSQLite)

	tests := []struct {
		name     string
		resource string
		format   string
		reset    string
		prefix   string
		wantPat  string
	}{
		{
			name:     "default format",
			resource: "inv1",
			format:   "",
			reset:    "never",
			wantPat:  "--00001",
		},
		{
			name:     "custom with counter padding",
			resource: "inv2",
			format:   "INV-{counter:05d}",
			reset:    "never",
			wantPat:  "INV-00001",
		},
		{
			name:     "with period yearly",
			resource: "inv3",
			format:   "INV-{period}-{counter:04d}",
			reset:    "yearly",
			wantPat:  "INV-" + time.Now().Format("2006") + "-0001",
		},
		{
			name:     "with period monthly",
			resource: "inv4",
			format:   "ORD-{period}-{counter:03d}",
			reset:    "monthly",
			wantPat:  "ORD-" + time.Now().Format("2006-01") + "-001",
		},
		{
			name:     "with period daily",
			resource: "inv5",
			format:   "TXN-{period}-{counter:02d}",
			reset:    "daily",
			wantPat:  "TXN-" + time.Now().Format("2006-01-02") + "-01",
		},
		{
			name:     "with resource and field",
			resource: "invoice",
			format:   "{resource}-{field}-{counter:05d}",
			reset:    "never",
			wantPat:  "invoice-number-00001",
		},
		{
			// {seq...} is the placeholder name every manifest in this repo
			// actually uses (e.g. Clinic's visit.queue_number: "{prefix}{year}{month}{day}-{seq:03d}") —
			// must be supported as an alias of {counter...}, not just {counter...}.
			name:     "seq placeholder with prefix (visit.queue_number shape)",
			resource: "visit",
			format:   "{prefix}-{seq:03d}",
			reset:    "never",
			prefix:   "Q",
			wantPat:  "Q-001",
		},
		{
			name:     "bare seq placeholder",
			resource: "inv6",
			format:   "{seq}",
			reset:    "never",
			wantPat:  "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := c.GenerateNaturalKey(ctx, "t1", tt.resource, "number", "", tt.reset, tt.format, tt.prefix)
			if err != nil {
				t.Fatalf("GenerateNaturalKey failed: %v", err)
			}
			if key != tt.wantPat {
				t.Errorf("expected %q, got %q", tt.wantPat, key)
			}
		})
	}
}

func TestNaturalKeyCounter_GenerateNaturalKey_Sequential(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "counter_seq_format.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	c := NewNaturalKeyCounter(d, DriverSQLite)

	key1, _ := c.GenerateNaturalKey(ctx, "t1", "invoice", "number", "", "never", "INV-{counter:05d}", "")
	key2, _ := c.GenerateNaturalKey(ctx, "t1", "invoice", "number", "", "never", "INV-{counter:05d}", "")
	key3, _ := c.GenerateNaturalKey(ctx, "t1", "invoice", "number", "", "never", "INV-{counter:05d}", "")

	if key1 != "INV-00001" {
		t.Errorf("expected INV-00001, got %q", key1)
	}
	if key2 != "INV-00002" {
		t.Errorf("expected INV-00002, got %q", key2)
	}
	if key3 != "INV-00003" {
		t.Errorf("expected INV-00003, got %q", key3)
	}
}

func TestNaturalKeyCounter_PeekCounter(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "counter_peek.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	c := NewNaturalKeyCounter(d, DriverSQLite)

	// Peek before any insert → should be 0
	val, _, err := c.PeekCounter(ctx, "t1", "invoice", "number", "", "never")
	if err != nil {
		t.Fatalf("PeekCounter before insert failed: %v", err)
	}
	if val != 0 {
		t.Errorf("expected 0 before insert, got %d", val)
	}

	// Generate first key
	key1, _ := c.GenerateNaturalKey(ctx, "t1", "invoice", "number", "", "never", "INV-{counter:05d}", "")
	if key1 != "INV-00001" {
		t.Errorf("expected INV-00001, got %q", key1)
	}

	// Peek after first → should be 1
	val2, _, err := c.PeekCounter(ctx, "t1", "invoice", "number", "", "never")
	if err != nil {
		t.Fatalf("PeekCounter after insert failed: %v", err)
	}
	if val2 != 1 {
		t.Errorf("expected 1 after insert, got %d", val2)
	}
}

func TestNaturalKeyCounter_ResetCounter(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "counter_reset.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	r := NewMigrationRunner(d, DriverSQLite)
	ctx := context.Background()
	if err := r.EnsureSystemTables(ctx); err != nil {
		t.Fatalf("EnsureSystemTables failed: %v", err)
	}

	c := NewNaturalKeyCounter(d, DriverSQLite)

	// Generate some keys
	c.GenerateNaturalKey(ctx, "t1", "invoice", "number", "", "never", "INV-{counter:05d}", "")
	c.GenerateNaturalKey(ctx, "t1", "invoice", "number", "", "never", "INV-{counter:05d}", "")
	c.GenerateNaturalKey(ctx, "t1", "invoice", "number", "", "never", "INV-{counter:05d}", "")

	// Peek → should be 3
	val, _, _ := c.PeekCounter(ctx, "t1", "invoice", "number", "", "never")
	if val != 3 {
		t.Errorf("expected 3 before reset, got %d", val)
	}

	// Reset
	if err := c.ResetCounter(ctx, "t1", "invoice", "number", "", "never"); err != nil {
		t.Fatalf("ResetCounter failed: %v", err)
	}

	// Peek → should be 0
	val2, _, _ := c.PeekCounter(ctx, "t1", "invoice", "number", "", "never")
	if val2 != 0 {
		t.Errorf("expected 0 after reset, got %d", val2)
	}

	// Next should be 1
	key, _ := c.GenerateNaturalKey(ctx, "t1", "invoice", "number", "", "never", "INV-{counter:05d}", "")
	if key != "INV-00001" {
		t.Errorf("expected INV-00001 after reset, got %q", key)
	}
}
