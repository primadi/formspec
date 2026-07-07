// Package api provides the API generation and serving layer for Forma.
//
// It implements the deny-by-default exposure model (D49) with multi-protocol
// routing (D50). Route descriptors are protocol-agnostic; the router dispatches
// to the correct protocol adapter (REST via chi, gRPC, WebSocket).
package api

import "github.com/forma/forma/pkg/spec"

// ProtocolType mirrors spec.ProtocolType for local convenience.
type ProtocolType = spec.ProtocolType

const (
	ProtocolREST      = spec.ProtocolREST
	ProtocolGRPC      = spec.ProtocolGRPC
	ProtocolWebSocket = spec.ProtocolWebSocket
)

// RouteDescriptor is a protocol-agnostic route specification.
// Multiple adapters consume the same descriptors to register their routes.
type RouteDescriptor struct {
	Module             string            // module name
	Entity             string            // entity name
	Plural             string            // table plural (for path building)
	Action             string            // list, find, create, update, delete, or custom
	Method             string            // GET, POST, PATCH, DELETE
	Path               string            // REST path template relative to workspace prefix
	Protocol           spec.ProtocolType // rest, grpc, ws
	Handler            string            // "auto" for CRUD, or custom action name
	RequiredPermission string            // permission string like "billing.customers.list" or "public"; empty = no check (internal only)
}

// StandardRESTActions is the set of auto-generated REST actions for entities.
var StandardRESTActions = []struct {
	Action, Method, PathSuffix string
	PermissionAction           string
}{
	{Action: "list", Method: "GET", PathSuffix: "", PermissionAction: "list"},
	{Action: "find", Method: "GET", PathSuffix: "/{id}", PermissionAction: "view"},
	{Action: "create", Method: "POST", PathSuffix: "", PermissionAction: "create"},
	{Action: "update", Method: "PATCH", PathSuffix: "/{id}", PermissionAction: "update"},
	{Action: "delete", Method: "DELETE", PathSuffix: "/{id}", PermissionAction: "delete"},
}
