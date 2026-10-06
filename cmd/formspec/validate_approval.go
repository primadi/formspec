// Cross-manifest approval validation.
//
// An approval gate is declared ON the state-machine transition it holds
// (`state_machine.transitions[].approval`), so the transition reference cannot
// dangle — there is nothing left to resolve. What this file checks is what a
// single manifest cannot: the shape rules the loader owns, plus the two
// cross-manifest lookups that used to validate green while the approval could
// never be satisfied:
//
//  1. display_fields naming a field the entity does not declare — the
//     ApprovalInbox then shows an empty column, which reads as "nothing to
//     decide on" rather than "typo".
//  2. roles that no role record provides — compared LITERALLY at runtime, so a
//     typo or an invented namespace (`cafe-order.supervisor` against a role
//     named `supervisor`) produced an approval that could never reach quorum
//     while validate stayed green.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/formspec/internal/auth"
	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// entityFieldIndex maps "{module}.{entity}" to the set of its declared field
// names — used to validate ApprovalStep.display_fields (S15): a display field
// that names no field would render an empty column in the ApprovalInbox, which
// reads as "the approver has nothing to decide on" rather than "typo".
type entityFieldIndex map[string]map[string]bool

// roleNameIndex is the set of role names the spec tree declares, from every
// `kind: Seed` record whose entity is the role entity.
//
// It exists because a workflow's `steps[].roles` is a NAME LOOKUP that nothing
// checked. Kafe shipped `roles: [cafe-order.supervisor]` while the seeded role
// is named `supervisor`: validate answered 89 manifests / 0 problems, and the
// void approval could never be granted — `CanApprove` refuses, the inbox is
// empty for the only approver who exists, and the failure is invisible until
// someone tries to approve. Roles the framework seeds itself (the owner roles
// `SeedOwnerRoles` creates at runtime) are not manifests, so they are added
// explicitly rather than guessed.
type roleNameIndex map[string]bool

// buildRoleNameIndex collects the declared role names. Keyed by name only, not
// by workspace: a manifest declares roles for the deployment it ships in, and
// the check is "does this name exist anywhere in this spec tree".
func buildRoleNameIndex(manifests []manifest.RawManifest) roleNameIndex {
	index := roleNameIndex{}
	for _, name := range frameworkRoleNames {
		index[name] = true
	}

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindSeed || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		seed, err := manifest.RawSpecTo[spec.SeedSpec](specMap)
		if err != nil || seed == nil {
			continue
		}
		for _, entity := range seed.Entities {
			if !isRoleEntityName(entity.Entity) {
				continue
			}
			for _, rec := range entity.Records {
				if name, ok := rec["name"].(string); ok && strings.TrimSpace(name) != "" {
					index[strings.TrimSpace(name)] = true
				}
			}
		}
	}
	return index
}

// frameworkRoleNames are the roles the AUTH SERVICE seeds itself
// (`auth.Service.SeedOwnerRoles`) — they have no manifest, so a workflow may
// legitimately name them and no spec-tree search would find them.
var frameworkRoleNames = []string{
	auth.RoleWorkspaceOwner,
	auth.RoleCloudOwner,
	auth.RoleAppOwner,
	auth.RoleModuleOwner,
}

// validateApprovals checks the approval gates declared on entities' state-machine
// transitions. Returns a map of manifest source → error message.
//
// Since an approval gate IS declared on the transition it holds, the transition
// reference can no longer dangle — there is nothing to resolve it against. What
// remains cross-manifest is what a single manifest cannot see: a step's
// `display_fields` must name real entity fields, and every role a step names must
// exist somewhere in the spec tree.
func validateApprovals(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}
	fields := buildEntityFieldIndex(manifests)
	roles := buildRoleNameIndex(manifests)

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || es == nil || es.StateMachine == nil {
			continue
		}
		qualified := m.Metadata.Module + "." + m.Metadata.Name
		for i := range es.StateMachine.Transitions {
			t := es.StateMachine.Transitions[i]
			if t.Approval == nil {
				continue
			}
			if msg := approvalDisplayFieldError(qualified, t.Approval, fields); msg != "" {
				rejects[m.Source] = msg
				break
			}
			if msg := approvalRoleError(qualified, t.Approval, roles); msg != "" {
				rejects[m.Source] = msg
				break
			}
		}
	}

	return rejects
}

