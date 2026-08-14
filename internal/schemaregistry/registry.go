package schemaregistry

import (
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is the canonical schema registry.
const DefaultBaseURL = "https://schemas.formspec.dev"

// Client downloads and caches schema versions from a registry.
type Client struct {
	// BaseURL is the registry root, e.g. https://schemas.formspec.dev.
	BaseURL string
	// CacheRoot overrides the cache location (used by tests). Empty means
	// os.UserCacheDir()/formspec/schemas.
	CacheRoot string
	// HTTP is the HTTP client used for downloads.
	HTTP *http.Client
}

// New returns a registry client for baseURL with the default cache location.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// ResolveBaseURL returns FORMSPEC_SCHEMA_REGISTRY if set, else the default.
// The project-level override lives in formspec-app.yaml (schema-registry:) and
// is layered on top by the CLI.
func ResolveBaseURL() string {
	if v := os.Getenv("FORMSPEC_SCHEMA_REGISTRY"); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	return DefaultBaseURL
}

// RootURL returns the registry URL for a version's root schema.
func (c *Client) RootURL(version string) string {
	return c.BaseURL + "/" + version + "/formspec.schema.json"
}

// KindURL returns the registry URL for a version's kind schema.
func (c *Client) KindURL(version, kind string) string {
	return c.BaseURL + "/" + version + "/kinds/" + kind + ".schema.json"
}
