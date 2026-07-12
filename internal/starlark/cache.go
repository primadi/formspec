// Package starlark — Compiled Program Cache
//
// Every action invocation used to call starlark.ExecFile, which parses,
// compiles, AND executes the module body from disk on every single call —
// there was no caching anywhere. For a script invoked repeatedly (which is
// the common case: any custom action gets called once per HTTP request),
// that's wasted parse/compile work on every call.
//
// This cache holds the compiled *starlark.Program (not the globals produced
// by running it) keyed by absolute script path + mtime, so edits during
// development still take effect without a process restart. Programs are
// documented by go.starlark.net as immutable and safe to share across
// concurrent goroutines — but Init(thread, predeclared) must be called fresh
// on every invocation to get a new, per-call globals StringDict. Caching the
// globals themselves instead of the Program would silently share Starlark
// module-level state across concurrent requests.
package starlark

import (
	"os"
	"sync"
	"time"

	"go.starlark.net/starlark"
)

type cachedProgram struct {
	prog    *starlark.Program
	modTime time.Time
}

type programCache struct {
	mu       sync.Mutex
	programs map[string]cachedProgram
}

var globalProgramCache = &programCache{programs: make(map[string]cachedProgram)}

// getProgram returns the compiled program for path, compiling (and caching)
// it if this is the first request or the file has changed since it was last
// cached. isPredeclared reports whether a name is one of the predeclared
// identifiers ExecuteScript will pass to Init (e.g. "ok", "fail").
func (c *programCache) getProgram(path string, isPredeclared func(name string) bool) (*starlark.Program, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	modTime := info.ModTime()

	c.mu.Lock()
	defer c.mu.Unlock()

	if cached, ok := c.programs[path]; ok && cached.modTime.Equal(modTime) {
		return cached.prog, nil
	}

	_, prog, err := starlark.SourceProgram(path, nil, isPredeclared)
	if err != nil {
		return nil, err
	}

	c.programs[path] = cachedProgram{prog: prog, modTime: modTime}
	return prog, nil
}
