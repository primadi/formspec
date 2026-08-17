package datastore

import (
	"context"
	"path/filepath"
	"testing"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// TestDBQuerier_QueryAgainstSQLite proves the DBQuerier adapter runs a real
// query against the app's primary database (SQLite here) and returns rows as
// column→value maps — the backend for ctx.db().query() (todo 2.9.1).
func TestDBQuerier_QueryAgainstSQLite(t *testing.T) {
	database, err := db.OpenSQLite(filepath.Join(t.TempDir(), "q.db"), nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if _, err := database.ExecContext(context.Background(), "CREATE TABLE t (id INTEGER, name TEXT)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := database.ExecContext(context.Background(), "INSERT INTO t (id, name) VALUES (1, 'a'), (2, 'b')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	q := &DBQuerier{DB: database}
	rows, err := q.Query(context.Background(), "SELECT id, name FROM t ORDER BY id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0]["id"] != int64(1) || rows[0]["name"] != "a" {
		t.Fatalf("row0 = %v, want {id:1 name:a}", rows[0])
	}
	if rows[1]["id"] != int64(2) || rows[1]["name"] != "b" {
		t.Fatalf("row1 = %v, want {id:2 name:b}", rows[1])
	}
}

// TestDBQuerier_QueryError propagates SQL errors.
func TestDBQuerier_QueryError(t *testing.T) {
	database, err := db.OpenSQLite(filepath.Join(t.TempDir(), "q.db"), nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	q := &DBQuerier{DB: database}
	if _, err := q.Query(context.Background(), "SELECT * FROM does_not_exist"); err == nil {
		t.Fatalf("expected error for missing table, got nil")
	}
}
