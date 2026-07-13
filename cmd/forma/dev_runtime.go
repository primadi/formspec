// ─── Runtime Auto-Detect ───
//
// Detects the app runtime from project files in a given directory (scoped
// to the app folder, not the project root — see cfg.AppDir).
package main

import (
	"os"
	"path/filepath"
)

// knownRuntimeFiles maps project indicator files to runtime names.
// Ordered by priority (most specific first). Every entry here should be a
// full-fledged sidecar runtime — the only runtime that maps to "local" is
// the fallback when nothing matches (detectRuntime returns "local").
var knownRuntimeFiles = []struct {
	filenames []string // any of these files indicate this runtime
	runtime   string
}{
	{[]string{"composer.json"}, "php"},
	{[]string{"package.json"}, "node"},
	{[]string{"pyproject.toml", "requirements.txt", "setup.py", "setup.cfg"}, "python"},
	{[]string{"Gemfile"}, "ruby"},
	{[]string{"pom.xml", "build.gradle", "build.gradle.kts"}, "java"},
	{[]string{"*.csproj", "*.sln"}, "dotnet"},
	{[]string{"go.mod"}, "go"},
	{[]string{"Cargo.toml"}, "rust"},
}

// detectRuntime scans dir for project indicator files and returns the
// matching runtime name. Returns "local" if nothing is detected (the engine
// runs in embedded mode, no child process).
func detectRuntime(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "local"
	}

	// Build a set of filenames in the app directory
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
