package vendor

import (
	"fmt"
	"regexp"
	"strings"
)

// Marker blocks live inside the App manifest's `modules:` list (adapted from
// the technical note: AppSpec.Modules is []string, so the marker carries the
// EFFECTIVE module name; source@version metadata lives in the header):
//
//	modules:
//	  - billing
//	  # >>> formspec:vendor github.com/acme/billing-module @1.0.0
//	  # - acme-billing          ← commented = inactive
//	  # <<< formspec:vendor
//
// Active entry (uncommented):
//
//	  # >>> formspec:vendor github.com/acme/billing-module @1.1.0
//	  - acme-billing
//	  # <<< formspec:vendor
//
// Re-install updates the version in the header and preserves the
// commented/uncommented state (D-g — idempotensi update).

const (
	MarkerBegin = ">>> formspec:vendor"
	MarkerEnd   = "<<< formspec:vendor"
)

var markerHeaderRe = regexp.MustCompile(`(?m)^\s*#\s*>>> formspec:vendor (\S+)(?: @(\S+))?\s*$`)

// Marker is one parsed vendor block in an App manifest.
type Marker struct {
	Source  string
	Version string
	// Entry is the module line inside the block with comments stripped
	// ("" when the entry line is missing).
	Entry string
	// Active reports whether the entry is uncommented.
	Active bool
	// Begin/End are line indexes (0-based, inclusive) in the file.
	Begin, End int
}

// FindMarkers parses all vendor marker blocks in an App manifest's text.
func FindMarkers(content string) []Marker {
	lines := strings.Split(content, "\n")
	var out []Marker
	inBlock := false
	var cur Marker
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inBlock && strings.Contains(trimmed, MarkerBegin):
			m := markerHeaderRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			inBlock = true
			cur = Marker{Source: m[1], Version: m[2], Begin: i}
		case inBlock && strings.Contains(trimmed, MarkerEnd):
			cur.End = i
			out = append(out, cur)
			inBlock = false
		case inBlock:
			// Entry line: "- name" or "# - name".
			entry := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if strings.HasPrefix(entry, "- ") {
				cur.Entry = strings.TrimSpace(strings.TrimPrefix(entry, "- "))
				cur.Active = !strings.HasPrefix(trimmed, "#")
			}
		}
	}
	return out
}

// RenderMarker renders one marker block. active controls whether the entry
// line is commented. indent must match the App manifest's modules-list
// indentation so the (uncommented) entry stays valid YAML.
func RenderMarker(source, version, effectiveName string, active bool, indent string) string {
	entry := indent + "- " + effectiveName
	if !active {
		entry = indent + "# - " + effectiveName
	}
	v := ""
	if version != "" {
		v = " @" + version
	}
	return fmt.Sprintf("%s# >>> formspec:vendor %s%s\n%s\n%s# <<< formspec:vendor", indent, source, v, entry, indent)
}

// UpsertMarker inserts or updates the marker block for source in the App
// manifest text, preserving the existing active state on update (D-g).
// Returns the new file content.
func UpsertMarker(content, source, version, effectiveName string, activeIfNew bool) string {
	lines := strings.Split(content, "\n")
	// An empty flow list (`modules: []`) cannot receive block entries —
	// convert it to an empty block list first.
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "modules:") && strings.HasSuffix(trimmed, "[]") {
			lines[i] = line[:len(line)-len(trimmed)] + "modules:"
			break
		}
	}
	content = strings.Join(lines, "\n")
	markers := FindMarkers(content)
	insertAt, indent := findModulesInsertPoint(lines)
	for _, m := range markers {
		if m.Source != source {
			continue
		}
		// D-g: re-install preserves the active state — EXCEPT when the
		// caller explicitly requests activation (--use): an explicit flag
		// is a user decision, not a surprise. A block without an entry
		// line falls back to activeIfNew.
		active := m.Active
		if activeIfNew || !m.EntrySet() {
			active = activeIfNew
		}
		block := RenderMarker(source, version, effectiveName, active, indent)
		replacement := strings.Split(block, "\n")
		newLines := append([]string{}, lines[:m.Begin]...)
		newLines = append(newLines, replacement...)
		newLines = append(newLines, lines[m.End+1:]...)
		return strings.Join(newLines, "\n")
	}
	// New block — append after the last list entry of the modules section,
	// or at the end of the file when the section can't be located.
	block := strings.Split(RenderMarker(source, version, effectiveName, activeIfNew, indent), "\n")
	newLines := append([]string{}, lines[:insertAt]...)
	newLines = append(newLines, block...)
	newLines = append(newLines, lines[insertAt:]...)
	return strings.Join(newLines, "\n")
}

// EntrySet reports whether an entry line was found in the block.
func (m Marker) EntrySet() bool { return m.Entry != "" }

// RemoveMarker deletes the marker block for source. Returns the new content
// and whether a block was removed.
func RemoveMarker(content, source string) (string, bool) {
	for _, m := range FindMarkers(content) {
		if m.Source != source {
			continue
		}
		lines := strings.Split(content, "\n")
		newLines := append([]string{}, lines[:m.Begin]...)
		newLines = append(newLines, lines[m.End+1:]...)
		return strings.Join(newLines, "\n"), true
	}
	return content, false
}

// keyLineRe matches a YAML mapping key like `menu:` or `root_url: /app` —
// used to detect where the modules list ends.
var keyLineRe = regexp.MustCompile(`^[A-Za-z_][\w.-]*:\s*(.*)$`)

// findModulesInsertPoint returns the line index after the last entry of the
// `modules:` list and the list's indentation (or len(lines)/"  " when the
// section is absent). The list ends at the next mapping key at the same or
// lower indent (e.g. `menu:`) — NOT at the next list item, since sibling
// sections like `menu:` are also lists.
func findModulesInsertPoint(lines []string) (int, string) {
	inModules := false
	insertAt := len(lines)
	indent := "  "
	listIndent := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "modules:") {
			inModules = true
			insertAt = i + 1
			continue
		}
		if !inModules {
			continue
		}
		if trimmed == "" {
			continue
		}
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		isComment := strings.HasPrefix(trimmed, "#")
		entryBody := trimmed
		if isComment {
			entryBody = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		}
		isEntry := strings.HasPrefix(entryBody, "- ")
		isMarker := strings.Contains(trimmed, MarkerBegin) || strings.Contains(trimmed, MarkerEnd)

		switch {
		case isEntry:
			if listIndent == -1 {
				listIndent = lineIndent
			}
			indent = line[:lineIndent]
			insertAt = i + 1
		case isMarker:
			insertAt = i + 1
		case isComment:
			// Comments at list indent or deeper stay inside the list.
			if listIndent != -1 && lineIndent < listIndent {
				return insertAt, indent
			}
		default:
			// A mapping key at the same or lower indent ends the list
			// (e.g. `menu:` after the module entries).
			if keyLineRe.MatchString(entryBody) && (listIndent == -1 || lineIndent <= listIndent) {
				return insertAt, indent
			}
			// Deeper-nested content of an entry — keep scanning.
			if listIndent != -1 && lineIndent > listIndent {
				continue
			}
			return insertAt, indent
		}
	}
	return insertAt, indent
}
