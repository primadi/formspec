package action

import (
	"context"
	"fmt"

	"github.com/forma/forma/pkg/spec"
)

// SidecarExecutor is a stub executor for impl: { type: sidecar }.
//
// Sidecar handlers run as separate processes communicating over Unix sockets
// or gRPC. Full sidecar protocol implementation is deferred to Fase 5
// (Plane Protocol) and is not part of Core Basic conformance.
type SidecarExecutor struct{}

// NewSidecarExecutor creates a stub sidecar executor.
func NewSidecarExecutor() *SidecarExecutor {
	return &SidecarExecutor{}
}

// Execute always returns an error indicating sidecar execution is not yet implemented.
func (e *SidecarExecutor) Execute(_ context.Context, action spec.Action, params ExecuteParams) (*ExecuteResult, error) {
	return nil, fmt.Errorf(
		"sidecar execution not implemented (action %s.%s, ref=%s): deferred to Fase 5",
		params.Module, params.ActionName, action.Impl.Ref,
	)
}
