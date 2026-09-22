// Summary-projection replay (todo 3.6.4, docs/spec/backend/02-core-extended.md §6).
//
// A summary Entity is populated exclusively by durable events, which for a
// Tier 2 subscription live in the durable stream. `formspec summary rebuild`
// therefore reconstructs a projection by replaying those streams from the
// beginning — through the exact same handler path live delivery uses
// (filter → transform → dispatchOne), so a rebuilt projection is identical to
// one produced by normal operation.
//
// Replay is read-only with respect to the live consumer groups: every run uses
// its own group name, so the StreamingWorker's cursor and pending entries are
// untouched and live delivery keeps working while a rebuild runs.
package subscription

import (
	"context"
	"log"

	"github.com/primadi/formspec/internal/starlark"
	"github.com/primadi/formspec/internal/stream"
)

// ReplayStream is one stream to replay — an event name plus the source
// resource it belongs to ("module/entity"). Resolved by internal/summary from
// the summary Entity's declared `sources`.
type ReplayStream struct {
	EventName string
	Resource  string
}

// ReplayOptions configures one rebuild run.
type ReplayOptions struct {
	// WorkspaceID limits replay to a single workspace's entries. Empty
	// replays every workspace present in the stream.
	WorkspaceID string
	// BatchSize is how many entries are claimed per read (default 100).
	BatchSize int
	// RunID makes the replay consumer group unique so `position: earliest`
	// takes effect and a later rebuild never resumes from an earlier run's
	// cursor. Callers pass a timestamp.
	RunID string
	// Subscribers narrows the replay to these subscriptions, named
	// "module/name". Empty replays every durable subscription that listens to
	// the requested events — which is correct for a projection rebuild, since
	// durable handlers are idempotent by contract (at-least-once delivery),
	// but the caller may scope it further.
	Subscribers []string
}

// StreamResult reports the replay of one stream for one subscription.
type StreamResult struct {
	EventName    string
	Subscription string
	Read         int
	Dispatched   int
	Skipped      int
	Failed       int
	Err          string
}

// ReplayResult aggregates every stream replayed by one run.
type ReplayResult struct {
	Streams    []StreamResult
	Read       int
	Dispatched int
	Skipped    int
	Failed     int
}

// Incomplete reports whether any stream failed or errored — a rebuild that did
// not fully complete and should be re-run after the cause is fixed.
func (r ReplayResult) Incomplete() bool {
	if r.Failed > 0 {
		return true
	}
	for _, s := range r.Streams {
		if s.Err != "" {
			return true
		}
	}
	return false
}

// replayGroupBase is the prefix of the disposable consumer group a rebuild
// creates. The live worker's group is "{module}/{name}" — never this.
const replayGroupBase = "formspec-summary-rebuild"

// ReplaySummaryProjection replays the given streams for every durable
// subscription that listens to them.
//
// A failing handler does not abort the run: the entry is acked in *this* run's
// group (so the loop terminates), counted as failed, and reported. Retry stays
// the live worker's job — the live group's pending entry is unaffected by a
// replay ack. Re-run the rebuild once the handler is fixed.
func (w *StreamingWorker) ReplaySummaryProjection(ctx context.Context, streams []ReplayStream, opts ReplayOptions) ReplayResult {
	res := ReplayResult{}
	if w == nil || w.stream == nil || w.reg == nil {
		return res
	}

	batch := opts.BatchSize
	if batch < 1 || batch > 500 {
		batch = 100
	}
	runID := opts.RunID
	if runID == "" {
		runID = "run"
	}

	durable := w.reg.Durable()
	for _, rs := range streams {
		streamName := stream.NormalizeStreamName(rs.EventName)
		for _, sub := range durable {
			if !subscriptionListensTo(sub.Spec.Events, rs.EventName) {
				continue
			}
			subKey := sub.Module + "/" + sub.Name
			if len(opts.Subscribers) > 0 && !containsString(opts.Subscribers, subKey) {
				continue
			}
			// One group per (stream, subscription, run) — mirroring the live
			// worker's group granularity, so each subscription sees every
			// entry exactly as it would have when the event was first emitted.
			group := replayGroupBase + ":" + runID + ":" + subKey
			sr := w.replayStreamForSub(ctx, sub, rs, streamName, group, batch, opts)
			res.Streams = append(res.Streams, sr)
			res.Read += sr.Read
			res.Dispatched += sr.Dispatched
			res.Skipped += sr.Skipped
			res.Failed += sr.Failed
		}
	}
	return res
}

