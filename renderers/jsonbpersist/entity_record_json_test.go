package db

import (
	"encoding/json"
	"testing"
)

// TestEntityRecord_MarshalJSON_FlattensAndUsesSnakeCase locks the wire
// contract clients depend on (docs/spec/backend/01-core-basic.md §8: single
// response `{ data, meta }` where `data` is the record's own fields plus
// id/version/timestamps at the same level, snake_case) — not a Go-field
// PascalCase struct with a nested "Data" key.
func TestEntityRecord_MarshalJSON_FlattensAndUsesSnakeCase(t *testing.T) {
	rec := EntityRecord{
		ID:        "inv-1",
		WorkspaceID:  "acme",
		Version:   3,
		CreatedAt: "2026-07-11T00:00:00Z",
		UpdatedAt: "2026-07-11T01:00:00Z",
		CreatedBy: "u1",
		UpdatedBy: "u2",
		Data:      map[string]any{"status": "draft", "total": 150000.0},
	}

	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// No nested "Data" or "ID" (PascalCase) keys should survive.
	if _, ok := out["Data"]; ok {
		t.Error(`response has a nested "Data" key — fields must be flattened`)
	}
	if _, ok := out["ID"]; ok {
		t.Error(`response has PascalCase "ID" — must be snake_case "id"`)
	}

	want := map[string]any{
		"id":         "inv-1",
		"tenant_id":  "acme",
		"version":    float64(3),
		"created_at": "2026-07-11T00:00:00Z",
		"updated_at": "2026-07-11T01:00:00Z",
		"created_by": "u1",
		"updated_by": "u2",
		"status":     "draft",
		"total":      150000.0,
	}
	for k, v := range want {
		if out[k] != v {
			t.Errorf("key %q = %v, want %v", k, out[k], v)
		}
	}
	if len(out) != len(want) {
		t.Errorf("got %d keys, want %d: %v", len(out), len(want), out)
	}
}

func TestEntityRecord_MarshalJSON_ReservedNameWins(t *testing.T) {
	rec := EntityRecord{
		ID:   "inv-1",
		Data: map[string]any{"id": "should-not-win"},
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	json.Unmarshal(raw, &out)
	if out["id"] != "inv-1" {
		t.Errorf(`reserved "id" = %v, want the record ID to win over a same-named field`, out["id"])
	}
}
