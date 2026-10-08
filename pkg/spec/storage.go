package spec

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// `allowed_types` grammar (gap #4b).
//
// The canonical form is a **bare extension without the dot** — `jpg`, `png`,
// `pdf` — because that is what the field-type docs have always shown. Three
// other spellings are accepted and normalized to the same meaning by the
// matchers on both sides (client: `lib/media.ts`, server: `internal/api/file.go`):
//
//	.jpg        → dotted extension
//	image/jpeg  → exact MIME type
//	image/*     → MIME wildcard
//
// Validation exists because the mismatch was invisible: the documented form
// (`[jpg]`) matched nothing at all, so a correct-looking manifest silently
// rejected every upload. Anything outside these four shapes is now refused at
// `formspec validate` instead of at the upload button.
var (
	extPattern       = regexp.MustCompile(`^[a-z0-9]{1,12}$`)
	dottedExtPattern = regexp.MustCompile(`^\.[a-z0-9]{1,12}$`)
	mimePattern      = regexp.MustCompile(`^[a-z][a-z0-9-]*/[a-z0-9][a-z0-9.+-]*$`)
	mimeWildPattern  = regexp.MustCompile(`^[a-z][a-z0-9-]*/\*$`)
)

// ValidateStorageSpec validates a file field's `storage` block.
func ValidateStorageSpec(fieldName string, s *StorageSpec) error {
	if s == nil {
		return nil
	}
	for i, raw := range s.AllowedTypes {
		t := strings.TrimSpace(raw)
		if t == "" {
			return fmt.Errorf("field %q: storage.allowed_types[%d] is empty — remove the entry or name a type", fieldName, i)
		}
		lower := strings.ToLower(t)
		if lower != t {
			return fmt.Errorf("field %q: storage.allowed_types[%d] (%s) must be lowercase (it is compared case-insensitively)", fieldName, i, t)
		}
		switch {
		case extPattern.MatchString(t):
			// Canonical: a bare extension.
		case dottedExtPattern.MatchString(t):
			// Legacy spelling of the same thing; accepted, not preferred.
		case mimeWildPattern.MatchString(t):
			// MIME wildcard, e.g. image/*.
		case mimePattern.MatchString(t):
			// Exact MIME type, e.g. image/jpeg.
		default:
			return fmt.Errorf(
				"field %q: storage.allowed_types[%d] (%s) is not a known form — use a bare extension (`jpg`), `.jpg`, a MIME type (`image/jpeg`), or a MIME wildcard (`image/*`)",
				fieldName, i, t)
		}
	}
	if s.MaxCount < 0 {
		return fmt.Errorf("field %q: storage.max_count must be positive (1 file, or more for a list of files)", fieldName)
	}
	if s.MaxSizeMB < 0 {
		return fmt.Errorf("field %q: storage.max_size_mb must be positive", fieldName)
	}
	// Durations are validated, not merely parsed later: an unparsable value used
	// to be ignored at runtime, so a typo ("15min" instead of "15m") silently
	// fell back to the default and the manifest appeared to work.
	for _, d := range []struct {
		key, value string
	}{
		{"signed_url_ttl", s.SignedURLTTL},
		{"download_cache_ttl", s.DownloadCacheTTL},
		{"ttl", s.TTL},
	} {
		if strings.TrimSpace(d.value) == "" {
			continue
		}
		parsed, err := time.ParseDuration(d.value)
		if err != nil {
			return fmt.Errorf("field %q: storage.%s (%s) is not a Go duration — use a unit suffix, e.g. `60s`, `15m`, `24h`",
				fieldName, d.key, d.value)
		}
		if parsed < 0 {
			return fmt.Errorf("field %q: storage.%s (%s) must not be negative", fieldName, d.key, d.value)
		}
	}
	return nil
}

// StorageAllowsImage reports whether a file field's declared types admit images.
// Renderers use it to decide between an inline preview and a download link; the
// same question is answered at runtime by the value's extension, so a field
// without `allowed_types` is not assumed to be images-only.
func StorageAllowsImage(s *StorageSpec) bool {
	if s == nil || len(s.AllowedTypes) == 0 {
		return false
	}
	for _, raw := range s.AllowedTypes {
		t := strings.ToLower(strings.TrimSpace(raw))
		if strings.HasPrefix(t, "image/") {
			return true
		}
		switch strings.TrimPrefix(t, ".") {
		case "png", "jpg", "jpeg", "gif", "webp", "svg", "avif":
			return true
		}
	}
	return false
}
