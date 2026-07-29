package db

import (
	"testing"

	"github.com/primadi/forma/pkg/spec"
)

func TestLifecycleGuard_Create(t *testing.T) {
	// create is always allowed regardless of doc_status
	for _, ds := range []spec.DocStatus{"", spec.DocStatusDraft, spec.DocStatusSubmitted, spec.DocStatusCancelled} {
		if err := LifecycleGuard("create", ds); err != nil {
			t.Errorf("create should be allowed for doc_status=%q, got error: %v", ds, err)
		}
	}
}

func TestLifecycleGuard_Update(t *testing.T) {
	tests := []struct {
		docStatus spec.DocStatus
		allowed   bool
	}{
		{"", true},                       // lifecycle-free
		{spec.DocStatusDraft, true},      // draft
		{spec.DocStatusSubmitted, false}, // submitted
		{spec.DocStatusCancelled, false}, // cancelled
	}

	for _, tt := range tests {
		err := LifecycleGuard("update", tt.docStatus)
		if tt.allowed && err != nil {
			t.Errorf("update with doc_status=%q should be allowed, got: %v", tt.docStatus, err)
		}
		if !tt.allowed && err == nil {
			t.Errorf("update with doc_status=%q should be blocked", tt.docStatus)
		}
	}
}

func TestLifecycleGuard_Submit(t *testing.T) {
	if err := LifecycleGuard("submit", spec.DocStatusDraft); err != nil {
		t.Errorf("submit from draft should be allowed: %v", err)
	}
	if err := LifecycleGuard("submit", spec.DocStatusSubmitted); err == nil {
		t.Error("submit from submitted should be blocked")
	}
	if err := LifecycleGuard("submit", spec.DocStatusCancelled); err == nil {
		t.Error("submit from cancelled should be blocked")
	}
	if err := LifecycleGuard("submit", ""); err == nil {
		t.Error("submit from lifecycle-free should be blocked")
	}
}

func TestLifecycleGuard_Cancel(t *testing.T) {
	if err := LifecycleGuard("cancel", spec.DocStatusSubmitted); err != nil {
		t.Errorf("cancel from submitted should be allowed: %v", err)
	}
	if err := LifecycleGuard("cancel", spec.DocStatusDraft); err == nil {
		t.Error("cancel from draft should be blocked")
	}
	if err := LifecycleGuard("cancel", spec.DocStatusCancelled); err == nil {
		t.Error("cancel from cancelled should be blocked")
	}
}

func TestLifecycleGuard_Delete(t *testing.T) {
	if err := LifecycleGuard("delete", spec.DocStatusDraft); err != nil {
		t.Errorf("delete draft should be allowed: %v", err)
	}
	if err := LifecycleGuard("delete", ""); err != nil {
		t.Errorf("delete lifecycle-free should be allowed: %v", err)
	}
	if err := LifecycleGuard("delete", spec.DocStatusSubmitted); err == nil {
		t.Error("delete submitted should be blocked")
	}
	if err := LifecycleGuard("delete", spec.DocStatusCancelled); err == nil {
		t.Error("delete cancelled should be blocked")
	}
}

func TestLifecycleGuard_Amend(t *testing.T) {
	if err := LifecycleGuard("amend", spec.DocStatusSubmitted); err != nil {
		t.Errorf("amend from submitted should be allowed: %v", err)
	}
	if err := LifecycleGuard("amend", spec.DocStatusCancelled); err != nil {
		t.Errorf("amend from cancelled should be allowed: %v", err)
	}
	if err := LifecycleGuard("amend", spec.DocStatusDraft); err == nil {
		t.Error("amend from draft should be blocked")
	}
	if err := LifecycleGuard("amend", ""); err == nil {
		t.Error("amend from lifecycle-free should be blocked")
	}
}

func TestLifecycleGuard_CreateSubmit(t *testing.T) {
	// create-submit is treated as create — always allowed
	for _, ds := range []spec.DocStatus{"", spec.DocStatusDraft, spec.DocStatusSubmitted} {
		if err := LifecycleGuard("create-submit", ds); err != nil {
			t.Errorf("create-submit should be allowed for doc_status=%q, got: %v", ds, err)
		}
	}
}

func TestLifecycleGuard_AmendSubmit(t *testing.T) {
	// amend-submit is treated as amend — only from submitted or cancelled
	if err := LifecycleGuard("amend-submit", spec.DocStatusSubmitted); err != nil {
		t.Errorf("amend-submit from submitted should be allowed: %v", err)
	}
	if err := LifecycleGuard("amend-submit", spec.DocStatusCancelled); err != nil {
		t.Errorf("amend-submit from cancelled should be allowed: %v", err)
	}
	if err := LifecycleGuard("amend-submit", spec.DocStatusDraft); err == nil {
		t.Error("amend-submit from draft should be blocked")
	}
}

func TestTransitiveDisabled(t *testing.T) {
	// submit disabled → cancel + amend implicitly disabled
	disabled := map[string]bool{"submit": true}
	result := TransitiveDisabled(disabled)
	if !result["cancel"] {
		t.Error("cancel should be transitively disabled when submit is disabled")
	}
	if !result["amend"] {
		t.Error("amend should be transitively disabled when submit is disabled")
	}

	// cancel disabled → amend implicitly disabled
	disabled2 := map[string]bool{"cancel": true}
	result2 := TransitiveDisabled(disabled2)
	if !result2["amend"] {
		t.Error("amend should be transitively disabled when cancel is disabled")
	}
	if result2["submit"] {
		t.Error("submit should NOT be transitively disabled when cancel is disabled")
	}
}

func TestDeriveReservedActions(t *testing.T) {
	// Both create and submit enabled → derive create-submit
	derived := DeriveReservedActions(
		map[string]bool{},
		map[string]bool{},
	)
	if len(derived) != 2 {
		t.Fatalf("expected 2 derived actions, got %d", len(derived))
	}
	if derived[0] != "create-submit" || derived[1] != "amend-submit" {
		t.Errorf("expected [create-submit, amend-submit], got %v", derived)
	}

	// create disabled → no create-submit
	derived2 := DeriveReservedActions(
		map[string]bool{"create": true},
		map[string]bool{},
	)
	for _, d := range derived2 {
		if d == "create-submit" {
			t.Error("create-submit should NOT be derived when create is disabled")
		}
	}

	// submit disabled → neither derived
	derived3 := DeriveReservedActions(
		map[string]bool{"submit": true},
		map[string]bool{},
	)
	if len(derived3) != 0 {
		t.Errorf("expected 0 derived actions when submit is disabled, got %d", len(derived3))
	}

	// Already declared → no duplicate
	derived4 := DeriveReservedActions(
		map[string]bool{},
		map[string]bool{"create-submit": true, "amend-submit": true},
	)
	if len(derived4) != 0 {
		t.Errorf("expected 0 derived actions when already declared, got %d", len(derived4))
	}
}

func TestLifecycleError(t *testing.T) {
	err := &LifecycleError{
		Action:    "submit",
		DocStatus: "cancelled",
		Code:      "FORMA.DOC.SUBMIT_NOT_DRAFT",
	}
	expected := "[FORMA.DOC.SUBMIT_NOT_DRAFT] action \"submit\" blocked by doc_status=cancelled"
	if err.Error() != expected {
		t.Errorf("error message mismatch:\n  got:      %s\n  expected: %s", err.Error(), expected)
	}
}
