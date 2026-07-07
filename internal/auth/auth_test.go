package auth

import (
	"testing"
)

func TestIdentityHasPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		required    string
		want        bool
	}{
		// Public / empty
		{name: "public always allowed", permissions: nil, required: "public", want: true},
		{name: "empty required always allowed", permissions: nil, required: "", want: true},

		// Super-wildcard (dev mode)
		{name: "wildcard star matches everything", permissions: []string{"*"}, required: "anything.here", want: true},
		{name: "wildcard star matches random", permissions: []string{"*"}, required: "foo.bar.baz", want: true},

		// Exact match
		{name: "exact match", permissions: []string{"billing.invoices.list"}, required: "billing.invoices.list", want: true},
		{name: "no match", permissions: []string{"billing.invoices.list"}, required: "billing.invoices.delete", want: false},
		{name: "empty permissions", permissions: nil, required: "billing.invoices.list", want: false},

		// Wildcard: module.entity.*
		{name: "wildcard matches child action", permissions: []string{"billing.invoices.*"}, required: "billing.invoices.list", want: true},
		{name: "wildcard matches another child action", permissions: []string{"billing.invoices.*"}, required: "billing.invoices.delete", want: true},
		{name: "wildcard does NOT match deeper", permissions: []string{"billing.invoices.*"}, required: "billing.invoices.list.items", want: false},
		{name: "wildcard does NOT match other entity", permissions: []string{"billing.invoices.*"}, required: "billing.customers.list", want: false},
		{name: "wildcard does NOT match partial prefix", permissions: []string{"billing.inv.*"}, required: "billing.invoices.list", want: false},

		// Multiple permissions
		{name: "match in list", permissions: []string{"billing.customers.list", "billing.invoices.list"}, required: "billing.invoices.list", want: true},
		{name: "wildcard in list", permissions: []string{"billing.customers.*", "billing.invoices.*"}, required: "billing.invoices.delete", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := &Identity{
				UserID:      "test-user",
				WorkspaceID: "demo",
				Permissions: tt.permissions,
			}
			got := id.HasPermission(tt.required)
			if got != tt.want {
				t.Errorf("HasPermission(%q) with perms=%v: got %v, want %v",
					tt.required, tt.permissions, got, tt.want)
			}
		})
	}
}

func TestIdentityIsAuthenticated(t *testing.T) {
	tests := []struct {
		name     string
		identity *Identity
		want     bool
	}{
		{name: "nil identity", identity: nil, want: false},
		{name: "empty user", identity: &Identity{UserID: ""}, want: false},
		{name: "valid user", identity: &Identity{UserID: "alice"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.identity.IsAuthenticated(); got != tt.want {
				t.Errorf("IsAuthenticated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIdentityDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		identity *Identity
		want     string
	}{
		{name: "nil identity", identity: nil, want: "anonymous"},
		{name: "valid identity", identity: &Identity{UserID: "alice", WorkspaceID: "acme"}, want: "alice@acme"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.identity.DisplayName(); got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
