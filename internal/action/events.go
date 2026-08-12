package action

import "github.com/primadi/formspec/pkg/spec"

// ResolveEmission builds an EventEmission for an action's Emits value by
// matching it against the entity's declared Events. Returns nil if Emits is
// empty or no EventDecl matches (a spec-authoring mistake caught by
// spec.ValidateActionEmits at formspec-apply time, not a runtime failure).
func ResolveEmission(events []spec.EventDecl, emits string, recordData map[string]any) *EventEmission {
	if emits == "" {
		return nil
	}
	for _, e := range events {
		if e.Name != emits {
			continue
		}
		payload := recordData
		if e.Payload != nil && len(e.Payload.Fields) > 0 {
			payload = projectFields(recordData, e.Payload.Fields)
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

// projectFields returns a copy of data containing only the named fields —
// used to build an event payload restricted to its declared payload.fields.
func projectFields(data map[string]any, fields []string) map[string]any {
	projected := make(map[string]any, len(fields))
	for _, f := range fields {
		projected[f] = data[f]
	}
	return projected
}
