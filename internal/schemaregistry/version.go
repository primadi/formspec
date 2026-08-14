package schemaregistry

// Package schemaregistry provides the schema registry client: resolving the
// registry base URL, mapping a manifest apiVersion to a schema version, and
// caching schema files locally so `formspec validate` and `formspec init`
// work offline once a version has been fetched.
//
// Design (docs/plan/schema-registry-online.md):
//   - Single CLI `formspec`; spec version lives in the manifest apiVersion.
//   - `formspec.dev/v1`, `formspec.dev/v2`, ... map to registry versions v1, v2, ...
//   - Schemas are fetched from the registry (default https://schemas.formspec.dev)
//     and cached under os.UserCacheDir()/formspec/schemas/<version>.
//   - No schema is embedded in the binary: new spec versions never require a
//     CLI reinstall.

import (
	"fmt"
	"regexp"
)

// apiVersionRegexp matches formspec.dev/v1, formspec.dev/v2, ...
var apiVersionRegexp = regexp.MustCompile(`^formspec\.dev/(v\d+)$`)

// ParseVersion extracts the schema version segment from a manifest apiVersion.
// "formspec.dev/v1" -> "v1". An error is returned for unsupported groups or
// pre-stable versions (e.g. formspec.dev/v1alpha1) — manifests must declare a
// stable version like formspec.dev/v1.
func ParseVersion(apiVersion string) (string, error) {
	m := apiVersionRegexp.FindStringSubmatch(apiVersion)
	if m == nil {
		return "", fmt.Errorf("unsupported apiVersion %q (want formspec.dev/v1, formspec.dev/v2, ...)", apiVersion)
	}
	return m[1], nil
}
