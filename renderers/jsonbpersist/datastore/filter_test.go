package datastore

import (
	"testing"

	"github.com/primadi/forma/pkg/spec"
)

func TestFilterMatch_NoFilter(t *testing.T) {
	ws := WorkspaceInfo{ID: "ws-1", Environment: "production"}

	if !FilterMatch(nil, ws) {
		t.Error("nil filter should match all")
	}

	empty := &spec.DatastoreAccessFilter{}
	if !FilterMatch(empty, ws) {
		t.Error("empty filter should match all")
	}
}

func TestFilterMatch_Environment(t *testing.T) {
	filter := &spec.DatastoreAccessFilter{Environment: "production"}

	tests := []struct {
		name        string
		ws          WorkspaceInfo
		shouldMatch bool
	}{
		{"match", WorkspaceInfo{ID: "ws-1", Environment: "production"}, true},
		{"mismatch", WorkspaceInfo{ID: "ws-1", Environment: "dev"}, false},
		{"empty env", WorkspaceInfo{ID: "ws-1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if FilterMatch(filter, tt.ws) != tt.shouldMatch {
				t.Errorf("expected match=%v", tt.shouldMatch)
			}
		})
	}
}

func TestFilterMatch_Workspaces(t *testing.T) {
	filter := &spec.DatastoreAccessFilter{Workspaces: []string{"corp-456", "corp-789"}}

	tests := []struct {
		name        string
		ws          WorkspaceInfo
		shouldMatch bool
	}{
		{"match first", WorkspaceInfo{ID: "corp-456"}, true},
		{"match second", WorkspaceInfo{ID: "corp-789"}, true},
		{"no match", WorkspaceInfo{ID: "corp-999"}, false},
		{"empty id", WorkspaceInfo{ID: ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if FilterMatch(filter, tt.ws) != tt.shouldMatch {
				t.Errorf("expected match=%v", tt.shouldMatch)
			}
		})
	}
}

func TestFilterMatch_Labels(t *testing.T) {
	filter := &spec.DatastoreAccessFilter{Labels: map[string]string{"tier": "enterprise"}}

	tests := []struct {
		name        string
		ws          WorkspaceInfo
		shouldMatch bool
	}{
		{"match", WorkspaceInfo{ID: "ws-1", Labels: map[string]string{"tier": "enterprise"}}, true},
		{"missing label", WorkspaceInfo{ID: "ws-1"}, false},
		{"wrong value", WorkspaceInfo{ID: "ws-1", Labels: map[string]string{"tier": "free"}}, false},
		{"superset labels", WorkspaceInfo{ID: "ws-1", Labels: map[string]string{"tier": "enterprise", "region": "us"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if FilterMatch(filter, tt.ws) != tt.shouldMatch {
				t.Errorf("expected match=%v", tt.shouldMatch)
			}
		})
	}
}

func TestFilterMatch_ANDLogic(t *testing.T) {
	filter := &spec.DatastoreAccessFilter{
		Environment: "production",
		Workspaces:  []string{"corp-456", "corp-789"},
		Labels:      map[string]string{"tier": "enterprise"},
	}

	// All match
	ws := WorkspaceInfo{ID: "corp-456", Environment: "production", Labels: map[string]string{"tier": "enterprise"}}
	if !FilterMatch(filter, ws) {
		t.Error("should match all criteria")
	}

	// Environment fails
	ws = WorkspaceInfo{ID: "corp-456", Environment: "dev", Labels: map[string]string{"tier": "enterprise"}}
	if FilterMatch(filter, ws) {
		t.Error("should fail on environment")
	}

	// Workspace fails
	ws = WorkspaceInfo{ID: "corp-999", Environment: "production", Labels: map[string]string{"tier": "enterprise"}}
	if FilterMatch(filter, ws) {
		t.Error("should fail on workspace")
	}

	// Labels fail
	ws = WorkspaceInfo{ID: "corp-456", Environment: "production", Labels: map[string]string{"tier": "free"}}
	if FilterMatch(filter, ws) {
		t.Error("should fail on labels")
	}
}

func TestPermissionCheck_Default(t *testing.T) {
	// No permission spec → read_write
	if !PermissionCheck(nil, spec.AccessRead, "any.scope") {
		t.Error("nil permission should allow read")
	}
	if !PermissionCheck(nil, spec.AccessWrite, "any.scope") {
		t.Error("nil permission should allow write")
	}
}

func TestPermissionCheck_ReadOnly(t *testing.T) {
	perm := &spec.DatastorePermission{Default: spec.AccessRead}

	if !PermissionCheck(perm, spec.AccessRead, "any.scope") {
		t.Error("read should be allowed")
	}
	if PermissionCheck(perm, spec.AccessWrite, "any.scope") {
		t.Error("write should be denied")
	}
}

func TestPermissionCheck_GranularRules(t *testing.T) {
	perm := &spec.DatastorePermission{
		Default: spec.AccessRead,
		Rules: []spec.DatastorePermissionRule{
			{Scope: "store.*", Access: spec.AccessReadWrite},
			{Scope: "billing.invoice", Access: spec.AccessWrite},
		},
	}

	tests := []struct {
		name     string
		op       spec.AccessPermission
		scope    string
		expected bool
	}{
		{"default read allowed", spec.AccessRead, "other.table", true},
		{"default write denied", spec.AccessWrite, "other.table", false},
		{"store.* read allowed", spec.AccessRead, "store.product", true},
		{"store.* write allowed", spec.AccessWrite, "store.product", true},
		{"billing.invoice write only", spec.AccessWrite, "billing.invoice", true},
		{"billing.invoice read denied", spec.AccessRead, "billing.invoice", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if PermissionCheck(perm, tt.op, tt.scope) != tt.expected {
				t.Errorf("expected %v for %s on %s", tt.expected, tt.op, tt.scope)
			}
		})
	}
}

func TestMatchScope(t *testing.T) {
	tests := []struct {
		pattern, scope string
		expected       bool
	}{
		{"*.*", "anything.here", true},
		{"store.*", "store.product", true},
		{"store.*", "store.order", true},
		{"store.*", "other.product", false},
		{"store.*", "store", false},
		{"billing.invoice", "billing.invoice", true},
		{"billing.invoice", "billing.payment", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.scope, func(t *testing.T) {
			if matchScope(tt.pattern, tt.scope) != tt.expected {
				t.Errorf("matchScope(%q, %q) = %v, want %v", tt.pattern, tt.scope, !tt.expected, tt.expected)
			}
		})
	}
}
