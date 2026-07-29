package db

import (
	"fmt"

	"github.com/primadi/forma/pkg/spec"
)

// LifecycleGuard validates that a reserved action can be executed given the
// current doc_status of a document. Returns nil if the action is allowed,
// or an error if blocked by the lifecycle. Returns specific error codes
// per error-glossary.yaml.
//
// Guards per spec §4.1b:
//
//	create       → always allowed
//	update       → doc_status == draft OR doc_status IS NULL
//	submit       → doc_status == draft
//	cancel       → doc_status == submitted
//	delete       → doc_status == draft OR doc_status IS NULL
//	amend        → doc_status == submitted OR doc_status == cancelled
//	create-submit → treated as create (auto-derived)
//	amend-submit  → treated as amend (auto-derived)
func LifecycleGuard(actionName string, docStatus spec.DocStatus) error {
	switch actionName {
	case "create", "create-submit":
		// Always allowed
		return nil

	case "update":
		if docStatus == spec.DocStatusSubmitted {
			return &LifecycleError{
				Action:    "update",
				DocStatus: string(docStatus),
				Code:      "FORMA.DOC.UPDATE_NOT_DRAFT",
			}
		}
		if docStatus == spec.DocStatusCancelled {
			return &LifecycleError{
				Action:    "update",
				DocStatus: string(docStatus),
				Code:      "FORMA.DOC.UPDATE_NOT_DRAFT",
			}
		}

	case "submit":
		if docStatus == spec.DocStatusSubmitted {
			return &LifecycleError{
				Action:    "submit",
				DocStatus: string(docStatus),
				Code:      "FORMA.DOC.ALREADY_SUBMITTED",
			}
		}
		if docStatus == spec.DocStatusCancelled {
			return &LifecycleError{
				Action:    "submit",
				DocStatus: string(docStatus),
				Code:      "FORMA.DOC.SUBMIT_NOT_DRAFT",
			}
		}
		if docStatus == "" {
			return &LifecycleError{
				Action:    "submit",
				DocStatus: "null",
				Code:      "FORMA.DOC.SUBMIT_NOT_DRAFT",
			}
		}

	case "cancel":
		if docStatus == spec.DocStatusCancelled {
			return &LifecycleError{
				Action:    "cancel",
				DocStatus: string(docStatus),
				Code:      "FORMA.DOC.ALREADY_CANCELLED",
			}
		}
		if docStatus == spec.DocStatusDraft {
			return &LifecycleError{
				Action:    "cancel",
				DocStatus: string(docStatus),
				Code:      "FORMA.DOC.CANCEL_NOT_SUBMITTED",
			}
		}
		if docStatus == "" {
			return &LifecycleError{
				Action:    "cancel",
				DocStatus: "null",
				Code:      "FORMA.DOC.CANCEL_NOT_SUBMITTED",
			}
		}

	case "delete":
		if docStatus == spec.DocStatusSubmitted {
			return &LifecycleError{
				Action:    "delete",
				DocStatus: string(docStatus),
				Code:      "FORMA.DOC.DELETE_NOT_DRAFT",
			}
		}
		if docStatus == spec.DocStatusCancelled {
			return &LifecycleError{
				Action:    "delete",
				DocStatus: string(docStatus),
				Code:      "FORMA.DOC.DELETE_NOT_DRAFT",
			}
		}

	case "amend", "amend-submit":
		if docStatus != spec.DocStatusSubmitted && docStatus != spec.DocStatusCancelled {
			return &LifecycleError{
				Action:    "amend",
				DocStatus: string(docStatus),
				Code:      "FORMA.DOC.AMEND_NOT_SUBMITTED_OR_CANCELLED",
			}
		}
	}

	return nil
}

// IsReservedLifecycleAction returns true if the action name is one of the
// reserved actions that require lifecycle guard checking.
func IsReservedLifecycleAction(name string) bool {
	return spec.IsReservedAction(name)
}

// DeriveReservedActions returns the list of auto-derived reserved actions
// that should be available based on which standard reserved actions are enabled.
//
// Rules:
//   - If create + submit are both enabled → auto-derive "create-submit"
//   - If amend + submit are both enabled → auto-derive "amend-submit"
//
// Parameters:
//   - disabledActions: set of explicitly disabled reserved action names
//   - existingActions: names of already-declared actions (avoid duplicates)
func DeriveReservedActions(disabledActions map[string]bool, existingActions map[string]bool) []string {
	var derived []string

	createEnabled := !disabledActions["create"]
	submitEnabled := !disabledActions["submit"]
	amendEnabled := !disabledActions["amend"]

	if createEnabled && submitEnabled && !existingActions["create-submit"] {
		derived = append(derived, "create-submit")
	}
	if amendEnabled && submitEnabled && !existingActions["amend-submit"] {
		derived = append(derived, "amend-submit")
	}

	return derived
}

// TransitiveDisabled returns the set of actions that are implicitly disabled
// due to transitive gating rules:
//   - submit disabled → cancel and amend are implicitly disabled
//   - cancel disabled → amend is implicitly disabled
func TransitiveDisabled(disabledActions map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for k, v := range disabledActions {
		result[k] = v
	}

	if result["submit"] {
		result["cancel"] = true
		result["amend"] = true
	}
	if result["cancel"] {
		result["amend"] = true
	}

	return result
}

// LifecycleError is returned when a lifecycle guard blocks an action.
type LifecycleError struct {
	Action    string
	DocStatus string
	Code      string
}

func (e *LifecycleError) Error() string {
	if e.DocStatus == "" {
		return fmt.Sprintf("[%s] action %q blocked: referential integrity violation", e.Code, e.Action)
	}
	return fmt.Sprintf("[%s] action %q blocked by doc_status=%s", e.Code, e.Action, e.DocStatus)
}
