package schemaregistry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CacheDir returns the root of the local schema cache.
func (c *Client) CacheDir() (string, error) {
	if c.CacheRoot != "" {
		return c.CacheRoot, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve user cache dir: %w", err)
	}
	return filepath.Join(base, "formspec", "schemas"), nil
}

// VersionDir returns the cache directory for a schema version.
func (c *Client) VersionDir(version string) (string, error) {
	root, err := c.CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, version), nil
}

// Ensure makes sure the schema files for version are cached. When force is
// true it refetches even if the cache already looks complete.
func (c *Client) Ensure(version string, kinds []string, force bool) error {
	dir, err := c.VersionDir(version)
	if err != nil {
		return err
	}
	if !force && c.complete(dir, kinds) {
		return nil
	}
	return c.fetchVersion(version, kinds)
}

// fetchVersion downloads the root schema and each requested kind schema into
// the cache. Used by `formspec validate`, which only needs the kinds actually
// present in the spec tree.
func (c *Client) fetchVersion(version string, kinds []string) error {
	dir, err := c.VersionDir(version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "kinds"), 0o755); err != nil {
		return fmt.Errorf("mkdir cache: %w", err)
	}
	if err := c.download("formspec.schema.json", c.RootURL(version), dir); err != nil {
		return err
	}
	for _, k := range kinds {
		if err := c.download("kinds/"+k+".schema.json", c.KindURL(version, k), dir); err != nil {
			return err
		}
	}
	return nil
}

// complete reports whether the root schema and every requested kind schema are
// already present in dir.
func (c *Client) complete(dir string, kinds []string) bool {
	if _, err := os.Stat(filepath.Join(dir, "formspec.schema.json")); err != nil {
		return false
	}
	for _, k := range kinds {
		if _, err := os.Stat(filepath.Join(dir, "kinds", k+".schema.json")); err != nil {
			return false
		}
	}
	return true
}

// EnsureFull makes sure the complete schema set for version is cached: the root
// schema, index.json, and every kind schema listed in index.json. Used by
// `formspec init` and `formspec schema fetch/update` where "all kinds" is wanted
// and the kind set is defined by the registry itself (index.json), not by the CLI.
func (c *Client) EnsureFull(version string, force bool) error {
	_ = force // EnsureFull always refreshes; callers invoke it only on demand.
	dir, err := c.VersionDir(version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "kinds"), 0o755); err != nil {
		return fmt.Errorf("mkdir cache: %w", err)
	}
	if err := c.download("formspec.schema.json", c.RootURL(version), dir); err != nil {
		return err
	}
	indexData, err := c.fetch(c.BaseURL + "/" + version + "/index.json")
	if err != nil {
		return fmt.Errorf("fetch %s/index.json: %w", version, err)
	}
	var idx struct {
		Kinds []string `json:"kinds"`
	}
	if err := json.Unmarshal(indexData, &idx); err != nil {
		return fmt.Errorf("decode %s/index.json: %w", version, err)
	}
	if len(idx.Kinds) == 0 {
		return fmt.Errorf("registry %s: index.json has no \"kinds\" list", version)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), indexData, 0o644); err != nil {
		return fmt.Errorf("write index.json: %w", err)
	}
	for _, k := range idx.Kinds {
		if err := c.download("kinds/"+k+".schema.json", c.KindURL(version, k), dir); err != nil {
			return err
		}
	}
	return nil
}

// fetch returns the bytes of url, failing on any non-200 response.
func (c *Client) fetch(url string) ([]byte, error) {
	resp, err := c.HTTP.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// download fetches url and writes it to dir/rel.
func (c *Client) download(rel, url, dir string) error {
	data, err := c.fetch(url)
	if err != nil {
		return err
	}
	dest := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("write cache %s: %w", dest, err)
	}
	return nil
}

// List returns the schema versions currently cached (ascending).
func (c *Client) List() ([]string, error) {
	root, err := c.CacheDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "v") {
			versions = append(versions, e.Name())
		}
	}
	sort.Strings(versions)
	return versions, nil
}

// Clear removes every cached schema version.
func (c *Client) Clear() error {
	root, err := c.CacheDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(root)
}
