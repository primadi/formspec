package datastore

import (
	"github.com/primadi/forma/pkg/spec"
)

// WorkspaceInfo holds runtime information about a workspace
// used to evaluate datastore access filters.
type WorkspaceInfo struct {
	ID          string
	Environment string
	Labels      map[string]string
}

// FilterMatch evaluates whether a workspace matches the datastore's access filter.
// Returns true if the workspace is authorized to use this datastore.
//
// Rules:
//   - If filter is nil or all fields empty → true (available to all)
//   - If filter fields are set → workspace must match ALL (AND logic)
func FilterMatch(filter *spec.DatastoreAccessFilter, ws WorkspaceInfo) bool {
	if filter == nil {
		return true
	}

	// Check if all filter fields are effectively empty
	if filter.Environment == "" && len(filter.Workspaces) == 0 && len(filter.Labels) == 0 {
		return true
	}

	// Environment filter
	if filter.Environment != "" {
		if ws.Environment != filter.Environment {
			return false
		}
	}

	// Workspaces filter
	if len(filter.Workspaces) > 0 {
		found := false
		for _, wid := range filter.Workspaces {
			if wid == ws.ID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Labels filter — workspace must have ALL required labels
	if len(filter.Labels) > 0 {
		if ws.Labels == nil {
			return false
		}
		for k, v := range filter.Labels {
			if ws.Labels[k] != v {
				return false
			}
		}
	}

	return true
}

// PermissionCheck checks if a requested operation (read/write) is allowed
// given the datastore's permission ceiling.
//
// Returns true if the operation is allowed, false if denied.
// scope is an optional module.table pattern for granular rules, e.g. "billing.invoice".
func PermissionCheck(perm *spec.DatastorePermission, operation spec.AccessPermission, scope string) bool {
	if perm == nil {
		// No permission spec → default is read_write for everything
		return true
	}

	effectiveAccess := perm.Default
	if effectiveAccess == "" {
		effectiveAccess = spec.AccessReadWrite
	}

	// Check granular rules — most specific match wins
	if len(perm.Rules) > 0 && scope != "" {
		bestMatch := findBestRuleMatch(perm.Rules, scope)
		if bestMatch != nil {
			effectiveAccess = bestMatch.Access
		}
	}

	return isOperationAllowed(effectiveAccess, operation)
}

// findBestRuleMatch finds the rule with the longest matching scope.
// Returns nil if no rule matches.
func findBestRuleMatch(rules []spec.DatastorePermissionRule, scope string) *spec.DatastorePermissionRule {
	var best *spec.DatastorePermissionRule
	bestLen := 0

	for i := range rules {
		if matchScope(rules[i].Scope, scope) {
			scopeLen := len(rules[i].Scope)
			if scopeLen > bestLen {
				best = &rules[i]
				bestLen = scopeLen
			}
		}
	}
	return best
}

// matchScope performs simple glob matching for module.table patterns.
// Supports:
//   - "*.*" matches everything
//   - "module.*" matches all tables in a module
//   - "module.table" matches exact table
func matchScope(pattern, scope string) bool {
	if pattern == "*.*" {
		return true
	}

	// Exact match
	if pattern == scope {
		return true
	}

	// Module wildcard: "store.*" matches "store.product", "store.order", etc.
	if len(pattern) > 2 && pattern[len(pattern)-2:] == ".*" {
		modulePrefix := pattern[:len(pattern)-2]
		// scope must be "module.tablename"
		if len(scope) > len(modulePrefix)+1 && scope[len(modulePrefix)] == '.' {
			return scope[:len(modulePrefix)] == modulePrefix
		}
	}

	return false
}

// isOperationAllowed checks if the effective permission allows the requested operation.
func isOperationAllowed(effective, requested spec.AccessPermission) bool {
	switch effective {
	case spec.AccessReadWrite:
		return true
	case spec.AccessRead:
		return requested == spec.AccessRead
	case spec.AccessWrite:
		return requested == spec.AccessWrite
	default:
		return false
	}
}
