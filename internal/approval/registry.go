// Package workflow provides the approval registry and engine for the gates
// declared on Entity state-machine transitions
// (`state_machine.transitions[].approval`, 02-core-extended.md §2).
//
// An approval gate HOLDS a transition: it does not execute until every
// applicable step reaches its quorum. Approval is a signed statement recorded in
// the audit trail, and the requester can never approve their own request.
package approval

import (
	"sort"
	"sync"

	"github.com/primadi/formspec/pkg/spec"
)

// Registry maps the approval gates declared on Entity transitions to the
// runtime, keyed by "{module}/{entity}.{transition}".
type Registry struct {
	mu           sync.RWMutex
	approvals    map[string]*spec.ApprovalSpec // key = "module/{entity}.{transition}"
	byTransition map[string][]*spec.ApprovalSpec
	meta         map[*spec.ApprovalSpec]approvalMeta
}

// approvalMeta records where an approval gate was declared, so a stored row can
// be resolved back to its entity and transition.
type approvalMeta struct {
	module     string
	entity     string // "module.entity"
	transition string
	froms      []string
	to         string
}

// NewRegistry creates an empty approval registry.
func NewRegistry() *Registry {
	return &Registry{
		approvals:    make(map[string]*spec.ApprovalSpec),
		byTransition: make(map[string][]*spec.ApprovalSpec),
		meta:         make(map[*spec.ApprovalSpec]approvalMeta),
	}
}

// AddEntity registers every approval gate declared on an entity's transitions.
//
// entity is the entity NAME (not "module.entity"); the qualified
// "{module}.{entity}" form is derived here, so callers pass what the manifest
// declares. A transition with no `approval`, or with no `via` name (an unnamed
// transition cannot be referenced), is skipped.
func (r *Registry) AddEntity(module, entity string, es *spec.EntitySpec) {
	if es == nil || es.StateMachine == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	qualified := module + "." + entity
	for i := range es.StateMachine.Transitions {
		t := &es.StateMachine.Transitions[i]
		if t.Approval == nil || t.Action == "" {
			continue
		}
		key := module + "/" + entity + "." + t.Action
		r.approvals[key] = t.Approval
		r.meta[t.Approval] = approvalMeta{
			module:     module,
			entity:     qualified,
			transition: t.Action,
			froms:      []string(t.From),
			to:         t.To,
		}
		indexKey := qualified + "." + t.Action
		r.byTransition[indexKey] = append(removeApproval(r.byTransition[indexKey], t.Approval), t.Approval)
	}
}

// removeApproval returns list without the given approval pointer.
func removeApproval(list []*spec.ApprovalSpec, target *spec.ApprovalSpec) []*spec.ApprovalSpec {
	out := list[:0]
	for _, a := range list {
		if a != target {
			out = append(out, a)
		}
	}
	return out
}

// Get returns the ApprovalSpec for "{entity}.{transition}" within module, or
// false when absent.
func (r *Registry) Get(module, name string) (*spec.ApprovalSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.approvals[module+"/"+name]
	return a, ok
}

// GetByName resolves an approval gate by its NAME alone
// ("{entity}.{transition}"), returning the module that owns it.
//
// It exists because a ROLE GRANT names the gate the way an author reads it —
// `{ page: "workflow:order.void-order" }` — with no module, and the duty
// permission it materializes to needs the module to be derived
// (`spec.StepPermission`). Resolving that here keeps the `module/name` key
// convention in one place instead of making every caller split strings.
//
// A name declared by two modules is ambiguous. It cannot be detected from the
// map alone (both keys exist), so the ambiguity is reported by the validator
// that materializes grants: whichever module the lookup returns, the grant text
// did not say which one it meant.
func (r *Registry) GetByName(name string) (string, *spec.ApprovalSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for key, a := range r.approvals {
		if module, wfName := splitKey(key); wfName == name {
			return module, a, true
		}
	}
	return "", nil, false
}

// ForTransition returns all approval gates declared on an entity's transition.
//
// entity is "module.entity" (e.g. "gl.journal-entry"); transition is the state
// machine's `via` name. The name is the ONLY reference: the gate is declared on
// the transition itself, so it covers every origin state by construction — the
// hole the state-pair form left (a `from: paid` reference missing the other
// three origins) is not expressible.
//
// The returned slice is a copy; callers must not mutate it.
func (r *Registry) ForTransition(entity, transition string) []*spec.ApprovalSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if transition == "" {
		return nil
	}
	list := r.byTransition[entity+"."+transition]
	out := make([]*spec.ApprovalSpec, len(list))
	copy(out, list)
	return out
}

// NameFor returns the "{module}/{entity}.{transition}" key for a registered
// approval pointer, or "" if it is not registered. Used to persist the gate's
// name (not a pointer address) in approval rows so the escalation worker can
// resolve it back.
func (r *Registry) NameFor(a *spec.ApprovalSpec) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for key, registered := range r.approvals {
		if registered == a {
			return key
		}
	}
	return ""
}

// ApprovalInfo is a lightweight summary of a registered approval gate.
type ApprovalInfo struct {
	Name       string   `json:"name"`
	Module     string   `json:"module"`
	Entity     string   `json:"entity"`
	Transition string   `json:"transition,omitempty"`
	From       []string `json:"from,omitempty"`
	To         string   `json:"to,omitempty"`
}

// List returns a sorted summary of all registered approval gates.
func (r *Registry) List() []ApprovalInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ApprovalInfo, 0, len(r.approvals))
	for key, a := range r.approvals {
		module, name := splitKey(key)
		info := ApprovalInfo{Module: module, Name: name}
		if m, ok := r.meta[a]; ok {
			info.Entity = m.entity
			info.Transition = m.transition
			info.From = m.froms
			info.To = m.to
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Module != out[j].Module {
			return out[i].Module < out[j].Module
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func splitKey(key string) (module, name string) {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return "", key
}
