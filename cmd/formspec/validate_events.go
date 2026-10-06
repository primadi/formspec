// Cross-manifest Event deliver-target validation.
//
// An Entity's `events[].deliver[]` with `channel: reliable_event` names a
// consequence the publisher promises: the outbox worker will make a sync call to
// `target.resource` + `target.action` (02-core-basic.md §12.2). Whether that
// target actually resolves is only answerable with every manifest in view, so
// this check lives beside validateIntegrators / validateApprovals in the CLI's
// cross-manifest layer rather than in the per-manifest loader.
//
// The failure mode it exists to catch previously validated GREEN: kafe's
// `journal-reversed` event declared `target: { resource: gl.gl-balance, action:
// reverse }` while `gl-balance` only ever declared an `update` action. The
// journal reversed correctly, the projection never updated, and the outbox entry
// retried until it exhausted its max and dead-lettered — with `formspec validate`
// reporting 0 problems the whole time.
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// actionSet is what a delivery reference can resolve against: an entity's or
// Service's declared actions, plus whether each is idempotent (7.7.3).
// Package-level so `queue` and `notification` references resolve through one
// function instead of each rebuilding the lookup.
type actionSet struct {
	actions    map[string]bool
	idempotent map[string]bool
}

// validateEventTargets checks every `deliver: channel: reliable_event` target
// against the declared entities and services. Returns source path -> message.
func validateEventTargets(manifests []manifest.RawManifest) map[string]string {
	rejects := map[string]string{}

	// Index what a target can be: an entity action, or a service action.
	entityActions := map[string]*actionSet{}
	serviceActions := map[string]*actionSet{}

	record := func(key string, name string, actions []spec.Action, into map[string]*actionSet) {
		set := &actionSet{actions: map[string]bool{}, idempotent: map[string]bool{}}
		for _, a := range actions {
			set.actions[a.Name] = true
			set.idempotent[a.Name] = a.Idempotent
		}
		// Framework-reserved actions every entity has.
		for _, r := range spec.ReservedActionNames {
			set.actions[r] = true
		}
		into[key] = set
		_ = name
	}

	for _, m := range manifests {
		if m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		switch spec.Kind(m.Kind) {
		case spec.KindEntity:
			es, err := manifest.RawSpecToEntitySpec(specMap)
			if err != nil {
				continue
			}
			record(m.Metadata.Module+"."+m.Metadata.Name, m.Metadata.Name, es.Actions, entityActions)
		case spec.KindService:
			ss, err := manifest.RawSpecToServiceSpec(specMap)
			if err != nil {
				continue
			}
			record(m.Metadata.Module+"."+m.Metadata.Name, m.Metadata.Name, ss.Actions, serviceActions)
		}
	}

	// Report the first failure per source, but sort the traversal so the message
	// is stable across runs.
	sorted := append([]manifest.RawManifest(nil), manifests...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Source < sorted[j].Source })

	for _, m := range sorted {
		if spec.Kind(m.Kind) != spec.KindEntity || m.Spec == nil {
			continue
		}
		specMap, ok := m.Spec.(map[string]any)
		if !ok {
			continue
		}
		es, err := manifest.RawSpecToEntitySpec(specMap)
		if err != nil {
			continue
		}
		ownModule := m.Metadata.Module

		for _, ev := range es.Events {
			for _, d := range ev.Deliver {
				// `queue` (todo 7.7.6): the entry MUST name the Service action it
				// runs. An unresolvable job validates green and then dead-letters
				// at runtime — the consequence silently never happens, which is
				// the failure this file exists to catch.
				if d.Channel == spec.ChannelQueue {
					if strings.TrimSpace(d.Job) == "" {
						rejects[m.Source] = fmt.Sprintf(
							"event %q delivers on channel `queue` without a `job:` — the outbox worker has nothing to call, so the consequence would silently never happen",
							ev.Name)
						continue
					}
					module, svcName, actionName, err := spec.ResolveJobRef(d.Job, ownModule)
					if err != nil {
						rejects[m.Source] = fmt.Sprintf("event %q: %v", ev.Name, err)
						continue
					}
					set, ok := serviceActions[module+"."+svcName]
					if !ok {
						// Service not in this manifest set (a module shipped
						// separately) — cannot verify; skip rather than reject.
						continue
					}
					if !set.actions[actionName] {
						rejects[m.Source] = fmt.Sprintf(
							"event %q names job %q, but Service action %s.%s does not exist — the outbox worker would retry it to dead-letter and the job would silently never run",
							ev.Name, d.Job, module+"."+svcName, actionName)
					}
					continue
				}
				// `notification` (todo 7.7.6): the entry must say WHAT to deliver.
				// `notification:` writes the in-app row (and `recipient` is what
				// `row_scope` matches, so a missing one produces a row nobody can
				// read); `handler:` names the Service action for other channels.
				// Neither = it delivers nothing, which is the failure this whole
				// file exists to catch.
				if d.Channel == spec.ChannelNotification {
					if d.Notification == nil && strings.TrimSpace(d.Handler) == "" {
						rejects[m.Source] = fmt.Sprintf(
							"event %q delivers on channel `notification` with neither `notification:` (in-app) nor `handler:` — it would deliver nothing",
							ev.Name)
						continue
					}
					if d.Notification != nil && strings.TrimSpace(d.Notification.Recipient) == "" {
						rejects[m.Source] = fmt.Sprintf(
							"event %q: `notification` has no `recipient` — the row would have no addressee, and the entity's `row_scope` admits only the addressee, so nobody could ever read it",
							ev.Name)
						continue
					}
					if h := strings.TrimSpace(d.Handler); h != "" {
						if msg := validateServiceActionRef(h, ownModule, serviceActions); msg != "" {
							rejects[m.Source] = fmt.Sprintf("event %q notification `handler:` %s", ev.Name, msg)
						}
					}
					continue
				}
				if d.Channel != spec.ChannelReliableEvent || d.Target == nil {
					continue
				}
				target := d.Target
				if target.Resource == "" || target.Action == "" {
					rejects[m.Source] = fmt.Sprintf(
						"event %q deliver target must name both `resource` and `action` (only one was given) — the outbox worker has nothing to call",
						ev.Name)
					continue
				}
				module, name := splitTargetRef(target.Resource, ownModule)
				key := module + "." + name

				set, ok := entityActions[key]
				if !ok {
					if svcSet, isSvc := serviceActions[key]; isSvc {
						set, ok = svcSet, true
					}
				}
				if !ok {
					// Target not in this manifest set (e.g. a module shipped
					// separately) — cannot verify; skip rather than reject.
					continue
				}
				if !set.actions[target.Action] {
					rejects[m.Source] = fmt.Sprintf(
						"event %q deliver target action %s.%s does not exist — the outbox worker would retry it to dead-letter and the consequence would silently never happen",
						ev.Name, target.Resource, target.Action)
					continue
				}
				// A cross-boundary call is retried by the outbox, so it must be
				// safe to run twice — the same rule validateIntegrators applies
				// (7.7.3).
				if !set.idempotent[target.Action] && entityActions[key] != nil {
					rejects[m.Source] = fmt.Sprintf(
						"event %q deliver target action %s.%s must be idempotent: true — the outbox retries it, so a non-idempotent consequence can be applied twice",
						ev.Name, target.Resource, target.Action)
				}
			}
		}
	}

	return rejects
}

