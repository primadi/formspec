// ─── Starlark sandbox hard limits (todo 7.14) ───
//
// Enforces the hard limits from 06-script-runtime.md §7:
//   - wall-clock 5000ms (via a context deadline threaded into primitives)
//   - iterations 100K (via starlark.Thread.SetMaxExecutionSteps)
//   - max 50 DB queries
//   - max 1000 records read
//
// Exceeding any limit aborts the script with an error — no partial results.
// Memory (64MB) is not directly measurable in the Starlark interpreter; the
// step limit is the practical bound on runaway allocation.

package starlark

import (
	"fmt"
	"sync"
)

// Default sandbox limits (06-script-runtime.md §7).
const (
	DefaultMaxExecutionSteps = 100000 // iterations
	DefaultMaxDBQueries      = 50
	DefaultMaxRecordsRead    = 1000
	DefaultWallClockTimeout  = 5000 // ms
)

// ScriptLimits tracks resource usage during one script execution. It is
// stored on the Starlark thread (like the Go context) so primitive
// operations can check/increment it.
type ScriptLimits struct {
	mu          sync.Mutex
	dbQueries   int
	recordsRead int
	maxQueries  int
	maxRecords  int
}

// NewScriptLimits creates a ScriptLimits with the standard defaults.
func NewScriptLimits() *ScriptLimits {
	return &ScriptLimits{
		maxQueries: DefaultMaxDBQueries,
		maxRecords: DefaultMaxRecordsRead,
	}
}

// CheckQuery reserves one DB query, returning an error when the limit is
// exceeded (todo 7.14.1 — abort, no partial results).
func (l *ScriptLimits) CheckQuery() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.maxQueries > 0 && l.dbQueries >= l.maxQueries {
		return fmt.Errorf("script exceeded max %d db queries", l.maxQueries)
	}
	l.dbQueries++
	return nil
}

// AddRecordsRead accounts for records read by a list/query operation,
// returning an error when the limit is exceeded.
func (l *ScriptLimits) AddRecordsRead(n int) error {
	if l == nil || n <= 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.maxRecords > 0 && l.recordsRead+n > l.maxRecords {
		return fmt.Errorf("script exceeded max %d records read", l.maxRecords)
	}
	l.recordsRead += n
	return nil
}

// limitsKey is the thread-local key under which the ScriptLimits for the
// current execution is stored.
const limitsKey = "formspec.script.limits"

// threadLimits returns the ScriptLimits stored on the thread, or nil.
func threadLimits(thread interface{ Local(key string) any }) *ScriptLimits {
	if v := thread.Local(limitsKey); v != nil {
		if l, ok := v.(*ScriptLimits); ok {
			return l
		}
	}
	return nil
}
