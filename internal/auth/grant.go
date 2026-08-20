package auth

// Grant types model the admin-facing permission grant hierarchy:
//
//	Role → Page → (Tab →) Action (+ conditions)
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

// ActionGrant is one action-level grant, optionally carrying ABAC conditions.
type ActionGrant struct {
	// Name is the action name (e.g. "create", "submit", or a custom action).
	Name string `json:"name"`
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
