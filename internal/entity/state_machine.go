// Package entity — State Machine Engine
//
// This file implements the state machine engine per spec §14.
// It evaluates state transitions, guards, and enforces valid transitions.
package entity

import (
	"fmt"
	"strings"

	"github.com/primadi/formspec/internal/starlark"
	"github.com/primadi/formspec/pkg/spec"
)

// StateMachineEngine evaluates and enforces state machine transitions.
type StateMachineEngine struct{}

// NewStateMachineEngine creates a new state machine engine.
func NewStateMachineEngine() *StateMachineEngine {
	return &StateMachineEngine{}
}

// CanTransition checks whether the given action is a valid transition from the
// current state. It evaluates any guard conditions on the matching transition.
//
// Parameters:
//   - entitySpec: the entity definition containing the state_machine block
//   - currentState: the current value of the state field (e.g. "draft")
//   - actionName: the action being invoked (e.g. "send", "post")
//   - resourceData: the full record data for guard evaluation
//
// Returns nil if the transition is valid, or an error explaining why it's not.
func (e *StateMachineEngine) CanTransition(
	entitySpec *spec.EntitySpec,
	currentState string,
	actionName string,
	resourceData map[string]any,
) error {
	if entitySpec == nil || entitySpec.StateMachine == nil {
		return nil // no state machine → no restrictions
	}

	sm := entitySpec.StateMachine

	// If no current state, use initial
	if currentState == "" {
		currentState = sm.Initial
	}

	// Find a matching transition
	trans := e.findTransition(sm, currentState, actionName)
	if trans == nil {
		return &StateTransitionError{
			From:   currentState,
			Action: actionName,
			Reason: "no transition defined for this action from the current state",
		}
	}

	// Evaluate guard
	if trans.Guard != nil && trans.Guard.Expression != "" {
		passed, msg, err := e.evaluateGuard(trans.Guard, resourceData)
		if err != nil {
			return fmt.Errorf("state machine guard error: %w", err)
		}
		if !passed {
			if msg == "" {
				msg = "guard condition not met"
			}
			return &StateTransitionError{
				From:   currentState,
				Action: actionName,
				Reason: msg,
			}
		}
	}

	return nil
}

// Transition returns the new state after executing the given action from the
// current state. Assumes CanTransition has already been called and passed.
func (e *StateMachineEngine) Transition(
	entitySpec *spec.EntitySpec,
	currentState string,
	actionName string,
) (string, error) {
	if entitySpec == nil || entitySpec.StateMachine == nil {
		return currentState, nil
	}

	sm := entitySpec.StateMachine
	if currentState == "" {
		currentState = sm.Initial
	}

	trans := e.findTransition(sm, currentState, actionName)
	if trans == nil {
		return currentState, &StateTransitionError{
			From:   currentState,
			Action: actionName,
			Reason: "no transition defined",
		}
	}

	return trans.To, nil
}

// GetInitial returns the initial state from the state machine definition.
func (e *StateMachineEngine) GetInitial(entitySpec *spec.EntitySpec) string {
	if entitySpec == nil || entitySpec.StateMachine == nil {
		return ""
	}
	return entitySpec.StateMachine.Initial
}

// HasStateMachine returns true if the entity has a state machine defined.
func (e *StateMachineEngine) HasStateMachine(entitySpec *spec.EntitySpec) bool {
	return entitySpec != nil && entitySpec.StateMachine != nil
}

// findTransition locates a transition from the given state via the given action.
func (e *StateMachineEngine) findTransition(sm *spec.StateMachine, currentState, actionName string) *spec.TransitionDecl {
	for i := range sm.Transitions {
		t := &sm.Transitions[i]
		if t.Action == actionName && t.From.Matches(currentState) {
			return t
		}
	}
	return nil
}

// evaluateGuard evaluates a guard condition using the shared guard evaluator
// (internal/starlark.EvaluateGuard — todo 7.5.4). The guard expression has
// access to resource data fields directly, plus pre-computed sum_line_* /
// item_count / line_count helpers.
func (e *StateMachineEngine) evaluateGuard(guard *spec.GuardDecl, resourceData map[string]any) (bool, string, error) {
	if guard == nil || guard.Expression == "" {
		return true, "", nil
	}
	passed, _, err := starlark.EvaluateGuard(guard.Expression, resourceData)
	if err != nil {
		return false, "", err
	}
	msg := ""
	if !passed {
		msg = guard.Message
	}
	return passed, msg, nil
}

// StateTransitionError is returned when a state transition is invalid.
type StateTransitionError struct {
	From   string
	Action string
	Reason string
}

func (e *StateTransitionError) Error() string {
	return fmt.Sprintf("STATE_TRANSITION_ERROR: cannot transition from %q via %q: %s",
		e.From, e.Action, e.Reason,
	)
}

// ─── Field name resolution ───

// StateField returns the name of the state field from the entity spec.
func StateField(entitySpec *spec.EntitySpec) string {
	if entitySpec == nil || entitySpec.StateMachine == nil {
		return ""
	}
	return entitySpec.StateMachine.Field
}

// NormaliseActionName converts action name aliases (spec.TransitionDecl accepts
// both "via" and "action" in YAML, both stored in TransitionDecl.Action).
func NormaliseActionName(name string) string {
	return strings.TrimSpace(name)
}
