package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
	db "github.com/primadi/formspec/renderers/jsonb-persist"
)

// CreateParams is one in-app notification to write.
type CreateParams struct {
	WorkspaceID string
	RecipientID string
	Title       string
	Body        string
	Level       string
	// Provenance, so a notification can be traced back to the event that
	// produced it without guessing from the heading.
	SourceEvent    string
	SourceResource string
	SourceID       string
}

// Store writes a notification row. Implemented by an adapter so this package
// does not depend on the entity registry (and so the delivery path can be
// tested without one).
type Store interface {
	CreateNotification(ctx context.Context, p CreateParams) (string, error)
}

// EntityStoreWriter adapts the framework entity store
// (`formspec.core.notification`) to Store.
//
// Writes are marked SystemCaller: a delivery runs in the outbox worker, with no
// user behind it, so permission guards cannot be evaluated — the same explicit
// marking the seed and job tracker use.
type EntityStoreWriter struct {
	Store *db.EntityStore
}

// CreateNotification implements Store.
func (w *EntityStoreWriter) CreateNotification(ctx context.Context, p CreateParams) (string, error) {
	if w == nil || w.Store == nil {
		return "", fmt.Errorf("notification store is not wired")
	}
	data := map[string]any{
		"recipient_id":    p.RecipientID,
		"title":           p.Title,
		"read":            false,
		"source_event":    p.SourceEvent,
		"source_resource": p.SourceResource,
	}
	if p.Body != "" {
		data["body"] = p.Body
	}
	if p.Level != "" {
		data["level"] = p.Level
	}
	if p.SourceID != "" {
		data["source_id"] = p.SourceID
	}
	return w.Store.Insert(ctx, db.InsertParams{
		WorkspaceID:  p.WorkspaceID,
		CreatedBy:    "system:notify",
		Data:         data,
		SystemCaller: true,
	})
}

// Dispatch is called for each `deliver: {channel: notification}` entry.
//
// It ALWAYS writes the in-app row (that is the part with no code behind it, and
// what NotificationCenter lists) and then, when the entry names a `handler:`,
// hands the payload to that Service action for channels outside in-app —
// email, WA, push. A handler failure is returned so the outbox retries the
// whole delivery; the row is written first, so a retry can leave a duplicate
// in-app notification, which the caller controls with `idempotency_key` (the
// same guard the `reliable_event` consequence uses).
type Dispatch func(ctx context.Context, workspaceID, resource, eventName string, payload map[string]any, decl *spec.NotificationDecl, handler string) error

// NewDispatch builds the delivery for the `notification` channel.
//
// store may be nil: in-app delivery is then unwired, and an entry that relies on
// it FAILS rather than reporting success — a delivery that silently does nothing
// is the failure this whole channel was fixed for.
//
// runHandler invokes a Service action named by `handler:`; it may be nil when no
// entry uses one.
func NewDispatch(store Store, runHandler func(ctx context.Context, workspaceID, resource, eventName string, payload map[string]any, ref string) error) Dispatch {
	return func(ctx context.Context, workspaceID, resource, eventName string, payload map[string]any, decl *spec.NotificationDecl, handler string) error {
		hasHandler := strings.TrimSpace(handler) != ""
		if decl == nil && !hasHandler {
			return fmt.Errorf(
				"event %s: `notification` entry declares neither `notification:` (in-app) nor `handler:` — it would deliver nothing",
				eventName)
		}

		if decl != nil {
			recipient, err := resolveRecipient(decl.Recipient, payload)
			if err != nil {
				return fmt.Errorf("event %s: %w", eventName, err)
			}
			title := interpolate(decl.Title, payload)
			if strings.TrimSpace(title) == "" {
				// A notification with no heading is unreadable in the feed; the
				// declaration requires `title`, so reaching here means the
				// template resolved to nothing — say which template instead of
				// writing a blank row.
				return fmt.Errorf("event %s: notification `title` %q resolved to an empty string", eventName, decl.Title)
			}
			if store == nil {
				return fmt.Errorf("event %s: cannot deliver in-app notification — no notification store is wired", eventName)
			}
			if _, err := store.CreateNotification(ctx, CreateParams{
				WorkspaceID:    workspaceID,
				RecipientID:    recipient,
				Title:          title,
				Body:           interpolate(decl.Body, payload),
				Level:          strings.TrimSpace(decl.Level),
				SourceEvent:    eventName,
				SourceResource: resource,
				SourceID:       payloadString(payload, "id"),
			}); err != nil {
				return fmt.Errorf("event %s: write notification: %w", eventName, err)
			}
		}

		if hasHandler {
			if runHandler == nil {
				return fmt.Errorf("event %s: `handler:` %q cannot run — no Service dispatcher is wired", eventName, handler)
			}
			if err := runHandler(ctx, workspaceID, resource, eventName, payload, handler); err != nil {
				return fmt.Errorf("event %s: notification handler %s: %w", eventName, handler, err)
			}
		}
		return nil
	}
}

// resolveRecipient resolves the declared payload path to the notification's
// addressee.
//
// An unresolvable path is an ERROR, not a blank recipient: `recipient_id` is
// what the entity's `row_scope` matches against, so an empty value produces a
// row NOBODY can read — a notification that looks delivered and is invisible.
func resolveRecipient(path string, payload map[string]any) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", fmt.Errorf("notification has no `recipient` — the row would have no addressee and `row_scope` admits only the addressee, so nobody could ever read it")
	}
	v, ok := lookupPath(payload, p)
	if !ok || v == nil {
		return "", fmt.Errorf("notification `recipient: %s` — the event payload has no %q, so there is nobody to address", p, p)
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if s == "" {
		return "", fmt.Errorf("notification `recipient: %s` resolved to an empty value", p)
	}
	return s, nil
}

// interpolate resolves `{dotted.path}` tokens against the payload. An
// unresolved token is left verbatim (visible in the notification rather than
// silently becoming an empty string), mirroring the integrator's `map:` and the
// delivery `idempotency_key` template — one template vocabulary.
//
// It advances a CURSOR past each closing brace rather than re-scanning the same
// string: rewriting the produced text in place and rescanning from the start
// makes an unresolved token a no-op (the same `{` is found forever), which is an
// infinite loop — caught by TestDispatch_EmptyTitleAfterInterpolationFails.
func interpolate(tmpl string, payload map[string]any) string {
	var b strings.Builder
	rest := tmpl
	for {
		start := strings.Index(rest, "{")
		if start < 0 {
			b.WriteString(rest)
			return b.String()
		}
		rel := strings.Index(rest[start:], "}")
		if rel < 0 {
			b.WriteString(rest)
			return b.String()
		}
		end := start + rel
		b.WriteString(rest[:start])
		path := strings.TrimSpace(rest[start+1 : end])
		if v, ok := lookupPath(payload, path); ok && v != nil {
			b.WriteString(stringify(v))
		} else {
			// Keep the token verbatim so a typo is visible, and advance past it.
			b.WriteString(rest[start : end+1])
		}
		rest = rest[end+1:]
	}
}

// stringify renders a scalar for a notification field; a structured value is
// JSON, so nothing is silently lost.
func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case map[string]any, []any:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// lookupPath resolves a dotted path against the payload, walking nested maps.
func lookupPath(payload map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	var cur any = payload
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// payloadString reads a top-level string field, tolerating any scalar.
func payloadString(payload map[string]any, key string) string {
	v, ok := payload[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
