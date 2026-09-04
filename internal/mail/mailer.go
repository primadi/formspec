// Package mail provides a minimal SMTP mailer for transactional email
// (password reset). It wraps net/smtp with a small Config: host/port,
// optional auth (user/pass), and a From address. Designed to work with
// Mailpit in dev (SMTP :1025, UI :8025) and any standard SMTP relay in prod.
package mail

import (
	"fmt"
	"net/smtp"
	"strings"
)

// Config configures the SMTP mailer.
type Config struct {
	Host string // SMTP host, e.g. "mailpit" (dev) or "smtp.example.com"
	Port int    // SMTP port, e.g. 1025 (Mailpit) or 587 (TLS submission)
	User string // optional SMTP username (empty = no auth)
	Pass string // optional SMTP password (empty = no auth)
	From string // From address, e.g. "no-reply@formspec.dev"
	// BaseURL is the public origin used to build links inside emails
	// (e.g. "http://localhost:18080"). Empty = relative links.
	BaseURL string
}

// Mailer sends transactional email over SMTP.
type Mailer struct {
	cfg Config
}

// New creates a Mailer from the given config.
func New(cfg Config) *Mailer {
	return &Mailer{cfg: cfg}
}

// Addr returns the host:port dial address.
func (m *Mailer) Addr() string {
	return fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
}

// BaseURL returns the configured public origin for email links.
func (m *Mailer) BaseURL() string {
	return m.cfg.BaseURL
}

// Send delivers a plain-text (and optional HTML) message to one recipient.
// The message is built as a minimal RFC 5322 email with a text/plain body
// and, when html is non-empty, a multipart/alternative text+html body.
func (m *Mailer) Send(to, subject, text, html string) error {
	if m.cfg.Host == "" {
		return fmt.Errorf("mail: smtp host not configured")
	}
	if m.cfg.From == "" {
		return fmt.Errorf("mail: from address not configured")
	}

	msg := buildMessage(m.cfg.From, to, subject, text, html)

	var auth smtp.Auth
	if m.cfg.User != "" {
		auth = smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)
	}
	if err := smtp.SendMail(m.Addr(), auth, m.cfg.From, []string{to}, msg); err != nil {
		return fmt.Errorf("mail: send to %s: %w", to, err)
	}
	return nil
}

// buildMessage assembles the raw RFC 5322 message bytes.
func buildMessage(from, to, subject, text, html string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	if html == "" {
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(text)
	} else {
		boundary := "formspec-boundary-7f4a"
		b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
		b.WriteString("\r\n")
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(text)
		b.WriteString("\r\n--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(html)
		b.WriteString("\r\n--" + boundary + "--\r\n")
	}
	return []byte(b.String())
}