// scanApprovalRoleOnlySteps reports approval steps that rest on `roles` alone,
// where a DUTY would do (todo 5.13.12).
//
// Advisory, not an error: such a step WORKS — `CanApprove` accepts a role
// holder — so failing the run would break manifests that are fine. What is worth
// saying is that it couples the manifest to role NAMES, which is what AGENTS.md
// rule 6 asks authors to avoid: a duty is `resource + action`, granted (and
// revoked) from the role seed, so renaming a role does not silently disable an
// approval.
//
// `mode: sequential` is EXEMPT, and that is the whole reason this is a
// deprecation rather than a removal: a chain is ordered BY its roles, and a duty
// is a flat permission with no position in that order — `validateApprovalStepMode`
// refuses the combination, so `roles` is the only form a chain can take.
func scanApprovalRoleOnlySteps(manifests []manifest.RawManifest) []honestyIssue {
	var issues []honestyIssue

	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil || es == nil || es.StateMachine == nil {
			continue
		}
		for i := range es.StateMachine.Transitions {
			t := es.StateMachine.Transitions[i]
			if t.Approval == nil {
				continue
			}
			for j, step := range t.Approval.Steps {
				if step.Mode == spec.StepModeSequential {
					continue // roles ARE the chain — the only form that can order it
				}
				if len(step.Roles) == 0 || strings.TrimSpace(step.Permission) != "" {
					continue
				}
				where := fmt.Sprintf("step %d", j)
				if step.Name != "" {
					where = fmt.Sprintf("step %d (%s)", j, step.Name)
				}
				issues = append(issues, honestyIssue{
					Source:   m.Source,
					Severity: "warning",
					Message: fmt.Sprintf(
						"transition %s->%s %s rests on `roles: %s` alone — it works, but it hardcodes role NAMES in the manifest (AGENTS.md rule 6). A `permission:` duty would be granted from the role seed instead, so renaming a role cannot silently disable this approval: add `permission: <duty-name>` and grant it with `{ page: \"workflow:%s.%s\", actions: [{name: %s}] }`",
						t.From, t.To, where, strings.Join(step.Roles, ", "),
						m.Metadata.Name, t.Action, step.Name),
				})
			}
		}
	}

	return issues
}

// buildEntityFieldIndex collects each entity's declared field names, keyed by
// "{module}.{entity}", for display_fields validation.
func buildEntityFieldIndex(manifests []manifest.RawManifest) entityFieldIndex {
	index := entityFieldIndex{}
	for _, m := range manifests {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		entitySpec, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil {
			continue
		}
		names := map[string]bool{}
		for _, f := range entitySpec.Fields {
			names[f.Name] = true
		}
		index[m.Metadata.Module+"."+m.Metadata.Name] = names
	}
	return index
}

// approvalRoleError reports the first role an approval chain names that no role
// record (or framework owner role) provides. Returns "" when all resolve.
//
// `roles` (who may approve the step) is the only role lookup left in an approval:
// escalation used to name roles too, but a takeover is now a DUTY permission, so
// it is checked by the grant-satisfiability test instead. A role name that
// resolves nowhere produces an approval that LOOKS configured and can never be
// satisfied — measured on kafe, where `roles: [cafe-order.supervisor]` met a
// seeded role named `supervisor` and the void approval answered 403 forever while
// validate stayed green.
//
// The index is deliberately exact-match: role names are compared literally at
// runtime (`workflow.hasAnyRole`), so accepting a near-miss here would be
// telling the author something the engine does not agree with.
//
// A tree that declares NO roles does not disable the check — it decides it: the
// names are then certainly unresolvable, and the remedy is to declare the role
// (or to drop the approval). Skipping that case would leave the loudest instance
// of the bug unreported, which is how the crc-management approvals naming
// `crc.cap-approver` sat unnoticed in a tree with no role seed at all.
func approvalRoleError(entity string, a *spec.ApprovalSpec, roles roleNameIndex) string {
	missing := func(where string, names []string) string {
		for _, n := range names {
			if strings.TrimSpace(n) == "" {
				continue
			}
			if roles[n] {
				continue
			}
			return fmt.Sprintf(
				"approval on %s %s names role %q, which no role declares — the step could never be approved (declared roles: %s)",
				entity, where, n, describeRoleNames(roles))
		}
		return ""
	}
	for i, step := range a.Steps {
		where := fmt.Sprintf("step %d roles", i)
		if step.Title != "" {
			where = fmt.Sprintf("step %d (%s) roles", i, step.Title)
		}
		if msg := missing(where, step.Roles); msg != "" {
			return msg
		}
	}
	return ""
}

// describeRoleNames renders the known roles for an error message, framework
// roles included so the list matches what the check accepts.
func describeRoleNames(roles roleNameIndex) string {
	names := make([]string, 0, len(roles))
	for n := range roles {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// approvalDisplayFieldError validates every step's display_fields against the
// entity's declared fields (S15). Returns "" when all resolve.
func approvalDisplayFieldError(entity string, a *spec.ApprovalSpec, fields entityFieldIndex) string {
	known, ok := fields[entity]
	if !ok {
		return ""
	}
	for i, step := range a.Steps {
		for _, f := range step.DisplayFields {
			if !known[f] {
				return fmt.Sprintf("approval on %s step %d display_fields names %q, which is not a field of %s — the ApprovalInbox would show an empty column", entity, i, f, entity)
			}
		}
	}
	return ""
}

// containsString reports whether list contains want.
func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
