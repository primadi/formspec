package db

import (
	"encoding/json"
	"testing"
)

// A record's id lives in its own table column, never in the data map, so it can
// never be selected by an event's payload.fields — yet a subscriber cannot react
// to an event without knowing which record it is about. withRecordID closes that
// gap at the single point where the payload is written to the outbox.
func TestWithRecordID(t *testing.T) {
	t.Run("injects id into the payload envelope", func(t *testing.T) {
		// The shape action.BuildEventMessage produces: event/resource/payload.
		in := `{"event":"on_paid","resource":"cafe-order/order","payload":{"number":"ORD-1","status":"paid"},"emitted_at":"2026-09-21T00:00:00Z"}`

		got := withRecordID(in, "01a0c332-d45b-7039-b2d3-f8f52d0e63dc")

		var msg map[string]any
		if err := json.Unmarshal([]byte(got), &msg); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		body, ok := msg["payload"].(map[string]any)
		if !ok {
			t.Fatalf("payload is not an object: %T", msg["payload"])
		}
		if body["id"] != "01a0c332-d45b-7039-b2d3-f8f52d0e63dc" {
			t.Errorf("payload.id = %v, want the record id", body["id"])
		}
		// The event's own fields must survive untouched.
		if body["number"] != "ORD-1" || body["status"] != "paid" {
			t.Errorf("payload fields were disturbed: %+v", body)
		}
		if msg["event"] != "on_paid" || msg["resource"] != "cafe-order/order" {
			t.Errorf("envelope was disturbed: %+v", msg)
		}
	})

	t.Run("an explicit id field wins over the record column", func(t *testing.T) {
		in := `{"event":"e","payload":{"id":"declared-field-value","x":1}}`

		got := withRecordID(in, "record-column-id")

		var msg map[string]any
		if err := json.Unmarshal([]byte(got), &msg); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}
		if body := msg["payload"].(map[string]any); body["id"] != "declared-field-value" {
			t.Errorf("payload.id = %v, want the entity's own id field to win", body["id"])
		}
	})

	t.Run("empty record id leaves the message untouched", func(t *testing.T) {
		in := `{"event":"e","payload":{"x":1}}`
		if got := withRecordID(in, ""); got != in {
			t.Errorf("got %q, want the input unchanged", got)
		}
	})

	t.Run("empty message is left alone", func(t *testing.T) {
		if got := withRecordID("", "rec-1"); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("malformed JSON passes through rather than being dropped", func(t *testing.T) {
		in := `{"event":` // truncated
		if got := withRecordID(in, "rec-1"); got != in {
			t.Errorf("got %q, want the input unchanged", got)
		}
	})

	t.Run("a payload that is not an object is not reshaped", func(t *testing.T) {
		in := `{"event":"e","payload":"scalar"}`
		if got := withRecordID(in, "rec-1"); got != in {
			t.Errorf("got %q, want the input unchanged", got)
		}
	})
}
