package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// SQLiteDB implements DB for SQLite.
type SQLiteDB struct {
	mu     sync.RWMutex
	db     *sql.DB
	dsn    string
	dbPath string
}

// sqliteTx implements Tx for SQLite.
type sqliteTx struct {
	tx *sql.Tx
}

func (t *sqliteTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *sqliteTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *sqliteTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *sqliteTx) Commit() error   { return t.tx.Commit() }
func (t *sqliteTx) Rollback() error { return t.tx.Rollback() }

// OpenSQLite opens a new SQLite database connection.
func OpenSQLite(dbPath string, extraPragmas map[string]string) (DB, error) {
	dsn := buildSQLiteDSN(dbPath, extraPragmas)

	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	sqldb.SetMaxOpenConns(1)
	sqldb.SetMaxIdleConns(1)

	s := &SQLiteDB{db: sqldb, dsn: dsn, dbPath: dbPath}

	if err := s.applyPragmas(context.Background()); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("sqlite pragmas: %w", err)
	}

	return s, nil
}

// buildSQLiteDSN constructs the SQLite DSN with FormSpec-required pragmas.
// Uses a simple file: URI format that modernc.org/sqlite accepts.
func buildSQLiteDSN(dbPath string, extraPragmas map[string]string) string {
	var params []string

	params = append(params, "_pragma=journal_mode(WAL)")
	params = append(params, "_pragma=foreign_keys(ON)")
	params = append(params, "_pragma=busy_timeout(5000)")
	params = append(params, "_pragma=case_sensitive_like(OFF)")
	params = append(params, "_pragma=cache_size(-32000)")

	for k, v := range extraPragmas {
		if strings.HasPrefix(k, "_pragma") {
			params = append(params, k+"="+v)
		}
	}

	// Build a file: URI manually to avoid url.URL encoding issues with relative paths
	// modernc.org/sqlite expects: file:path?params  or  file:///absolute/path?params
	var dsn string
	if strings.HasPrefix(dbPath, "/") {
		dsn = "file://" + dbPath
	} else {
		dsn = "file:" + dbPath
	}

	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}
	return dsn
}

func (s *SQLiteDB) applyPragmas(ctx context.Context) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := s.db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}

func (s *SQLiteDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

func (s *SQLiteDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s *SQLiteDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *SQLiteDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("sqlite begin tx: %w", err)
	}
	return &sqliteTx{tx: tx}, nil
}

func (s *SQLiteDB) Close() error                   { return s.db.Close() }
func (s *SQLiteDB) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *SQLiteDB) DSN() string                    { return s.dsn }
func (s *SQLiteDB) DriverName() string             { return "sqlite" }

func (s *SQLiteDB) HasTable(ctx context.Context, schema, table string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
		table,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("sqlite has_table: %w", err)
	}
	return count > 0, nil
}

func (s *SQLiteDB) Driver() *sql.DB { return s.db }
