package notify

import (
	"context"
	"strings"
	"testing"

	"github.com/primadi/formspec/pkg/spec"
)

// recordingStore captures the notifications a dispatch writes.
type recordingStore struct {
	created []CreateParams
	err     error
}

func (s *recordingStore) CreateNotification(_ context.Context, p CreateParams) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	s.created = append(s.created, p)
	return "n-1", nil
}

// TestDispatch_WritesAddressedNotification is the point of the channel: an event
// produces an in-app notification for the person named by `recipient`, with the
// title interpolated from the payload. That row is what NotificationCenter
// lists, so this one path closes both symptoms of 7.7.6.
func TestDispatch_WritesAddressedNotification(t *testing.T) {
	store := &recordingStore{}
	d := NewDispatch(store, nil)

	err := d(context.Background(), "ws-1", "billing/order", "billing.order.paid",
		map[string]any{"id": "ord-1", "number": "INV-9", "customer_id": "u-42"},
		&spec.NotificationDecl{
			Recipient: "customer_id",
			Title:     "Pesanan {number} dibayar",
			Body:      "Terima kasih, {id}",
			Level:     "info",
		}, "")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(store.created) != 1 {
		t.Fatalf("wrote %d notifications, want 1", len(store.created))
	}
	got := store.created[0]
	if got.RecipientID != "u-42" {
		t.Errorf("recipient = %q, want u-42 (the payload value, not the path)", got.RecipientID)
	}
	if got.Title != "Pesanan INV-9 dibayar" {
		t.Errorf("title = %q — templates must interpolate the payload", got.Title)
	}
	if got.Body != "Terima kasih, ord-1" {
		t.Errorf("body = %q", got.Body)
	}
	// Provenance: the notification can be traced to what produced it.
	if got.SourceEvent != "billing.order.paid" || got.SourceResource != "billing/order" || got.SourceID != "ord-1" {
		t.Errorf("provenance = %+v", got)
	}
}

// TestDispatch_UnresolvedRecipientFails: `recipient_id` is what the entity's
// `row_scope` matches, so an empty value produces a row NOBODY can read — a
// notification that looks delivered and is invisible. Failing means the outbox
// dead-letters it, which is visible.
func TestDispatch_UnresolvedRecipientFails(t *testing.T) {
	store := &recordingStore{}
	d := NewDispatch(store, nil)

	err := d(context.Background(), "ws-1", "billing/order", "billing.order.paid",
		map[string]any{"id": "ord-1"}, // no customer_id
		&spec.NotificationDecl{Recipient: "customer_id", Title: "Hi"}, "")
	if err == nil {
		t.Fatal("an unresolvable recipient must fail")
	}
	if !strings.Contains(err.Error(), "customer_id") {
		t.Errorf("error %q should name the missing path", err)
	}
	if len(store.created) != 0 {
		t.Fatalf("nothing should be written, got %+v", store.created)
	}
}

// TestDispatch_MissingRecipientDeclarationFails is the declared-shape half: no
// `recipient` at all cannot be addressed either.
func TestDispatch_MissingRecipientDeclarationFails(t *testing.T) {
	d := NewDispatch(&recordingStore{}, nil)
	err := d(context.Background(), "ws-1", "b/x", "x.y",
		map[string]any{"id": "1"}, &spec.NotificationDecl{Title: "Hi"}, "")
	if err == nil || !strings.Contains(err.Error(), "no `recipient`") {
		t.Fatalf("expected the missing-recipient refusal, got %v", err)
	}
}

// TestDispatch_EmptyTitleAfterInterpolationFails: a notification with no heading
// is unreadable in the feed, so a title template that RESOLVES TO an empty value
// must say so rather than write a blank row.
//
// Note this is distinct from an UNRESOLVED token, which is left verbatim
// (`{missing}`) and therefore non-empty — see
// TestDispatch_UnresolvedTitleTokenStaysVisible.
func TestDispatch_EmptyTitleAfterInterpolationFails(t *testing.T) {
	d := NewDispatch(&recordingStore{}, nil)
	err := d(context.Background(), "ws-1", "b/x", "x.y",
		map[string]any{"customer_id": "u-1", "number": ""}, // resolves, to empty
		&spec.NotificationDecl{Recipient: "customer_id", Title: "{number}"}, "")
	if err == nil || !strings.Contains(err.Error(), "title") {
		t.Fatalf("expected the empty-title refusal, got %v", err)
	}
}

