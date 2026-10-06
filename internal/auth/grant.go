package auth

import "github.com/primadi/formspec/pkg/spec"

// Grant types model the admin-facing permission grant hierarchy:
//
//	Role → Page → (Tab →) Action (+ conditions / row scope)
//
// This is the admin's mental model (one-to-one with what they see in the UI).
// At enforcement time these grants are MATERIALIZED into concrete
// `{module}.{entity}.{action}` permission strings (todo 5.12.5) — the page/tab
// structure is a grouping for admin UX, never the source of authorization.
//
// A page with blocks (no tabs) uses `Actions` directly; a tabbed page uses
// `Tabs` (each tab carrying its own actions).

// Grant is one page-level grant within a role.
type Grant struct {
	// Page is the kind: Page name this grant refers to.
	Page string `json:"page"`
	// Actions are the granted actions for a block page (no tabs).
	Actions []ActionGrant `json:"actions,omitempty"`
	// Tabs are the granted tabs for a tabbed page.
	Tabs []TabGrant `json:"tabs,omitempty"`
}

// TabGrant is one tab-level grant within a page.
type TabGrant struct {
	// Tab is the tab label (matches PageTab.Label).
	Tab string `json:"tab"`
	// Actions are the granted actions within this tab.
	Actions []ActionGrant `json:"actions"`
}

// ActionGrant is one action-level grant, optionally carrying ABAC conditions
// and a row scope.
//
// Two constraints live here and they are NOT interchangeable:
//
//   - `RowScope` restricts WHICH ROWS the action may touch. It is a filter
//     list, so it is enforced by the storage layer together with the entity's
//     own `row_scope` — a role may hold `list` on orders yet see only the paid
//     ones (kafe 10.67: "hanya pesanan lunas yang masuk dapur").
//   - `Conditions` constrains the PAYLOAD being written (an attribute
//     predicate over `resource` + `params`). It cannot express a row filter:
//     a read would have to load every row and evaluate it one by one, which
//     breaks pagination and pushes nothing into the query.
type ActionGrant struct {
	// Name is the action name (e.g. "create", "submit", or a custom action).
	Name string `json:"name"`
	// RowScope restricts the rows this granted action applies to. Values come
	// from the request context (`from: session|route`) or are a literal
	// constant (`value`), never from the client — a caller cannot widen or drop
	// it by editing the request. Empty = every row the permission reaches.
	RowScope []spec.FilterSpec `json:"row_scope,omitempty"`
	// Conditions are attribute-based constraints (FormSpecExpr) evaluated at
	// enforcement time against the resource data (todo 6.2.6). Empty = no
	// constraint beyond the permission itself.
	Conditions []ConditionGrant `json:"conditions,omitempty"`
}

// ConditionGrant is an ABAC condition attached to an action grant.
type ConditionGrant struct {
	// Expr is a FormSpecExpr evaluated against `resource` (the data being
	// submitted) + `params`. If it evaluates false, the action is rejected
	// with Message.
	Expr string `json:"expr"`
	// Message is the custom error message shown when the condition fails.
	Message string `json:"message,omitempty"`
}
