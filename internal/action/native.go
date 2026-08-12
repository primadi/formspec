package action

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/primadi/formspec/pkg/spec"
)

// NativeHandler is a Go function that implements a native action.
// It receives the execution context and action parameters.
// Returns the result data or an error.
type NativeHandler func(ctx context.Context, params ExecuteParams) (any, error)

// NativeExecutor executes actions with impl: { type: native }.
// Handlers are registered explicitly via RegisterNativeHandler.
type NativeExecutor struct {
	mu       sync.RWMutex
	handlers map[string]NativeHandler // key = "Module.Entity.Action" or "TypeName.MethodName"
}

// NewNativeExecutor creates a NativeExecutor with an empty handler registry.
func NewNativeExecutor() *NativeExecutor {
	return &NativeExecutor{
		handlers: make(map[string]NativeHandler),
	}
}

// Register registers a native handler.
//
// The key can be either:
//   - "{module}.{entity}.{action}" (e.g. "billing.order.update-discount-rule")
//   - "{TypeName}.{MethodName}" (e.g. "OrderResource.UpdateDiscountRule")
//
// The latter is the ref format used in impl: { type: native, ref: "OrderResource.UpdateDiscountRule" }.
//
// Panics if a handler is already registered for the given key.
func (e *NativeExecutor) Register(key string, handler NativeHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.handlers[key]; ok {
		panic(fmt.Sprintf("native handler %q already registered", key))
	}
	e.handlers[key] = handler
}

// Execute runs the native handler for the given action.
func (e *NativeExecutor) Execute(ctx context.Context, action spec.Action, params ExecuteParams) (*ExecuteResult, error) {
	if action.Impl == nil || action.Impl.Ref == "" {
		return nil, fmt.Errorf("native action %s has no impl.ref", action.Name)
	}

	handler := e.resolve(action.Impl.Ref, params)
	if handler == nil {
		return nil, fmt.Errorf("native handler %q not registered for action %s.%s (%s)",
			action.Impl.Ref, params.Module, params.ActionName, action.Name)
	}

	data, err := handler(ctx, params)
	if err != nil {
		return nil, err
	}

	return &ExecuteResult{Data: data}, nil
}

// resolve finds a handler by ref, trying multiple key formats.
//
// Resolution order:
//  1. Exact ref match: "OrderResource.UpdateDiscountRule"
//  2. Module-entity-action match: "billing.order.update-discount-rule"
//  3. Module-method match: "billing.OrderResource.UpdateDiscountRule"
func (e *NativeExecutor) resolve(ref string, params ExecuteParams) NativeHandler {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. Exact match
	if h, ok := e.handlers[ref]; ok {
		return h
	}

	// 2. Module.Entity.Action match
	key := fmt.Sprintf("%s.%s.%s", params.Module, params.Entity, params.ActionName)
	if h, ok := e.handlers[key]; ok {
		return h
	}

	// 3. Module.TypeName.MethodName match (ref = "TypeName.MethodName")
	if dot := strings.LastIndex(ref, "."); dot > 0 {
		typeName := ref[:dot]
		methodName := ref[dot+1:]
		fullKey := fmt.Sprintf("%s.%s.%s", params.Module, typeName, methodName)
		if h, ok := e.handlers[fullKey]; ok {
			return h
		}
	}

	return nil
}
