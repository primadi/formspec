package manifest

import (
	"path/filepath"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// examplesDir points at the conformance/demo projects shipped with the repo
// (Clinic-UI-Showcase, Midtrans-Payment-Gateway, reference-app). They are
// written in the spec's canonical manifest vocabulary, so loading them
// end-to-end guards against drift between docs_old/spec and pkg/spec.
const examplesDir = "../../examples"

// verticalsDir points at the real, independently-installable vertical Apps
// (company, billing, inventory, gl, notifications, and the two integrator
// apps) — see docs/architecture/07-vertical-modules.md. Each is its own
// project root exactly like an examplesDir entry.
const verticalsDir = "../../verticals"

// TestExamplesLoadAndValidate walks every example and vertical project and
// asserts that all manifests parse, use known kinds, and pass entity-spec
// validation.
func TestExamplesLoadAndValidate(t *testing.T) {
	var projects []string
	for _, dir := range []string{examplesDir, verticalsDir} {
		found, err := filepath.Glob(filepath.Join(dir, "*"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		projects = append(projects, found...)
	}
	if len(projects) == 0 {
		t.Fatalf("no example/vertical projects found under %s or %s", examplesDir, verticalsDir)
	}

	total := 0
	for _, project := range projects {
		loader := NewLoader(project)
		result, err := loader.LoadAll()
		if err != nil {
			t.Fatalf("%s: LoadAll failed: %v", project, err)
		}
		for _, perr := range result.Errors {
			t.Errorf("%s: parse error: %v", project, &perr)
		}
		for _, m := range result.Manifests {
			total++
			if err := loader.Validate(m); err != nil {
				t.Errorf("validate: %v", err)
			}
		}
	}
	if total == 0 {
		t.Fatal("no manifests loaded — glob or discovery is broken")
	}
	t.Logf("validated %d manifests", total)
}

// TestOrderEntityRoundTrip pins the billing order entity — the canonical
// spec example, formerly shipped as Order-to-Cash's order.yaml, now the
// billing vertical App — field by field against the typed structs. Each
// assertion here corresponds to a construct documented in Core Basic
// (§10–§14) that the YAML layer must be able to represent.
func TestOrderEntityRoundTrip(t *testing.T) {
	loader := NewLoader(filepath.Join(verticalsDir, "billing"))
	result, err := loader.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	var order *spec.EntitySpec
	for _, m := range result.Manifests {
		if m.Kind == "Entity" && m.Metadata.Name == "order" {
			es, err := RawSpecToEntitySpec(m.Spec.(map[string]any))
			if err != nil {
				t.Fatalf("RawSpecToEntitySpec(order): %v", err)
			}
			order = es
		}
	}
	if order == nil {
		t.Fatal("order entity not found in billing vertical")
	}

	// §10.1 field types: integer / decimal must survive parsing
	fields := map[string]spec.Field{}
	for _, f := range order.Fields {
		fields[f.Name] = f
	}
	if got := fields["total"].Type; got != spec.FieldDecimal {
		t.Errorf("total: want decimal, got %q", got)
	}
	items := fields["items"]
	if items.Child == nil {
		t.Fatal("items.child missing")
	}
	childTypes := map[string]spec.FieldType{}
	for _, cf := range items.Child.Fields {
		childTypes[cf.Name] = cf.Type
	}
	if childTypes["quantity"] != spec.FieldInteger {
		t.Errorf("items.quantity: want integer, got %q", childTypes["quantity"])
	}
	if childTypes["price"] != spec.FieldDecimal {
		t.Errorf("items.price: want decimal, got %q", childTypes["price"])
	}

	// §14 state machine: via, from-list, inline guard
	sm := order.StateMachine
	if sm == nil {
		t.Fatal("state_machine missing")
	}
	if len(sm.Transitions) != 3 {
		t.Fatalf("want 3 transitions, got %d", len(sm.Transitions))
	}
	checkout := sm.Transitions[0]
	if checkout.Action != "checkout" {
		t.Errorf("transition[0] via: want checkout, got %q", checkout.Action)
	}
	if checkout.Guard == nil || checkout.Guard.Expression == "" {
		t.Error("transition[0] inline guard not parsed")
	}
	void := sm.Transitions[2]
	if len(void.From) != 2 || !void.From.Matches("awaiting_payment") {
		t.Errorf("transition[2] from-list not parsed: %v", void.From)
	}

	// §11 actions: conditions {script,message}, disabled, emits, uses.db
	actions := map[string]spec.Action{}
	for _, a := range order.Actions {
		actions[a.Name] = a
	}
	if upd := actions["update"]; len(upd.Conditions) == 0 || upd.Conditions[0].Script == "" || upd.Conditions[0].Message == "" {
		t.Error("update.conditions script/message not parsed")
	}
	if !actions["delete"].Disabled {
		t.Error("delete.disabled not parsed")
	}
	markPaid := actions["mark-paid"]
	if markPaid.Emits != "paid" {
		t.Errorf("mark-paid.emits: want paid, got %q", markPaid.Emits)
	}
	if !markPaid.Idempotent || markPaid.IdempotencyKey == nil {
		t.Error("mark-paid idempotency not parsed")
	}
	discount := actions["update-discount-rule"]
	if discount.Uses == nil || discount.Uses.Db == nil || len(discount.Uses.Db.Write) != 1 {
		t.Error("update-discount-rule uses.db.write not parsed")
	}

	// §12 events: payload fields + deliver consequence map
	if len(order.Events) != 1 {
		t.Fatalf("want 1 event, got %d", len(order.Events))
	}
	paid := order.Events[0]
	if paid.Publish == nil || !paid.Publish.Durable {
		t.Error("paid.publish.durable not parsed")
	}
	if paid.Payload == nil || len(paid.Payload.Fields) != 5 {
		t.Error("paid.payload.fields not parsed")
	}
	if len(paid.Deliver) != 5 {
		t.Fatalf("paid.deliver: want 5 entries, got %d", len(paid.Deliver))
	}
	reliable := paid.Deliver[4]
	if reliable.Channel != "reliable_event" {
		t.Errorf("deliver[4].channel: want reliable_event, got %q", reliable.Channel)
	}
	if reliable.Target == nil || reliable.Target.Resource != "gl.journal-entry" || reliable.Target.Action != "create" {
		t.Error("deliver[4].target not parsed")
	}
	if reliable.Retry == nil || reliable.Retry.Max != 10 || reliable.Retry.Backoff != "exponential" {
		t.Error("deliver[4].retry not parsed")
	}
	if reliable.DeadLetter == nil || reliable.DeadLetter.Resource != "failed-event" {
		t.Error("deliver[4].dead_letter not parsed")
	}
	if reliable.IdempotencyKey != "order.paid.{id}" {
		t.Errorf("deliver[4].idempotency_key: got %q", reliable.IdempotencyKey)
	}
}
