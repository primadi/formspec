// impl/billing/notify_wa.go
//
// Implementasi native untuk kind: Service "notify-wa".

package billing

import (
	"context"
	"fmt"
)

// NotifyWA adalah wrapper WhatsApp Business API.
// Method: send — impl: { type: native, ref: "NotifyWA.Send" }

type NotifyWA struct{}

// Send mengirim pesan WhatsApp.
// Dipanggil dari job send-wa-notification (via Subscription).
func (n *NotifyWA) Send(ctx context.Context, params NotifyWAParams) error {
	// TODO: implement
	// 1. Panggil WA Business API
	// 2. Log hasil
	return fmt.Errorf("not implemented")
}

type NotifyWAParams struct {
	To      string `json:"to"`      // nomor WA tujuan
	Message string `json:"message"` // isi pesan
}
