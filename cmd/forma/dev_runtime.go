// ─── Runtime Auto-Detect ───
//
// Detects the app runtime from project files in the current directory.
package main

import (
	"os"
	"path/filepath"
)

// knownRuntimeFiles maps project indicator files to runtime names.
// Ordered by priority (most specific first).
var knownRuntimeFiles = []struct {
	filenames []string // any of these files indicate this runtime
	runtime   string
}{
	{[]string{"composer.json"}, "php"},
	{[]string{"package.json"}, "node"},
	{[]string{"pyproject.toml", "requirements.txt", "setup.py", "setup.cfg"}, "python"},
	{[]string{"Cargo.toml"}, "local"},        // Rust — embedded mode
	{[]string{"go.mod"}, "local"},            // Go — use `go run .` for server
	{[]string{"*.csproj", "*.sln"}, "local"}, // .NET — no SDK yet
}

// detectRuntime scans the current directory for project indicator files
// and returns the matching runtime. Returns "local" if nothing is detected.
func detectRuntime() string {
	entries, err := os.ReadDir(".")
	if err != nil {
		return "local"
	}

	// Build a set of filenames in CWD
	fileSet := make(map[string]bool, len(entries))
	extSet := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() {
			fileSet[e.Name()] = true
			extSet[filepath.Ext(e.Name())] = true
		}
	}

	for _, rule := range knownRuntimeFiles {
		for _, name := range rule.filenames {
			if name[0] == '*' {
				// Glob pattern: *.csproj → match extension
				ext := name[1:] // ".csproj"
				if extSet[ext] {
					return rule.runtime
				}
			} else if fileSet[name] {
				return rule.runtime
			}
		}
	}

	return "local"
}
