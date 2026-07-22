package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func newScopeTestDB(t *testing.T, name string) DB {
	t.Helper()
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, name), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if _, err := d.ExecContext(context.Background(),
		`CREATE TABLE items (id text PRIMARY KEY, val text NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return d
}

func TestTxScope_JoinSameStoreReusesTransaction(t *testing.T) {
	d := newScopeTestDB(t, "scope_reuse.db")
	ctx := context.Background()

	// SQLite here is configured with exactly one physical connection
	// (SetMaxOpenConns(1) — see sqlite_db.go), so this test deliberately
	// never queries `d` directly while a transaction from `d` is open —
	// that would try to check out a second connection from a pool with
	// none free and deadlock. Every read/write while the scope's
	// transaction is open goes through the txdb the scope itself handed
	// back, exactly like production code must.
	scope := NewTxScope()
	txdb1, err := scope.join(ctx, d)
	if err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	if _, err := txdb1.ExecContext(ctx, `INSERT INTO items (id, val) VALUES ('1', 'a')`); err != nil {
		t.Fatalf("insert via first join: %v", err)
	}

	txdb2, err := scope.join(ctx, d)
	if err != nil {
		t.Fatalf("second join (same store) failed: %v", err)
	}
	if _, err := txdb2.ExecContext(ctx, `INSERT INTO items (id, val) VALUES ('2', 'b')`); err != nil {
		t.Fatalf("insert via second join: %v", err)
	}

	// Rolling back must undo BOTH inserts — proving the two joins shared
	// one transaction rather than each auto-committing independently.
	if err := scope.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	var count int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&count); err != nil {
		t.Fatalf("count after rollback: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows after rollback (both inserts shared one transaction), got %d", count)
	}

	// Re-run and commit this time — both rows should land.
	scope2 := NewTxScope()
	txdb3, err := scope2.join(ctx, d)
	if err != nil {
		t.Fatalf("join on fresh scope: %v", err)
	}
	if _, err := txdb3.ExecContext(ctx, `INSERT INTO items (id, val) VALUES ('1', 'a')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := txdb3.ExecContext(ctx, `INSERT INTO items (id, val) VALUES ('2', 'b')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := scope2.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&count); err != nil {
		t.Fatalf("count after commit: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows visible after commit, got %d", count)
	}
}

func TestTxScope_JoinDifferentStoreErrors(t *testing.T) {
	dA := newScopeTestDB(t, "scope_cross_a.db")
	dB := newScopeTestDB(t, "scope_cross_b.db")
	ctx := context.Background()

	scope := NewTxScope()
	if _, err := scope.join(ctx, dA); err != nil {
		t.Fatalf("join store A: %v", err)
	}

	_, err := scope.join(ctx, dB)
	if err == nil {
		t.Fatal("expected ErrCrossStoreTx joining a different store, got nil")
	}
	if !errors.Is(err, ErrCrossStoreTx) {
		t.Fatalf("expected ErrCrossStoreTx, got %v", err)
	}

	_ = scope.Rollback()
}

func TestTxScope_CommitRollbackNoopWhenNeverOpened(t *testing.T) {
	scope := NewTxScope()
	if err := scope.Commit(); err != nil {
		t.Fatalf("Commit on never-opened scope should be a no-op, got %v", err)
	}
	scope2 := NewTxScope()
	if err := scope2.Rollback(); err != nil {
		t.Fatalf("Rollback on never-opened scope should be a no-op, got %v", err)
	}
}

func TestTxScope_PeekBehavior(t *testing.T) {
	dA := newScopeTestDB(t, "scope_peek_a.db")
	dB := newScopeTestDB(t, "scope_peek_b.db")
	ctx := context.Background()

	scope := NewTxScope()

	if _, ok := scope.peek(dA); ok {
		t.Fatal("peek before any join should return ok=false")
	}

	txdb, err := scope.join(ctx, dA)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	if got, ok := scope.peek(dA); !ok || got != txdb {
		t.Fatalf("peek(dA) after join should return the same txdb, ok=%v", ok)
	}
	if _, ok := scope.peek(dB); ok {
		t.Fatal("peek(dB) should return ok=false — different store than the one joined")
	}

	_ = scope.Rollback()
}

func TestScopeRegistry_RoundTrip(t *testing.T) {
	scope := NewTxScope()
	id := RegisterScope(scope)
	if id == "" {
		t.Fatal("RegisterScope returned empty id")
	}

	got, ok := LookupScope(id)
	if !ok || got != scope {
		t.Fatalf("LookupScope(%q) = %v, %v — want the registered scope, true", id, got, ok)
	}

	UnregisterScope(id)
	if _, ok := LookupScope(id); ok {
		t.Fatal("LookupScope should fail after UnregisterScope")
	}
}

func TestTxScope_ContextRoundTrip(t *testing.T) {
	scope := NewTxScope()
	ctx := WithTxScope(context.Background(), scope, "scope-123")

	if got := TxScopeFromContext(ctx); got != scope {
		t.Fatalf("TxScopeFromContext = %v, want %v", got, scope)
	}
	if id := ScopeIDFromContext(ctx); id != "scope-123" {
		t.Fatalf("ScopeIDFromContext = %q, want %q", id, "scope-123")
	}

	if got := TxScopeFromContext(context.Background()); got != nil {
		t.Fatalf("TxScopeFromContext on a plain context should be nil, got %v", got)
	}
}
