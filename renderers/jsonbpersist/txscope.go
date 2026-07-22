package db

import (
	"context"
	"fmt"
	"sync"
)

// TxScope tracks at most one open transaction for one action execution
// (a custom action's Dispatch, and everything it calls into — scripts,
// native handlers, sidecar callbacks) and which physical store it was
// opened against.
//
// The scope is joined by *store identity*, never module identity: this
// codebase's current architecture gives every EntityStore the exact same
// underlying DB (no per-module Datastore exists yet — Fase 2.9), so a
// custom action touching several modules joins the same transaction fine.
// The only thing that must error is an attempt to share one SQL
// transaction across two genuinely different connection pools — that is
// physically impossible, and per docs/spec/backend/01-core-basic.md §3,
// cross-Datastore interaction must go through the outbox, never a shared
// transaction.
type TxScope struct {
	mu   sync.Mutex
	base DB // nil until first join; the store all participants share
	tx   Tx
	txdb DB
}

// NewTxScope creates an empty scope. Its store is established lazily by
// whichever entity's join call happens first.
func NewTxScope() *TxScope {
	return &TxScope{}
}

// Peek is the exported form of peek, for callers outside this package
// (e.g. internal/api's HandleCustomAction, deciding whether a durable
// event's outbox enqueue can ride the scope's open transaction).
func (s *TxScope) Peek(base DB) (DB, bool) {
	return s.peek(base)
}

// join is the WRITE path: lazily opens a transaction on base on first
// call; a later call with the SAME base reuses it. A call with a
// DIFFERENT base returns ErrCrossStoreTx.
func (s *TxScope) join(ctx context.Context, base DB) (DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.base == nil {
		tx, err := base.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("open request-scoped transaction: %w", err)
		}
		s.base = base
		s.tx = tx
		s.txdb = &txDB{base: base, tx: tx}
		return s.txdb, nil
	}

	if s.base != base {
		return nil, fmt.Errorf("%w: a transaction is already open on a different store — cross-store mutation must go through the outbox, never a shared transaction", ErrCrossStoreTx)
	}

	return s.txdb, nil
}

// peek is the READ path and the outbox-enqueue decision: non-mutating,
// never opens a transaction, never errors. Returns the open transaction's
// DB only if one exists AND was opened against the same base.
func (s *TxScope) peek(base DB) (DB, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.base == nil || s.base != base {
		return nil, false
	}
	return s.txdb, true
}

// Commit commits the scope's transaction. No-op if one was never opened.
func (s *TxScope) Commit() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tx == nil {
		return nil
	}
	return s.tx.Commit()
}

// Rollback rolls back the scope's transaction. No-op if one was never opened.
func (s *TxScope) Rollback() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tx == nil {
		return nil
	}
	return s.tx.Rollback()
}

// ErrCrossStoreTx is returned by join when a second, genuinely different
// store attempts to join a transaction already open on another store.
var ErrCrossStoreTx = fmt.Errorf("cross-store transaction")

// ─── context propagation ───

type txScopeCtxValue struct {
	scope *TxScope
	id    string
}

type txScopeCtxKey struct{}

// WithTxScope attaches scope (and its registry id, "" if not registered)
// to ctx — the same context.WithValue pattern already used by
// internal/api/handler.go's WithIdentity/WithWorkspace.
func WithTxScope(ctx context.Context, scope *TxScope, id string) context.Context {
	return context.WithValue(ctx, txScopeCtxKey{}, txScopeCtxValue{scope: scope, id: id})
}

// TxScopeFromContext extracts the active TxScope, or nil if none is set.
func TxScopeFromContext(ctx context.Context) *TxScope {
	v, _ := ctx.Value(txScopeCtxKey{}).(txScopeCtxValue)
	return v.scope
}

// ScopeIDFromContext extracts the active scope's registry id, or "" if
// none is set — used by the sidecar executor to forward the id across
// the process boundary (see scopeRegistry below).
func ScopeIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(txScopeCtxKey{}).(txScopeCtxValue)
	return v.id
}

// ─── process-local scope registry (sidecar correlation) ───
//
// The sidecar protocol is two separate HTTP round-trips — the outbound
// POST {app-endpoint}/invoke/... call and the app process's inbound
// POST /ctx/entity/{op} callback — both served by the same OS process
// (confirmed in cmd/forma/dev.go: one `forma dev` binary runs both the
// SidecarExecutor's outbound client and the CtxHandler's inbound
// listener). A context.Context value cannot cross that process boundary,
// so a generated id + this in-memory map reconstructs the same effect:
// the app process echoes the id back on its callback, and the callback
// handler looks up the live *TxScope to resume the same transaction.

var scopeRegistry sync.Map // string -> *TxScope

// RegisterScope makes scope reachable by a generated id, for the duration
// of one action execution. Callers must UnregisterScope when done.
func RegisterScope(scope *TxScope) string {
	id := NewUUIDv7()
	scopeRegistry.Store(id, scope)
	return id
}

// LookupScope resolves a previously registered scope id.
func LookupScope(id string) (*TxScope, bool) {
	v, ok := scopeRegistry.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*TxScope), true
}

// UnregisterScope removes a previously registered scope id.
func UnregisterScope(id string) {
	scopeRegistry.Delete(id)
}
