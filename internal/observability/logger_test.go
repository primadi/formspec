package observability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestLogger_MandatoryFields proves every info+ record carries the 12
// mandatory fields (spec §2.1), with null (not omitted) for empty values.
func TestLogger_MandatoryFields(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, LevelInfo)
	log.Info(Fields{"message": "hello"})

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, buf.String())
	}

	mandatory := []string{
		"timestamp", "level", "request_id", "workspace", "module", "entity",
		"action", "actor", "duration_ms", "error_code", "trace_id", "environment",
	}
	for _, k := range mandatory {
		if _, ok := rec[k]; !ok {
			t.Errorf("mandatory field %q missing from record", k)
		}
	}
	if rec["level"] != "info" {
		t.Errorf("level = %v, want info", rec["level"])
	}
	// Empty values must be null, not omitted (spec §2.1).
	if rec["request_id"] != nil {
		t.Errorf("request_id = %v, want null", rec["request_id"])
	}
	// timestamp must be RFC 3339 UTC with millisecond precision.
	ts, _ := rec["timestamp"].(string)
	if !strings.HasSuffix(ts, "Z") || !strings.Contains(ts, ".") {
		t.Errorf("timestamp %q is not RFC 3339 UTC with milliseconds", ts)
	}
}

// TestLogger_DebugGating proves debug records are dropped unless explicitly
// enabled (spec §2.2 — debug off by default in prod).
func TestLogger_DebugGating(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, LevelInfo)

	log.Debug(Fields{"business_value": "secret-42"})
	if buf.Len() != 0 {
		t.Fatalf("debug record emitted while gated: %s", buf.String())
	}

	log.SetDebugEnabled(true)
	log.Debug(Fields{"business_value": "secret-42"})
	if !strings.Contains(buf.String(), "secret-42") {
		t.Fatal("debug record not emitted after explicit enable")
	}
}

// TestLogger_LevelGating proves min-level filtering.
func TestLogger_LevelGating(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, LevelWarn)
	log.Info(Fields{"message": "dropped"})
	if buf.Len() != 0 {
		t.Fatalf("info record emitted at warn threshold: %s", buf.String())
	}
	log.Error(Fields{"message": "kept"})
	if !strings.Contains(buf.String(), "kept") {
		t.Fatal("error record dropped at warn threshold")
	}
}

// TestLogger_Overrides proves per-call fields override base fields.
func TestLogger_Overrides(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger(&buf, LevelInfo)
	log.SetBase(Fields{"environment": "production"})
	log.Warn(Fields{"environment": "staging", "error_code": "X"})

	var rec map[string]any
	json.Unmarshal(buf.Bytes(), &rec)
	if rec["environment"] != "staging" {
		t.Errorf("environment = %v, want staging (per-call override)", rec["environment"])
	}
	if rec["error_code"] != "X" {
		t.Errorf("error_code = %v, want X", rec["error_code"])
	}
}
