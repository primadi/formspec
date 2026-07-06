// Package db provides the database abstraction layer for Forma.
//
// It defines the DB interface that all storage operations use, supports
// SQLite (development) and PostgreSQL (production), and provides a factory
// function to create the appropriate implementation from a DSN string.
package db

import (
	"context"
	"database/sql"
)

// DB is the core database interface for Forma.
// It wraps database/sql with Forma-specific convenience methods
// and ensures consistent behavior across SQLite and PostgreSQL.
type DB interface {
	// ExecContext executes a query without returning rows.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)

	// QueryContext executes a query that returns rows.
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)

	// QueryRowContext executes a query that returns at most one row.
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row

	// BeginTx starts a transaction with the given options.
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)

	// Close closes the database connection.
	Close() error

	// Ping verifies the database connection is alive.
	Ping(ctx context.Context) error

	// DSN returns the connection string (sanitized).
	DSN() string

	// DriverName returns "sqlite" or "postgres".
	DriverName() string

	// HasTable checks if a table exists in the database.
	HasTable(ctx context.Context, schema, table string) (bool, error)

	// Driver exposes the underlying *sql.DB for migration tools.
	Driver() *sql.DB
}

// Tx wraps a database transaction.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	Commit() error
	Rollback() error
}
