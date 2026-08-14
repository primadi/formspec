package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_SQLite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	d, err := OpenSQLite(dbPath, nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	if d.DriverName() != "sqlite" {
		t.Errorf("expected sqlite, got %s", d.DriverName())
	}

	if err := d.Ping(context.Background()); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestOpen_DefaultDevDSN(t *testing.T) {
	// Test that the default DSN parses correctly
	cfg, err := ParseDSN(".formspec/data.db")
	if err != nil {
		t.Fatalf("ParseDSN failed: %v", err)
	}
	if cfg.Driver != DriverSQLite {
		t.Errorf("expected sqlite, got %s", cfg.Driver)
	}
}

func TestOpen_InvalidDSN(t *testing.T) {
	_, err := Open("mysql://user:pass@localhost/db")
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestSQLite_Ping(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "ping.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	if err := d.Ping(context.Background()); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestSQLite_ExecAndQuery(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "exec.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	ctx := context.Background()

	_, err = d.ExecContext(ctx, "CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Create table failed: %v", err)
	}

	_, err = d.ExecContext(ctx, "INSERT INTO test_table (name) VALUES (?)", "hello")
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	rows, err := d.QueryContext(ctx, "SELECT id, name FROM test_table")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected at least one row")
	}
	var id int
	var name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if name != "hello" {
		t.Errorf("expected 'hello', got %s", name)
	}
}

func TestSQLite_HasTable(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "hastable.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	ctx := context.Background()

	exists, err := d.HasTable(ctx, "", "test_table")
	if err != nil {
		t.Fatalf("HasTable failed: %v", err)
	}
	if exists {
		t.Error("expected table to not exist yet")
	}

	_, err = d.ExecContext(ctx, "CREATE TABLE test_table (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Fatalf("Create table failed: %v", err)
	}

	exists, err = d.HasTable(ctx, "", "test_table")
	if err != nil {
		t.Fatalf("HasTable failed: %v", err)
	}
	if !exists {
		t.Error("expected table to exist after creation")
	}
}

func TestSQLite_TransactionCommit(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "txcommit.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	ctx := context.Background()

	_, err = d.ExecContext(ctx, "CREATE TABLE tx_test (id INTEGER PRIMARY KEY, val TEXT)")
	if err != nil {
		t.Fatalf("Create table failed: %v", err)
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO tx_test (val) VALUES (?)", "tx-value")
	if err != nil {
		tx.Rollback()
		t.Fatalf("Insert in tx failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	var val string
	err = d.QueryRowContext(ctx, "SELECT val FROM tx_test WHERE id=1").Scan(&val)
	if err != nil {
		t.Fatalf("Query after commit failed: %v", err)
	}
	if val != "tx-value" {
		t.Errorf("expected 'tx-value', got %s", val)
	}
}

func TestSQLite_TransactionRollback(t *testing.T) {
	dir := t.TempDir()
	d, err := OpenSQLite(filepath.Join(dir, "txrollback.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLite failed: %v", err)
	}
	defer d.Close()

	ctx := context.Background()

	_, err = d.ExecContext(ctx, "CREATE TABLE tx_test (id INTEGER PRIMARY KEY, val TEXT)")
	if err != nil {
		t.Fatalf("Create table failed: %v", err)
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO tx_test (val) VALUES (?)", "rollback-val")
	if err != nil {
		t.Fatalf("Insert in tx failed: %v", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	var count int
	err = d.QueryRowContext(ctx, "SELECT COUNT(*) FROM tx_test").Scan(&count)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

func TestSQLite_DefaultDevDSN(t *testing.T) {
	cfg, err := ParseDSN(".formspec/data.db")
	if err != nil {
		t.Fatalf("ParseDSN failed: %v", err)
	}
	if cfg.Driver != DriverSQLite {
		t.Errorf("expected sqlite, got %s", cfg.Driver)
	}
	if cfg.Database != ".formspec/data.db" {
		t.Errorf("expected .formspec/data.db, got %s", cfg.Database)
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
