package db

import (
	"fmt"

	"github.com/primadi/forma/pkg/spec"
)

// LifecycleGuard validates that a reserved action can be executed given the
// current doc_status of a document. Returns nil if the action is allowed,
// or an error if blocked by the lifecycle.
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
		if docStatus != spec.DocStatusDraft && docStatus != "" {
			return &LifecycleError{
				Action:    "update",
				DocStatus: string(docStatus),
				Required:  "draft or lifecycle-free (null)",
				Code:      "FORMA.DOC.UPDATE_NOT_DRAFT",
			}
		}

	case "submit":
		if docStatus != spec.DocStatusDraft {
			return &LifecycleError{
				Action:    "submit",
				DocStatus: string(docStatus),
				Required:  "draft",
				Code:      "FORMA.DOC.SUBMIT_NOT_DRAFT",
			}
		}

	case "cancel":
		if docStatus != spec.DocStatusSubmitted {
			return &LifecycleError{
				Action:    "cancel",
				DocStatus: string(docStatus),
				Required:  "submitted",
				Code:      "FORMA.DOC.CANCEL_NOT_SUBMITTED",
			}
		}

	case "delete":
		if docStatus != spec.DocStatusDraft && docStatus != "" {
			return &LifecycleError{
				Action:    "delete",
				DocStatus: string(docStatus),
				Required:  "draft or lifecycle-free (null)",
				Code:      "FORMA.DOC.DELETE_NOT_DRAFT",
			}
		}

	case "amend", "amend-submit":
		if docStatus != spec.DocStatusSubmitted && docStatus != spec.DocStatusCancelled {
			return &LifecycleError{
				Action:    "amend",
				DocStatus: string(docStatus),
				Required:  "submitted or cancelled",
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
	Required  string
	Code      string
}

func (e *LifecycleError) Error() string {
	status := e.DocStatus
	if status == "" {
		status = "(null — lifecycle-free)"
	}
	return fmt.Sprintf("[%s] action %q requires doc_status=%s, got %s", e.Code, e.Action, e.Required, status)
}
