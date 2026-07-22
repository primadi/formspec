package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// NewUUIDv7 returns a new UUID v7 (time-ordered) string. Primary keys are
// generated here, at the app layer, so SQLite and PostgreSQL share the same
// value space — Core Basic §2 mandates UUID v7 PKs with no per-backend
// exception.
func NewUUIDv7() string {
	id, err := uuid.NewV7()
	if err != nil {
		// NewV7 only fails when the entropy source does; a random v4 keeps
		// the write alive at the cost of time-ordering for this one value.
		return uuid.NewString()
	}
	return id.String()
}

// txDB adapts an open Tx to the DB interface so store code (EntityStore,
// ChildStore, NaturalKeyCounter, OutboxStore, ...) runs unchanged inside a
// transaction: statement execution goes through the transaction, metadata
// methods delegate to the base DB.
type txDB struct {
	base DB
	tx   Tx
}

func (t *txDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *txDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *txDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

// BeginTx on an already-transaction-bound DB joins the open transaction:
// the returned Tx executes on the same transaction, and its Commit/Rollback
// are no-ops because the outermost InTx owns the transaction boundary.
func (t *txDB) BeginTx(_ context.Context, _ *sql.TxOptions) (Tx, error) {
	return joinedTx{t.tx}, nil
}

func (t *txDB) Close() error                   { return t.base.Close() }
func (t *txDB) Ping(ctx context.Context) error { return t.base.Ping(ctx) }
func (t *txDB) DSN() string                    { return t.base.DSN() }
func (t *txDB) DriverName() string             { return t.base.DriverName() }
func (t *txDB) Driver() *sql.DB                { return t.base.Driver() }

func (t *txDB) HasTable(ctx context.Context, schema, table string) (bool, error) {
	return t.base.HasTable(ctx, schema, table)
}

// joinedTx wraps the outer transaction for a nested BeginTx caller. Rollback
// intentionally does nothing — with InTx, rollback happens by returning an
// error to the outermost call, never by a nested participant.
type joinedTx struct{ Tx }

func (joinedTx) Commit() error   { return nil }
func (joinedTx) Rollback() error { return nil }

// runTx runs fn against either the request-scoped transaction carried on
// ctx (if one exists — joining it via TxScope.join, which errors on a
// genuine cross-store attempt) or a fresh self-contained transaction on
// base otherwise (today's InTx behavior, unchanged when no scope is
// present). This is the single entry point EntityStore's write methods
// use so each doesn't duplicate the scope-or-InTx branch.
func runTx(ctx context.Context, base DB, fn func(txdb DB) error) error {
	if scope := TxScopeFromContext(ctx); scope != nil {
		txdb, err := scope.join(ctx, base)
		if err != nil {
			return err
		}
		return fn(txdb)
	}
	return InTx(ctx, base, fn)
}

// writeDB resolves which DB a single-statement write (UpdateFields,
// IncrementField, DecrementField — already atomic as one SQL statement,
// so no InTx wrapping needed on their own) should target: the
// request-scoped transaction if one is active (joining it — erroring on a
// genuine cross-store mismatch, same as runTx), else base itself,
// unchanged from today's behavior.
func writeDB(ctx context.Context, base DB) (DB, error) {
	if scope := TxScopeFromContext(ctx); scope != nil {
		return scope.join(ctx, base)
	}
	return base, nil
}

// txReadDB returns the request-scoped transaction's DB if one is active
// for base (read-your-own-writes, and — on a single-connection driver
// like SQLite — avoiding a deadlock against a pool with no free
// connections), else base itself, unchanged from today's behavior.
func txReadDB(ctx context.Context, base DB) DB {
	if scope := TxScopeFromContext(ctx); scope != nil {
		if txdb, ok := scope.peek(base); ok {
			return txdb
		}
	}
	return base
}

// InTx runs fn inside a single database transaction. fn receives a DB whose
// statements all execute on that transaction; returning an error rolls
// everything back, returning nil commits. If base is itself already
// transaction-bound (a nested InTx call), fn simply joins the open
// transaction and the outermost call keeps ownership of Commit/Rollback —
// so store methods that wrap themselves in InTx compose into larger
// transactions for free.
func InTx(ctx context.Context, base DB, fn func(txdb DB) error) error {
	if _, ok := base.(*txDB); ok {
		return fn(base)
	}
	tx, err := base.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	w := &txDB{base: base, tx: tx}
	if err := fn(w); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
