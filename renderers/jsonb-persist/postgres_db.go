package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresDB implements DB for PostgreSQL.
type PostgresDB struct {
	db     *sql.DB
	dsn    string
	schema string
}

// pgTx implements Tx for PostgreSQL.
type pgTx struct {
	tx *sql.Tx
}

func (t *pgTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, pgRewritePlaceholders(query), args...)
}

func (t *pgTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, pgRewritePlaceholders(query), args...)
}

func (t *pgTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, pgRewritePlaceholders(query), args...)
}

func (t *pgTx) Commit() error   { return t.tx.Commit() }
func (t *pgTx) Rollback() error { return t.tx.Rollback() }

// OpenPostgres opens a new PostgreSQL database connection.
func OpenPostgres(connString, schema string) (DB, error) {
	sqldb, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}

	sqldb.SetMaxOpenConns(25)
	sqldb.SetMaxIdleConns(10)

	p := &PostgresDB{db: sqldb, dsn: connString, schema: schema}

	if err := p.Ping(context.Background()); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	return p, nil
}

// pgRewritePlaceholders rewrites SQLite-style `?` placeholders to PostgreSQL's
// `$n`. The whole persist backend was written against the SQLite driver and
// uses `?` throughout; pgx does not translate them, so the first real
// PostgreSQL run failed with "syntax error at or near ','" at the first
// parameterized statement (master todo 15.8).
//
// Quoted string literals are passed through untouched, and the rewriting is
// positional in statement order, so pgx's statement cache keeps working (the
// rewritten SQL for a given input is stable).
//
// The jsonb existence operator (`data ? 'k'`) is NOT a placeholder — callers
// must use its functional form jsonb_exists(data, 'k') instead; the operator's
// `?` would be rewritten into a parameter and corrupt the SQL.
func pgRewritePlaceholders(query string) string {
	if !strings.ContainsRune(query, '?') {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 16)
	n := 0
	inString := false
	for i := 0; i < len(query); i++ {
		c := query[i]
		if inString {
			b.WriteByte(c)
			if c == '\'' {
				// '' inside a quoted string is an escaped quote, not the end.
				if i+1 < len(query) && query[i+1] == '\'' {
					b.WriteByte('\'')
					i++
				} else {
					inString = false
				}
			}
			continue
		}
		if c == '\'' {
			inString = true
			b.WriteByte(c)
			continue
		}
		if c == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func (p *PostgresDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return p.db.ExecContext(ctx, pgRewritePlaceholders(query), args...)
}

func (p *PostgresDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, pgRewritePlaceholders(query), args...)
}

func (p *PostgresDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.db.QueryRowContext(ctx, pgRewritePlaceholders(query), args...)
}

func (p *PostgresDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := p.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("postgres begin tx: %w", err)
	}
	return &pgTx{tx: tx}, nil
}

func (p *PostgresDB) Close() error                   { return p.db.Close() }
func (p *PostgresDB) Ping(ctx context.Context) error { return p.db.PingContext(ctx) }
func (p *PostgresDB) DSN() string                    { return p.dsn }
func (p *PostgresDB) DriverName() string             { return "postgres" }

func (p *PostgresDB) HasTable(ctx context.Context, schema, table string) (bool, error) {
	if schema == "" {
		schema = p.schema
	}
	var count int
	err := p.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=$1 AND table_name=$2",
		schema, table,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("postgres has_table: %w", err)
	}
	return count > 0, nil
}

func (p *PostgresDB) Driver() *sql.DB { return p.db }