// TestDispatch_HandlerRunsAlongsideInApp pins the two-source design: the in-app
// row is always written, and an optional Service handler covers channels outside
// in-app (email, WA, push) without either replacing the other.
func TestDispatch_HandlerRunsAlongsideInApp(t *testing.T) {
	store := &recordingStore{}
	var calledWith string
	d := NewDispatch(store, func(_ context.Context, _, _, _ string, _ map[string]any, ref string) error {
		calledWith = ref
		return nil
	})

	err := d(context.Background(), "ws-1", "billing/order", "billing.order.paid",
		map[string]any{"id": "o", "customer_id": "u-1"},
		&spec.NotificationDecl{Recipient: "customer_id", Title: "Hi"},
		"notify-jobs.send-email")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(store.created) != 1 {
		t.Errorf("in-app row must still be written, got %d", len(store.created))
	}
	if calledWith != "notify-jobs.send-email" {
		t.Errorf("handler = %q, want notify-jobs.send-email", calledWith)
	}
}

// TestDispatch_HandlerOnlyIsAllowed covers the entry that only wants an outside
// channel: no in-app block, but a handler — that still delivers something.
func TestDispatch_HandlerOnlyIsAllowed(t *testing.T) {
	called := false
	d := NewDispatch(nil, func(_ context.Context, _, _, _ string, _ map[string]any, _ string) error {
		called = true
		return nil
	})
	if err := d(context.Background(), "ws-1", "b/x", "x.y", map[string]any{}, nil, "svc.act"); err != nil {
		t.Fatalf("handler-only entry must be allowed: %v", err)
	}
	if !called {
		t.Fatal("the handler must have run")
	}
}

// TestDispatch_NothingDeclaredFails is the "looks configured, delivers nothing"
// shape this whole feature exists to remove.
func TestDispatch_NothingDeclaredFails(t *testing.T) {
	d := NewDispatch(&recordingStore{}, nil)
	err := d(context.Background(), "ws-1", "b/x", "x.y", map[string]any{}, nil, "")
	if err == nil {
		t.Fatal("an entry with neither `notification:` nor `handler:` must fail")
	}
	if !strings.Contains(err.Error(), "would deliver nothing") {
		t.Errorf("error %q should say so plainly", err)
	}
}

// TestDispatch_UnwiredStoreFails: in-app delivery with no store is a wiring gap,
// and a wiring gap must not be reported as a delivered notification.
func TestDispatch_UnwiredStoreFails(t *testing.T) {
	d := NewDispatch(nil, nil)
	err := d(context.Background(), "ws-1", "b/x", "x.y",
		map[string]any{"customer_id": "u-1"},
		&spec.NotificationDecl{Recipient: "customer_id", Title: "Hi"}, "")
	if err == nil || !strings.Contains(err.Error(), "no notification store") {
		t.Fatalf("expected the unwired-store refusal, got %v", err)
	}
}

// TestDispatch_UnresolvedTitleTokenStaysVisible: a template token that does not
// resolve is left verbatim, so the receiver sees `{missing}` rather than a
// silently truncated heading.
func TestDispatch_UnresolvedTitleTokenStaysVisible(t *testing.T) {
	store := &recordingStore{}
	d := NewDispatch(store, nil)
	if err := d(context.Background(), "ws-1", "b/x", "x.y",
		map[string]any{"customer_id": "u-1"},
		&spec.NotificationDecl{Recipient: "customer_id", Title: "Order {missing} paid"}, ""); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if store.created[0].Title != "Order {missing} paid" {
		t.Errorf("title = %q — an unresolved token must stay visible", store.created[0].Title)
	}
}

// TestResolveRecipient_NestedPathAndScalars covers the resolution itself,
// including a nested path and a non-string scalar (an id may be numeric).
func TestResolveRecipient_NestedPathAndScalars(t *testing.T) {
	got, err := resolveRecipient("customer.id", map[string]any{
		"customer": map[string]any{"id": "u-7"},
	})
	if err != nil || got != "u-7" {
		t.Fatalf("nested path: got %q, %v", got, err)
	}
	got, err = resolveRecipient("customer_id", map[string]any{"customer_id": 42})
	if err != nil || got != "42" {
		t.Fatalf("numeric id: got %q, %v", got, err)
	}
}
