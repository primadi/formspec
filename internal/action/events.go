package action

import "github.com/primadi/formspec/pkg/spec"

// ResolveEmission builds an EventEmission for an action's Emits value by
// matching it against the entity's declared Events. Returns nil if Emits is
// empty or no EventDecl matches (a spec-authoring mistake caught by
// spec.ValidateActionEmits at formspec-apply time, not a runtime failure).
//
// recordID is the id of the record the event is about. It is threaded
// separately from recordData because a record's id is a table column, not a
// field: `data["id"]` is always absent, so a `payload.fields: [id]` projection
// produced a null id and every consumer that addressed the record failed
// (gl-balance's `resource.fetch("gl.journal-entry", params.id)` died on
// "got NoneType, want string"). Pass "" when the caller has no id.
func ResolveEmission(events []spec.EventDecl, emits, recordID string, recordData map[string]any) *EventEmission {
	if emits == "" {
		return nil
	}
	for _, e := range events {
		if e.Name != emits {
			continue
		}
		// A declared payload.fields projects the record down to those keys; with
		// no declaration the whole record travels. Either way the record's own id
		// is merged in — see withRecordID.
		var payload map[string]any
		if e.Payload != nil && len(e.Payload.Fields) > 0 {
			payload = projectFields(recordData, e.Payload.Fields, recordID)
		} else {
			payload = withRecordID(recordData, recordID)
		}
		return &EventEmission{
			Name:      e.Name,
			Durable:   e.Publish != nil && e.Publish.Durable,
			Payload:   payload,
			DeliverTo: e.Deliver,
		}
	}
	return nil
}

// withRecordID returns data with "id" filled in from the record's own column.
// A data map that already carries an "id" key keeps it (an entity that really
// declares an `id` field wins over the framework column), and the map is copied
// rather than mutated — the caller's live record must not gain an "id" key that
// would then be persisted.
func withRecordID(data map[string]any, recordID string) map[string]any {
	if recordID == "" {
		return data
	}
	if _, ok := data["id"]; ok {
		return data
	}
	out := make(map[string]any, len(data)+1)
	for k, v := range data {
		out[k] = v
	}
	out["id"] = recordID
	return out
}

// ResolveTransitionEmission builds an EventEmission for the state-machine
// transition that moved a record from oldState to newState (S13). It finds the
// transition whose `from` matches oldState and whose `to` matches newState, and
// resolves its `emit` against the entity's declared events. Returns nil when no
// transition matches or the matching transition declares no `emit` — a
// transition without `emit` simply publishes nothing, which is the default.
//
// This is what makes the transition↔event link explicit: the event is emitted
// because the transition says so, not because its name happens to match a state.
func ResolveTransitionEmission(sm *spec.StateMachine, events []spec.EventDecl, oldState, newState, recordID string, recordData map[string]any) *EventEmission {
	if sm == nil || oldState == newState {
		return nil
	}
	for _, t := range sm.Transitions {
		if !t.From.Matches(oldState) || t.To != newState {
			continue
		}
		if t.Emit == "" {
			return nil
		}
		return ResolveEmission(events, t.Emit, recordID, recordData)
	}
	return nil
}

// projectFields returns a copy of data containing only the named fields —
// used to build an event payload restricted to its declared payload.fields.
//
// A declared field the record does not carry stays present-but-null rather than
// being dropped: the publisher promised that field, so a consumer can rely on
// the key existing. `id` is the one field that is always resolvable even though
// it is never in the data map — it is the record's own identity.
func projectFields(data map[string]any, fields []string, recordID string) map[string]any {
	projected := make(map[string]any, len(fields))
	for _, f := range fields {
		if f == "id" {
			if _, inData := data["id"]; !inData {
				projected[f] = nullableID(recordID)
				continue
			}
		}
		projected[f] = data[f]
	}
	return projected
}

// nullableID returns nil for an unknown id so the payload stays JSON-null
// rather than the empty string, which a consumer would read as a real (but
// wrong) identifier.
func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
