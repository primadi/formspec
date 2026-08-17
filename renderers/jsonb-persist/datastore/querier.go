package datastore

import (
	"context"

	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// Querier is the capability interface a ctx.db() connection must implement
// to serve ctx.db().query(sql). It mirrors internal/starlark's Querier and
// internal/sidecar's Querier — Go interfaces are structural, so this adapter
// satisfies all three without importing them.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error)
}

// DBQuerier adapts a db.DB (the app's primary database) to the Querier
// capability interface. It is what the ctx.* resolver returns for the "db"
// primitive so Starlark scripts can run ctx.db().query(...) against the
// app's primary datastore (SQLite in dev, Postgres in prod).
type DBQuerier struct {
	DB db.DB
}

// Query executes a read-only SQL statement and returns the rows as a slice
// of column→value maps. Values are returned as their raw database/sql scan
// types (int64, float64, string, []byte, time.Time, nil); callers convert to
// Starlark via toStarlark.
func (q *DBQuerier) Query(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := q.DB.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = vals[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
