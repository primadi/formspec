package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// requestIDKey is the context key for the correlation request ID
// (spec §2.3). It lives here — not in internal/api — so the Starlark
// executor can read it without importing the API layer (import cycle).
type requestIDKey struct{}

// WithRequestID stores the request ID in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the request ID, or "" when absent.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// NewRequestID generates a random 16-hex-char request ID. Used at the
// boundary when no upstream X-Request-ID was supplied (spec §2.3).
func NewRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
