package action

import "context"

// systemCallerKey marks a context whose execution has no user behind it.
type systemCallerKey struct{}

// WithSystemCaller marks ctx as a system execution (no user behind it).
//
// It exists because the script write handlers (`resource.save`,
// `resource.create`) receive only a context, not the ExecuteParams — so the
// explicit `ExecuteParams.SystemCaller` decision has to travel with the
// context that reaches them.
//
// NEVER set this from a request path. An anonymous HTTP caller has no identity
// either, so treating "no identity" (or a context that merely lacks one) as
// system would promote anonymous requests to system privileges.
func WithSystemCaller(ctx context.Context) context.Context {
	return context.WithValue(ctx, systemCallerKey{}, true)
}

// IsSystemCaller reports whether ctx was explicitly marked as a system
// execution. Absent marker = NOT system.
func IsSystemCaller(ctx context.Context) bool {
	v, _ := ctx.Value(systemCallerKey{}).(bool)
	return v
}
