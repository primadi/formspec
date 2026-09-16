// Package summary implements the summary-projection rebuild contract
// (docs/spec/backend/02-core-extended.md §6, todo 3.6.4).
//
// A summary Entity is a projection: it holds no data of its own and is written
// exclusively by durable events. That makes it disposable — it can always be
// recomputed from the source data, which is why backups exclude it (§6). This
// package resolves *how* that recomputation happens (`Plan`) from the Entity's
// declared `sources` / `join_key` / `rebuild` metadata, and names the durable
// streams to replay.
//
// Replaying the events themselves lives in internal/subscription
// (StreamingWorker.ReplaySummaryProjection), so a rebuild goes through the same
// filter → transform → handler path as live delivery.
package summary

import (
	"fmt"
	"sort"
	"strings"

	"github.com/primadi/formspec/internal/entity"
	"github.com/primadi/formspec/internal/subscription"
	"github.com/primadi/formspec/pkg/spec"
)

// StreamRef is one durable stream that feeds a summary projection — the
// fully-qualified event plus the durable subscription that consumes it.
type StreamRef struct {
	EventName    string
	Resource     string
	Subscription string
}

// Plan is the resolved rebuild plan for one summary Entity.
type Plan struct {
	Module   string
	Entity   string
	Plural   string
	Table    string
	Schema   string
	Strategy string // full | partial | none; an absent `rebuild` block means full
	JoinKey  string
	Window   string
	Since    string
	Sources  []string
	// Streams are the durable streams to replay, one row per
	// (event, subscription).
	Streams []StreamRef
	// Orphaned lists declared sources that no durable subscription listens
	// to. They cannot be replayed, so the rebuild is necessarily partial —
	// surfaced rather than silently ignored.
	Orphaned []string
}

// Resolve resolves an entity reference to a registered Entity. ref accepts
// "module/entity", "module.entity", or a bare entity name — the latter only
// among `characteristic: summary` entities, so a rebuild can never be pointed
// at a non-summary Entity by accident.
func Resolve(reg *entity.Registry, ref string) (module, name string, es *spec.EntitySpec, err error) {
	if canon, ok := spec.NormalizeEntityRef(ref); ok {
		m, n, _ := strings.Cut(canon, "/")
		info, ok := reg.GetEntity(m, n)
		if !ok {
			return "", "", nil, fmt.Errorf("no Entity %q in the spec tree", ref)
		}
		if info.EntitySpec == nil {
			return "", "", nil, fmt.Errorf("Entity %q has no parsed spec", ref)
		}
		return m, n, info.EntitySpec, nil
	}

	var matches []entity.EntityInfo
	for _, e := range reg.GetEntitiesByCharacteristic(spec.CharSummary) {
		if e.Name == ref {
			matches = append(matches, e)
		}
	}
	switch len(matches) {
	case 0:
		known := reg.GetEntitiesByCharacteristic(spec.CharSummary)
		var names []string
		for _, k := range known {
			names = append(names, k.Module+"/"+k.Name)
		}
		sort.Strings(names)
		if len(names) == 0 {
			return "", "", nil, fmt.Errorf("no summary Entity %q — the spec tree declares no `characteristic: summary` entities", ref)
		}
		return "", "", nil, fmt.Errorf("no summary Entity %q; known summary entities: %s", ref, strings.Join(names, ", "))
	case 1:
		m, n := matches[0].Module, matches[0].Name
		info, ok := reg.GetEntity(m, n)
		if !ok || info.EntitySpec == nil {
			return "", "", nil, fmt.Errorf("summary Entity %s/%s is registered but has no parsed spec", m, n)
		}
		return m, n, info.EntitySpec, nil
	default:
		var names []string
		for _, m := range matches {
			names = append(names, m.Module+"/"+m.Name)
		}
		return "", "", nil, fmt.Errorf("entity name %q is ambiguous across modules: %s (qualify it as module/entity)", ref, strings.Join(names, ", "))
	}
}

