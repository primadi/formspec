package vendor

import (
	"context"
	"os/exec"
)

// execCommand is a seam for git invocation (tests never hit the network).
var execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
