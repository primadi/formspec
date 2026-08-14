// Package db provides the database abstraction layer for FormSpec.
package db

import (
	"fmt"
)

// DefaultDevDSN is the default DSN for development mode.
const DefaultDevDSN = "sqlite://file:.formspec/data.db"

// Open opens a database connection based on the DSN.
// Supports two schemes:
//   - sqlite://file:.formspec/data.db (default for dev)
//   - postgres://user:pass@host:port/dbname?sslmode=require
func Open(dsn string) (DB, error) {
	cfg, err := ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	switch cfg.Driver {
	case DriverSQLite:
		return openSQLite(cfg)
	case DriverPostgres:
		return openPostgres(cfg)
	default:
		return nil, fmt.Errorf("db open: unsupported driver %q", cfg.Driver)
	}
}

// openSQLite opens a SQLite database.
func openSQLite(cfg *Config) (DB, error) {
	return OpenSQLite(cfg.Database, cfg.SQLitePragmas())
}

// openPostgres opens a PostgreSQL database.
func openPostgres(cfg *Config) (DB, error) {
	return OpenPostgres(cfg.PostgresConnString(), cfg.Schema)
}