// PlanRebuild resolves the rebuild plan for the summary Entity named by ref,
// naming the durable streams (from subReg) that must be replayed.
//
// It does not touch the database or the stream — a plan is cheap and safe to
// print (`--dry-run`). An Entity that declares no `sources` is rejected: there
// is nothing to replay from, and guessing would produce a silently empty
// projection.
func PlanRebuild(reg *entity.Registry, subReg *subscription.Registry, ref string) (*Plan, error) {
	module, name, es, err := Resolve(reg, ref)
	if err != nil {
		return nil, err
	}
	if es.Characteristic != spec.CharSummary {
		return nil, fmt.Errorf("%s/%s has characteristic %q — only summary entities are rebuilt from events", module, name, es.Characteristic)
	}
	if len(es.Sources) == 0 {
		return nil, fmt.Errorf("%s/%s declares no `sources` — nothing to replay (see docs/spec/backend/02-core-extended.md §6)", module, name)
	}

	strategy := "full"
	plan := &Plan{
		Module:   module,
		Entity:   name,
		Plural:   es.Plural,
		Strategy: strategy,
		JoinKey:  es.JoinKey,
	}
	if es.Rebuild != nil {
		if es.Rebuild.Strategy != "" {
			plan.Strategy = es.Rebuild.Strategy
		}
		plan.Window = es.Rebuild.Window
		plan.Since = es.Rebuild.Since
	}
	if plan.Strategy == "none" {
		return nil, fmt.Errorf("%s/%s declares rebuild.strategy: none — it is not recomputed from events, so there is nothing to rebuild", module, name)
	}

	if info, ok := reg.GetEntity(module, name); ok && info.TableInfo != nil {
		plan.Table = info.TableInfo.TableName
		plan.Schema = info.TableInfo.Schema
	}

	sources := normalizeSources(module, es.Sources)
	plan.Sources = sources

	// Match every durable subscription's declared events against the sources.
	// Event names are "{module}.{entity}.{event}", resources "module/entity".
	seen := map[string]bool{}
	for _, sub := range subReg.Durable() {
		subKey := sub.Module + "/" + sub.Name
		for _, ev := range sub.Spec.Events {
			for _, src := range sources {
				if !eventBelongsToResource(ev, src) {
					continue
				}
				key := ev + "\x00" + subKey
				if seen[key] {
					continue
				}
				seen[key] = true
				plan.Streams = append(plan.Streams, StreamRef{EventName: ev, Resource: src, Subscription: subKey})
			}
		}
	}
	sort.Slice(plan.Streams, func(i, j int) bool {
		if plan.Streams[i].EventName != plan.Streams[j].EventName {
			return plan.Streams[i].EventName < plan.Streams[j].EventName
		}
		return plan.Streams[i].Subscription < plan.Streams[j].Subscription
	})

	// Sources nothing listens to cannot be replayed — report, don't hide.
	covered := map[string]bool{}
	for _, s := range plan.Streams {
		covered[s.Resource] = true
	}
	for _, src := range sources {
		if !covered[src] {
			plan.Orphaned = append(plan.Orphaned, src)
		}
	}
	sort.Strings(plan.Orphaned)

	return plan, nil
}

// ReplayStreams converts the plan into the subscription package's replay input,
// deduplicated by (event, source).
func (p *Plan) ReplayStreams() []subscription.ReplayStream {
	seen := map[string]bool{}
	var out []subscription.ReplayStream
	for _, s := range p.Streams {
		key := s.EventName + "\x00" + s.Resource
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, subscription.ReplayStream{EventName: s.EventName, Resource: s.Resource})
	}
	return out
}

// normalizeSources canonicalises each declared source to "module/entity". A
// bare entity name means "same module" — the common case for a projection that
// joins entities of its own module.
func normalizeSources(module string, sources []spec.SummarySource) []string {
	var normalized []string
	for _, src := range sources {
		ref := strings.TrimSpace(src.Entity)
		if ref == "" {
			continue
		}
		if canon, ok := spec.NormalizeEntityRef(ref); ok {
			normalized = append(normalized, canon)
			continue
		}
		normalized = append(normalized, module+"/"+ref)
	}
	sort.Strings(normalized)
	return normalized
}

// eventBelongsToResource reports whether a fully-qualified event name
// ("cafe-order.order.on_submit") was emitted by the resource
// ("cafe-order/order").
func eventBelongsToResource(eventName, resource string) bool {
	prefix := strings.ReplaceAll(resource, "/", ".") + "."
	return strings.HasPrefix(eventName, prefix)
}
