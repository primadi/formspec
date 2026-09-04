package vendor

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveAlias computes the effective name for a module being installed
// (Opsi B, technical note D-e): the alias is fixed at INSTALL time, computed
// against ALL previously installed modules (active or not) plus local
// modules — never re-derived at activation time, so activating two
// same-named vendors later can never surprise-rename either.
//
// takenNames must contain every effective name already in use: lock entries
// (effective names) + local module directory names.
//
// Derivation: `{org}-{name}` from the source path
// (github.com/acme/billing-module → acme-billing); fallback `{name}-2`,
// `{name}-3`, … when the derived alias is also taken.
func ResolveAlias(name, source string, takenNames []string) string {
	taken := make(map[string]bool, len(takenNames))
	for _, t := range takenNames {
		taken[t] = true
	}
	if !taken[name] {
		return "" // no conflict — no alias
	}
	candidate := deriveAlias(name, source)
	if candidate == "" || taken[candidate] {
		for i := 2; ; i++ {
			candidate = fmt.Sprintf("%s-%d", name, i)
			if !taken[candidate] {
				return candidate
			}
		}
	}
	return candidate
}

// deriveAlias extracts `{org}-{name}` from the source: the path segment
// before the module segment, for both git URLs and folder paths. The module
// segment is matched by PREFIX (the repo/folder is often named
// `{name}-module`), and the org is the segment immediately before it.
func deriveAlias(name, source string) string {
	source = strings.TrimSuffix(strings.TrimSuffix(source, "/"), ".git")
	parts := strings.Split(filepath.ToSlash(source), "/")
	for i := len(parts) - 1; i > 0; i-- {
		seg := parts[i]
		if seg == name || strings.HasPrefix(seg, name+"-") {
			org := sanitizeAliasPart(parts[i-1])
			if org != "" && org != name {
				return org + "-" + name
			}
		}
	}
	// Fallback: vendor suffix when no parent segment matches.
	return name + "-vendor"
}

// sanitizeAliasPart keeps alias-safe characters (alnum + dash).
func sanitizeAliasPart(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == '_' || r == '.':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
