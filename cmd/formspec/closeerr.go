// Close-error propagation for artifact writers.
//
// A writer chain that produces a file — tar → gzip → os.File, or a plain
// buffered file — only reaches disk when the *last* Close returns. Discarding
// that error means a truncated or corrupt artifact is reported as success: the
// archive exists, its name is right, and nothing can read it back. So for these
// writers the close error is part of the operation, not cleanup.
//
// Deferred in registration order (out, gz, tw) they unwind LIFO — tw flushes
// into gz, gz into out, out onto disk — which is the order the formats require.
package main

import "io"

// closeInto closes c and returns its error, keeping the first error seen. The
// pattern is `defer func() { err = closeInto(err, w) }()` in a function with a
// named error result.
func closeInto(err error, c io.Closer) error {
	if cerr := c.Close(); cerr != nil && err == nil {
		return cerr
	}
	return err
}