// replayStreamForSub drains one stream for one durable subscription.
func (w *StreamingWorker) replayStreamForSub(ctx context.Context, sub DurableSub, rs ReplayStream, streamName, group string, batch int, opts ReplayOptions) StreamResult {
	sr := StreamResult{EventName: rs.EventName, Subscription: sub.Module + "/" + sub.Name}

	for {
		entries, err := w.stream.Read(ctx, streamName, group, "formspec-rebuild", "earliest", batch)
		if err != nil {
			sr.Err = err.Error()
			return sr
		}
		if len(entries) == 0 {
			return sr
		}

		for _, e := range entries {
			sr.Read++
			if !w.replayEntry(ctx, sub, rs, streamName, group, e, &sr, opts) {
				continue
			}
		}

		// A short read means the stream is drained for this group.
		if len(entries) < batch {
			return sr
		}
	}
}

// replayEntry replays one stream entry for one subscription, returning false
// when the entry was skipped without dispatch.
func (w *StreamingWorker) replayEntry(ctx context.Context, sub DurableSub, rs ReplayStream, streamName, group string, e stream.Entry, sr *StreamResult, opts ReplayOptions) bool {
	workspaceID, _ := e.Data["workspace_id"].(string)
	if opts.WorkspaceID != "" && workspaceID != opts.WorkspaceID {
		_ = w.stream.Ack(ctx, streamName, group, e.ID)
		sr.Skipped++
		return false
	}

	resource, _ := e.Data["resource"].(string)
	if resource == "" {
		resource = rs.Resource
	}
	occurredAt, _ := e.Data["occurred_at"].(string)
	payload, _ := e.Data["payload"].(map[string]any)
	if payload == nil {
		payload = map[string]any{}
	}

	env := eventEnv(rs.EventName, resource, workspaceID, occurredAt, payload)

	// filter — same contract as live delivery: a non-matching entry is
	// skipped, not an error.
	if sub.Spec.Filter != "" {
		ok, _, err := starlark.EvaluateGuard(sub.Spec.Filter, env)
		if err != nil {
			log.Printf("[summary-rebuild] %s/%s filter error on %s: %v (skipping)", sub.Module, sub.Name, e.ID, err)
			_ = w.stream.Ack(ctx, streamName, group, e.ID)
			sr.Skipped++
			return false
		}
		if !ok {
			_ = w.stream.Ack(ctx, streamName, group, e.ID)
			sr.Skipped++
			return false
		}
	}

	// transform — replaces the payload with the expression's result.
	if sub.Spec.Transform != "" {
		result, err := starlark.EvalExpr(sub.Spec.Transform, env)
		if err != nil {
			log.Printf("[summary-rebuild] %s/%s transform error on %s: %v (skipping)", sub.Module, sub.Name, e.ID, err)
			_ = w.stream.Ack(ctx, streamName, group, e.ID)
			sr.Skipped++
			return false
		}
		if m, ok := result.(map[string]any); ok {
			payload = m
		}
	}

	if err := w.dispatcher.dispatchOne(ctx, workspaceID, rs.EventName, resource, payload, OwnedSub(sub)); err != nil {
		log.Printf("[summary-rebuild] %s/%s entry %s failed: %v", sub.Module, sub.Name, e.ID, err)
		_ = w.stream.Ack(ctx, streamName, group, e.ID)
		sr.Failed++
		return false
	}

	_ = w.stream.Ack(ctx, streamName, group, e.ID)
	sr.Dispatched++
	return true
}

// subscriptionListensTo reports whether a subscription's declared events
// include eventName.
func subscriptionListensTo(events []string, eventName string) bool {
	for _, ev := range events {
		if ev == eventName {
			return true
		}
	}
	return false
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
