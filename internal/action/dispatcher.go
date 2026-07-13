// Package action provides the central action execution engine for Forma.
//
// It dispatches action requests (from HTTP, events, or internal calls) to the
// appropriate executor based on the action's impl type (native, script_ref,
// script, compiled, sidecar). The dispatcher also enforces state-machine
// transitions and evaluates guard conditions before executing.
//
// Architecture:
//
//	HTTP Handler → Dispatcher.Dispatch() → Executor.Execute()
//	                                        ├── ScriptExecutor (Starlark)
//	                                        ├── NativeExecutor (Go)
//	                                        ├── CompiledExecutor (WASM — deferred)
//	                                        └── SidecarExecutor (external — deferred)
package action

import (
	"context"
	"fmt"
	"time"

	"github.com/primadi/forma/pkg/spec"
)

// ─── Types ───

// ExecuteParams carries everything an action execution needs.
type ExecuteParams struct {
	// Module is the owning module (e.g. "billing").
	Module string
	// Entity is the entity or service name (e.g. "order").
	Entity string
	// ActionName is the action being invoked (e.g. "checkout").
	ActionName string
	// ResourceID is the entity record ID for entity actions; empty for service actions.
	ResourceID string
	// Resource is the current entity record data (for entity actions).
	Resource map[string]any
	// ResourceVersion is the current known version of the record (for entity
	// actions), threaded through so script/native saves can use real
	// optimistic-concurrency (CAS) instead of a hardcoded version.
	ResourceVersion int
	// Params are the action parameters from the request body.
	Params map[string]any
	// WorkspaceID is the current workspace identifier.
	WorkspaceID string
	// UserID is the authenticated user.
	UserID string
	// Identity carries the full auth identity (user, workspace, permissions, roles).
	Identity *IdentityInfo
	// Ctx carries runtime context primitives available to scripts.
	RuntimeCtx *RuntimeContext
}

// IdentityInfo mirrors auth.Identity for the action package (no import cycle).
type IdentityInfo struct {
	UserID      string
	WorkspaceID string
	Permissions []string
	Roles       []string
}

// ExecuteResult is the outcome of an action execution.
type ExecuteResult struct {
	// Data is the action's return value (may be nil).
	Data any
	// NewState is the new state-machine state after transition, if any.
	NewState string
	// Events contains event declarations that should be emitted post-execution.
	Events []EventEmission
	// JobID is set for async actions (call: async).
	JobID string
}

// EventEmission is a pending event publication after an action.
type EventEmission struct {
	Name      string
	Durable   bool
	Payload   map[string]any
	DeliverTo []spec.EventDeliveryDecl
}

// ─── Executor Interface ───

// Executor executes an action's business logic. Each impl type has its own executor.
type Executor interface {
	// Execute runs the action's business logic and returns the result.
	Execute(ctx context.Context, action spec.Action, params ExecuteParams) (*ExecuteResult, error)
}

// ─── Dispatcher ───

// Dispatcher routes action invocations to the correct Executor based on impl type.
type Dispatcher struct {
	executors map[spec.ImplType]Executor
	nativeEx  *NativeExecutor // exposed via NativeExecutor() for public registration
}

// NativeExecutor returns the native executor, allowing external callers to
// register Go handlers through App.RegisterNative without importing internal/.
func (d *Dispatcher) NativeExecutor() *NativeExecutor { return d.nativeEx }

// SetNativeExecutor stores the native executor reference so that
// Dispatcher.NativeExecutor() returns the same instance used for dispatch.
// Called by resource/forma.go's newDispatcher after registering it.
func (d *Dispatcher) SetNativeExecutor(ex *NativeExecutor) { d.nativeEx = ex }

// NewDispatcher creates an action dispatcher with no executors registered.
// Use RegisterExecutor to add executors for each impl type.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		executors: make(map[spec.ImplType]Executor),
	}
}

// RegisterExecutor registers an executor for a specific impl type.
// Panics if an executor for that type is already registered.
func (d *Dispatcher) RegisterExecutor(implType spec.ImplType, executor Executor) {
	if _, ok := d.executors[implType]; ok {
		panic(fmt.Sprintf("executor for %q already registered", implType))
	}
	d.executors[implType] = executor
}

// HasExecutor returns true if an executor is registered for the given impl type.
func (d *Dispatcher) HasExecutor(implType spec.ImplType) bool {
	_, ok := d.executors[implType]
	return ok
}

// Dispatch routes an action to the appropriate executor based on impl type.
//
// Resolution priority (Core Basic §6.1):
//
//	native > compiled > sidecar > script_ref > script
//
// If the action has no impl, it's a no-op (e.g. standard CRUD handled by the
// framework layer below the dispatcher).
func (d *Dispatcher) Dispatch(ctx context.Context, action spec.Action, params ExecuteParams) (*ExecuteResult, error) {
	if action.Impl == nil {
		return &ExecuteResult{}, nil
	}

	executor, ok := d.executors[action.Impl.Type]
	if !ok {
		return nil, fmt.Errorf("no executor registered for impl type %q (action %s.%s)",
			action.Impl.Type, params.Module, params.ActionName)
	}

	start := time.Now()
	result, err := executor.Execute(ctx, action, params)
	elapsed := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("action %s.%s (%s, %v): %w",
			params.Module, params.ActionName, action.Impl.Type, elapsed, err)
	}

	return result, nil
}

// ─── RuntimeContext ───

// RuntimeContext provides access to ctx.* primitives for script execution.
// This is the concrete implementation of what scripts see as `ctx`.
type RuntimeContext struct {
	Workspace *WorkspaceInfo
	User      *UserInfo
	Auth      *AuthInfo
	Now       func() time.Time
	Logger    RuntimeLogger
	NextKey   func(fieldName string) (string, error)
	// DB, Cache, Lock, Queue, PubSub, Storage are injected by the dispatcher.
	DB interface {
		Load(entity, id string) (map[string]any, error)
	}
	Config interface{ Get(key string) (any, error) }
	Lock   interface {
		Acquire(name string) error
		Release(name string) error
	}
}

// WorkspaceInfo is exposed as ctx.workspace in scripts.
type WorkspaceInfo struct {
	ID   string
	Name string
}

// UserInfo is exposed as ctx.user in scripts.
type UserInfo struct {
	ID          string
	Role        string
	Permissions []string
}

// AuthInfo is exposed as ctx.auth in scripts.
type AuthInfo struct{}

// RuntimeLogger is the logger exposed to scripts as ctx.log.
type RuntimeLogger interface {
	Info(event string, meta map[string]any)
	Warn(event string, meta map[string]any)
	Error(event string, meta map[string]any)
}

// ─── Default Logger ───

type defaultLogger struct{}

func (defaultLogger) Info(event string, meta map[string]any)  {}
func (defaultLogger) Warn(event string, meta map[string]any)  {}
func (defaultLogger) Error(event string, meta map[string]any) {}

// DefaultLogger is a no-op logger used when no other logger is configured.
var DefaultLogger RuntimeLogger = defaultLogger{}
