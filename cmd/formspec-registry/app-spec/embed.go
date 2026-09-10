// Package registry embeds the FormSpec Registry app spec so the native
// binary (cmd/formspec-registry) can run without a checked-out source tree
// (todo 13.5.6 / Plan C — "native app binary" deployment mode).
package registry

import (
	"embed"
	"io/fs"
)

//go:embed spec
var specFS embed.FS

// SpecFS returns the embedded spec tree rooted at "spec/".
func SpecFS() fs.FS { return specFS }