// validateServiceActionRef resolves a `service.action` / `module.service.action`
// reference against the declared Services, returning "" when it is fine or a
// ready-to-append message when it is not.
//
// Shared by `queue`'s `job:` and `notification`'s `handler:` — two fields, one
// reference vocabulary, so the two cannot drift into different rules. A Service
// outside this manifest set is SKIPPED, not rejected: its action cannot be
// verified here (the same rule the delivery-target check uses).
func validateServiceActionRef(ref, ownModule string, serviceActions map[string]*actionSet) string {
	module, svcName, actionName, err := spec.ResolveServiceActionRef(ref, ownModule)
	if err != nil {
		return err.Error()
	}
	set, ok := serviceActions[module+"."+svcName]
	if !ok {
		return ""
	}
	if !set.actions[actionName] {
		return fmt.Sprintf("names %q, but Service action %s.%s does not exist — the outbox worker would retry it to dead-letter and it would silently never run",
			ref, module+"."+svcName, actionName)
	}
	return ""
}

// splitTargetRef resolves a declared target resource ("module.entity") to
// (module, entity), defaulting the module to the publishing entity's own module
// for a bare name. Mirrors resource.splitResourceRef so validation and runtime
// agree on what a target means.
func splitTargetRef(ref, ownModule string) (module, entity string) {
	if i := strings.IndexByte(ref, '.'); i >= 0 {
		return ref[:i], ref[i+1:]
	}
	return ownModule, ref
}
