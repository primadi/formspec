package entity

import (
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

func mkSM(field, initial string, transitions []spec.TransitionDecl) *spec.EntitySpec {
	return &spec.EntitySpec{
		StateMachine: &spec.StateMachine{
			Field:       field,
			Initial:     initial,
			Transitions: transitions,
		},
	}
}

func TestStateMachine_CanTransition_NoSM(t *testing.T) {
	e := NewStateMachineEngine()
	err := e.CanTransition(nil, "draft", "send", nil)
	if err != nil {
		t.Fatalf("expected no error without state machine, got: %v", err)
	}
}

func TestStateMachine_CanTransition_Valid(t *testing.T) {
	ent := mkSM("status", "draft", []spec.TransitionDecl{
		{From: spec.StateList{"draft"}, To: "posted", Action: "post"},
	})

	e := NewStateMachineEngine()
	err := e.CanTransition(ent, "draft", "post", map[string]any{})
	if err != nil {
		t.Fatalf("expected valid transition, got: %v", err)
	}
}

func TestStateMachine_CanTransition_InvalidAction(t *testing.T) {
	ent := mkSM("status", "draft", []spec.TransitionDecl{
		{From: spec.StateList{"draft"}, To: "posted", Action: "post"},
	})

	e := NewStateMachineEngine()
	err := e.CanTransition(ent, "draft", "delete", map[string]any{})
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
}

func TestStateMachine_CanTransition_InvalidFrom(t *testing.T) {
	ent := mkSM("status", "posted", []spec.TransitionDecl{
		{From: spec.StateList{"draft"}, To: "posted", Action: "post"},
	})

	e := NewStateMachineEngine()
	err := e.CanTransition(ent, "posted", "post", map[string]any{})
	if err == nil {
		t.Fatal("expected error for wrong source state")
	}
}

func TestStateMachine_CanTransition_GuardPasses(t *testing.T) {
	ent := mkSM("status", "draft", []spec.TransitionDecl{
		{
			From:   spec.StateList{"draft"},
			To:     "sent",
			Action: "send",
			Guard:  &spec.GuardDecl{Expression: "total > 0", Message: "total must be positive"},
		},
	})

	e := NewStateMachineEngine()
	err := e.CanTransition(ent, "draft", "send", map[string]any{"total": float64(100)})
	if err != nil {
		t.Fatalf("expected guard to pass, got: %v", err)
	}
}

func TestStateMachine_CanTransition_GuardFails(t *testing.T) {
	ent := mkSM("status", "draft", []spec.TransitionDecl{
		{
			From:   spec.StateList{"draft"},
			To:     "sent",
			Action: "send",
			Guard:  &spec.GuardDecl{Expression: "total > 100", Message: "total is too low"},
		},
	})

	e := NewStateMachineEngine()
	err := e.CanTransition(ent, "draft", "send", map[string]any{"total": float64(50)})
	if err == nil {
		t.Fatal("expected guard to fail")
	}
	if serr, ok := err.(*StateTransitionError); ok {
		if serr.Reason != "total is too low" {
			t.Errorf("expected guard message 'total is too low', got %q", serr.Reason)
		}
	}
}

func TestStateMachine_CanTransition_WildcardFrom(t *testing.T) {
	ent := mkSM("status", "draft", []spec.TransitionDecl{
		{From: spec.StateList{"draft"}, To: "paid", Action: "mark-paid"},
		{From: spec.StateList{"*"}, To: "void", Action: "void"},
	})

	e := NewStateMachineEngine()

	err := e.CanTransition(ent, "draft", "void", map[string]any{})
	if err != nil {
		t.Fatalf("expected wildcard transition to work from draft, got: %v", err)
	}

	err = e.CanTransition(ent, "paid", "void", map[string]any{})
	if err != nil {
		t.Fatalf("expected wildcard transition to work from paid, got: %v", err)
	}
}

func TestStateMachine_CanTransition_MultiFrom(t *testing.T) {
	ent := mkSM("status", "draft", []spec.TransitionDecl{
		{From: spec.StateList{"draft", "awaiting_payment"}, To: "void", Action: "void"},
	})

	e := NewStateMachineEngine()

	err := e.CanTransition(ent, "draft", "void", map[string]any{})
	if err != nil {
		t.Fatalf("expected multi-from to work from draft: %v", err)
	}

	err = e.CanTransition(ent, "awaiting_payment", "void", map[string]any{})
	if err != nil {
		t.Fatalf("expected multi-from to work from awaiting_payment: %v", err)
	}
}

func TestStateMachine_Transition(t *testing.T) {
	ent := mkSM("status", "draft", []spec.TransitionDecl{
		{From: spec.StateList{"draft"}, To: "posted", Action: "post"},
	})

	e := NewStateMachineEngine()
	newState, err := e.Transition(ent, "draft", "post")
	if err != nil {
		t.Fatalf("expected transition to succeed: %v", err)
	}
	if newState != "posted" {
		t.Errorf("expected 'posted', got %q", newState)
	}
}

func TestStateMachine_Initial(t *testing.T) {
	ent := mkSM("status", "draft", nil)
	e := NewStateMachineEngine()

	initial := e.GetInitial(ent)
	if initial != "draft" {
		t.Errorf("expected initial 'draft', got %q", initial)
	}
}

func TestStateMachine_HasStateMachine(t *testing.T) {
	e := NewStateMachineEngine()

	if e.HasStateMachine(nil) {
		t.Error("expected false for nil spec")
	}

	ent := &spec.EntitySpec{}
	if e.HasStateMachine(ent) {
		t.Error("expected false for spec without state machine")
	}

	ent.StateMachine = &spec.StateMachine{Field: "status", Initial: "draft"}
	if !e.HasStateMachine(ent) {
		t.Error("expected true for spec with state machine")
	}
}

func TestStateMachine_GuardSumLine(t *testing.T) {
	ent := mkSM("status", "draft", []spec.TransitionDecl{
		{
			From:   spec.StateList{"draft"},
			To:     "posted",
			Action: "post",
			Guard:  &spec.GuardDecl{Expression: "sum_line_debit == sum_line_credit and sum_line_debit > 0"},
		},
	})

	e := NewStateMachineEngine()

	err := e.CanTransition(ent, "draft", "post", map[string]any{
		"status": "draft",
		"lines": []any{
			map[string]any{"debit": float64(100), "credit": float64(0)},
			map[string]any{"debit": float64(0), "credit": float64(100)},
		},
	})
	if err != nil {
		t.Fatalf("expected guard to pass for balanced lines: %v", err)
	}

	err = e.CanTransition(ent, "draft", "post", map[string]any{
		"status": "draft",
		"lines": []any{
			map[string]any{"debit": float64(100), "credit": float64(0)},
			map[string]any{"debit": float64(0), "credit": float64(0)},
		},
	})
	if err == nil {
		t.Fatal("expected guard to fail for unbalanced lines")
	}
}

func TestStateField(t *testing.T) {
	ent := mkSM("status", "draft", nil)
	if f := StateField(ent); f != "status" {
		t.Errorf("expected 'status', got %q", f)
	}

	if f := StateField(nil); f != "" {
		t.Errorf("expected '' for nil spec, got %q", f)
	}
}
