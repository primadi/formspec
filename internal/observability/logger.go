// Package observability — structured logging, metrics, health, request ID.
//
// Implements the engine observability contract
// (docs/spec/platform/09-observability.md):
//
//   - logger.go    — JSON-lines structured logging with the 12 mandatory
//     fields (§2.1) and PII discipline (§2.2)
//   - metrics.go   — Prometheus /metrics with the 12 mandatory metrics (§3.1)
//     and bounded-label cardinality discipline (§3.2)
//   - health.go    — machine-readable health vocabulary (§5)
//   - requestid.go — request ID context propagation (§2.3)
//
// The package is dependency-light so it can be imported by internal/api,
// internal/starlark, and cmd/formspec without import cycles.
package observability

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Level is a log severity (spec §2.1).
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Logger emits JSON-lines structured logs to a writer (stdout by default).
//
// Every record at info level and above carries the 12 mandatory fields
// (spec §2.1): timestamp, level, request_id, workspace, module, entity,
// action, actor, duration_ms, error_code, trace_id, environment. Empty
// values are written as null, never omitted.
//
// PII discipline (spec §2.2): info/warn/error records must only carry
// metadata. Business values may only appear at debug level, which is
// disabled by default in production and enabled via explicit operator
// control (SetDebugEnabled).
type Logger struct {
	mu       sync.Mutex
	w        io.Writer
	minLevel Level
	debugOn  bool
	base     Fields // boot-time fields (environment, workspace when constant)
	nowFn    func() time.Time
}

// Fields is a set of structured log fields.
type Fields map[string]interface{}

// NewLogger creates a JSON-lines logger writing to w. minLevel gates which
// records are emitted; debug is dropped unless explicitly enabled.
func NewLogger(w io.Writer, minLevel Level) *Logger {
	if w == nil {
		w = os.Stdout
	}
	return &Logger{w: w, minLevel: minLevel, nowFn: time.Now}
}

// NewProductionLogger creates a logger for production mode: JSON lines to
// stdout, info level, debug disabled (spec §2.2 — debug MUST be off by
// default in prod).
func NewProductionLogger() *Logger {
	return NewLogger(os.Stdout, LevelInfo)
}

// SetDebugEnabled toggles debug-level output. In production this is the
// operator control gate (spec §2.2) — it must be recorded by the operator
// stack, not flipped casually by application code.
func (l *Logger) SetDebugEnabled(on bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugOn = on
}

// SetBase sets fields included in every record (e.g. environment).
func (l *Logger) SetBase(f Fields) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.base = f
}

// levelRank orders levels for gating.
func levelRank(l Level) int {
	switch l {
	case LevelDebug:
		return 0
	case LevelInfo:
		return 1
	case LevelWarn:
		return 2
	case LevelError:
		return 3
	}
	return 1
}

// Log emits one structured record. fields override base fields; the 12
// mandatory fields are always present (null when empty).
func (l *Logger) Log(level Level, f Fields) {
	l.mu.Lock()
	defer l.mu.Unlock()

	rank := levelRank(level)
	// Debug gate (spec §2.2): debug records require the explicit operator
	// toggle; when enabled they bypass the min-level threshold.
	if level == LevelDebug {
		if !l.debugOn {
			return
		}
	} else if rank < levelRank(l.minLevel) {
		return
	}

	rec := make(map[string]interface{}, 16)
	// Mandatory fields default to null (spec §2.1: empty → null, not omitted).
	for _, k := range []string{"request_id", "workspace", "module", "entity",
		"action", "actor", "error_code", "trace_id", "environment"} {
		rec[k] = nil
	}
	rec["duration_ms"] = nil
	for k, v := range l.base {
		rec[k] = v
	}
	for k, v := range f {
		rec[k] = v
	}
	rec["timestamp"] = l.nowFn().UTC().Format("2006-01-02T15:04:05.000Z07:00")
	rec["level"] = string(level)

	line, err := json.Marshal(rec)
	if err != nil {
		// Never fail on logging; fall back to a minimal record.
		line = []byte(fmt.Sprintf(`{"level":"error","error_code":"LOG_MARSHAL_FAILED","message":%q}`,
			err.Error()))
	}
	l.w.Write(append(line, '\n'))
}

// Debug logs at debug level (business values allowed per §2.2).
func (l *Logger) Debug(f Fields) { l.Log(LevelDebug, f) }

// Info logs at info level (metadata only — no business data).
func (l *Logger) Info(f Fields) { l.Log(LevelInfo, f) }

// Warn logs at warn level (metadata only).
func (l *Logger) Warn(f Fields) { l.Log(LevelWarn, f) }

// Error logs at error level (metadata only).
func (l *Logger) Error(f Fields) { l.Log(LevelError, f) }

// RedactError returns an error message safe for info+ logging: contract
// error code + resource identifier, never business values (spec §2.2).
// Callers pass already-redacted contract errors; this helper strips
// multi-line content and truncates long messages.
func RedactError(msg string) string {
	msg = strings.ReplaceAll(msg, "\n", " ")
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	return msg
}
