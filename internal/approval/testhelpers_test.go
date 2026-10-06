package approval

import "github.com/primadi/formspec/pkg/spec"

// regAdd builds a one-transition entity carrying an approval gate and registers
// it, returning the gate that was registered. It exists so a test states the
// gate (steps) and the transition it gates, not the boilerplate of an entity
// state machine — the registry derives gates from entities now.
func regAdd(reg *Registry, module, entity, transition string, froms []string, to string, steps ...spec.ApprovalStep) *spec.ApprovalSpec {
	approval := &spec.ApprovalSpec{Steps: steps}
	es := &spec.EntitySpec{
		Version: "v1",
		StateMachine: &spec.StateMachine{
			Field:   "status",
			Initial: froms[0],
			States:  []spec.StateDecl{{Name: froms[0]}, {Name: to}},
			Transitions: []spec.TransitionDecl{{
				From:     spec.StateList(froms),
				To:       to,
				Action:   transition,
				Approval: approval,
			}},
		},
	}
	reg.AddEntity(module, entity, es)
	return approval
}

// gateName is the derived name a stored approval row carries for a gate:
// "{entity}.{transition}".
func gateName(entity, transition string) string { return entity + "." + transition }
