package vendor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RegistryClient talks to a FormSpec Module Registry app (todo 13.3) over
// its REST surface (13.3.4): public read, authenticated publish (API key).
//
// Endpoints used (workspace-scoped, default workspace "default"):
//
//	POST {base}/api/v1/registry/vendors            find-or-create vendor
//	POST {base}/api/v1/registry/modules            find-or-create module
//	POST {base}/api/v1/registry/module-versions    create version (draft)
//	POST {base}/api/v1/registry/module-versions/{id}/tarball   upload tarball
//	GET  {base}/api/v1/registry/module-version?module_id=&semver=   lookup
//	GET  {base}/api/v1/registry/module-versions/{id}/tarball       download
type RegistryClient struct {
	BaseURL   string // e.g. https://registry.formspec.dev
	Workspace string // default "default"
	APIKey    string // Bearer token for publish (empty = read-only)
	HTTP      *http.Client
}

// NewRegistryClient creates a client with sane defaults.
func NewRegistryClient(baseURL, workspace, apiKey string) *RegistryClient {
	if workspace == "" {
		workspace = "default"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &RegistryClient{
		BaseURL:   baseURL,
		Workspace: workspace,
		APIKey:    apiKey,
		HTTP:      &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *RegistryClient) apiURL(path string) string {
	// Workspace-scoped surface (D50): /{workspace}/api/v1/... — the
	// unprefixed /api/v1 is deny-by-default for external services.
	return c.BaseURL + "/" + c.Workspace + "/api/v1" + path
}

// do performs an authenticated JSON request and decodes the envelope
// {data: ...} / {error: {code, message}}.
func (c *RegistryClient) do(ctx context.Context, method, path string, body io.Reader, contentType string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL(path), body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		// Try the contract error envelope, fall back to raw body.
		var errEnv struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &errEnv) == nil && errEnv.Error.Message != "" {
			return nil, fmt.Errorf("%s %s: %d %s: %s", method, path, resp.StatusCode, errEnv.Error.Code, errEnv.Error.Message)
		}
		return nil, fmt.Errorf("%s %s: %d: %s", method, path, resp.StatusCode, truncateStr(string(data), 300))
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(data, &env) == nil && len(env.Data) > 0 {
		return env.Data, nil
	}
	return json.RawMessage(data), nil
}

// FindOrCreateVendor resolves a vendor by name (list filter), creating it
// with the given public key when absent. Created records are submitted —
// relation targets must be past draft (document model, todo 4.x).
func (c *RegistryClient) FindOrCreateVendor(ctx context.Context, name, displayName, publicKey string) (string, error) {
	id, err := c.findByField(ctx, "registry/vendors", "name", name)
	if err == nil {
		return id, nil
	}
	body, _ := json.Marshal(map[string]any{
		"name": name, "display_name": displayName, "public_key": publicKey,
	})
	data, err := c.do(ctx, "POST", "/registry/vendors", bytes.NewReader(body), "application/json")
	if err != nil {
		return "", err
	}
	id, err = extractID(data)
	if err != nil {
		return "", err
	}
	_, _ = c.do(ctx, "POST", fmt.Sprintf("/registry/vendors/%s/submit", id), nil, "")
	return id, nil
}

// FindOrCreateModule resolves a module by name, creating it under vendorID
// when absent (submitted — see FindOrCreateVendor).
func (c *RegistryClient) FindOrCreateModule(ctx context.Context, name, displayName, vendorID string) (string, error) {
	id, err := c.findByField(ctx, "registry/modules", "name", name)
	if err == nil {
		return id, nil
	}
	body, _ := json.Marshal(map[string]any{
		"name": name, "display_name": displayName, "vendor_id": vendorID,
	})
	data, err := c.do(ctx, "POST", "/registry/modules", bytes.NewReader(body), "application/json")
	if err != nil {
		return "", err
	}
	id, err = extractID(data)
	if err != nil {
		return "", err
	}
	_, _ = c.do(ctx, "POST", fmt.Sprintf("/registry/modules/%s/submit", id), nil, "")
	return id, nil
}

// CreateModuleVersion records a version (draft) with checksum + signature.
func (c *RegistryClient) CreateModuleVersion(ctx context.Context, moduleID, semver, checksum, signature, publicKey string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"module_id":         moduleID,
		"semver":            semver,
		"checksum":          checksum,
		"signature":         signature,
		"signer_public_key": publicKey,
		"transaction_date":  time.Now().UTC().Format("2006-01-02"),
	})
	data, err := c.do(ctx, "POST", "/registry/module-versions", bytes.NewReader(body), "application/json")
	if err != nil {
		return "", err
	}
	return extractID(data)
}

// UploadTarball uploads the tarball file to the version record's tarball
// field (POST /{module}/{entity}/{id}/{field}, todo 7.17.1).
func (c *RegistryClient) UploadTarball(ctx context.Context, versionID, tarballPath string) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	// Explicit MIME header — Go's CreateFormFile defaults to
	// application/octet-stream, and allowed_types matches ".gz"/
	// "application/gzip" (a compound ".tar.gz" extension never matches
	// filepath.Ext).
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="file"; filename=%q`, filepath.Base(tarballPath)))
	h.Set("Content-Type", "application/gzip")
	part, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	w.Close()

	path := fmt.Sprintf("/registry/module-version/%s/tarball", versionID)
	_, err = c.do(ctx, "POST", path, &buf, w.FormDataContentType())
	return err
}

// LookupVersion finds a module-version by module name + semver. No
// server-side filter on relation fields — list and filter client-side.
func (c *RegistryClient) LookupVersion(ctx context.Context, moduleName, semver string) (versionID, checksum, signature, publicKey string, err error) {
	// Resolve module id first.
	moduleID, err := c.findByField(ctx, "registry/modules", "name", moduleName)
	if err != nil {
		return "", "", "", "", fmt.Errorf("module %q not found in registry", moduleName)
	}
	data, err := c.do(ctx, "GET", "/registry/module-versions?per_page=100", nil, "")
	if err != nil {
		return "", "", "", "", err
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return "", "", "", "", fmt.Errorf("decode versions: %w", err)
	}
	for _, row := range rows {
		if fmt.Sprint(row["module_id"]) != moduleID {
			continue
		}
		if fmt.Sprint(row["semver"]) == semver {
			id := fmt.Sprint(row["id"])
			return id, fmt.Sprint(row["checksum"]), fmt.Sprint(row["signature"]), fmt.Sprint(row["signer_public_key"]), nil
		}
	}
	return "", "", "", "", fmt.Errorf("version %q of module %q not found in registry", semver, moduleName)
}

// DownloadTarball fetches the tarball of a version to destPath.
func (c *RegistryClient) DownloadTarball(ctx context.Context, versionID, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL(fmt.Sprintf("/registry/module-version/%s/tarball", versionID)), nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("download tarball: %d", resp.StatusCode)
	}
	data := readAll(resp.Body)
	return os.WriteFile(destPath, data, 0644)
}

// PublishModule runs the full publish flow (todo 13.3.7): find-or-create
// vendor + module, create the version with checksum + signature (or reuse
// an existing version with the same semver — idempotent re-publish), upload
// the tarball.
func (c *RegistryClient) PublishModule(ctx context.Context, opts PublishOptions) (*PublishResult, error) {
	vendorID, err := c.FindOrCreateVendor(ctx, opts.VendorName, opts.VendorName, opts.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("vendor: %w", err)
	}
	moduleID, err := c.FindOrCreateModule(ctx, opts.ModuleName, opts.ModuleName, vendorID)
	if err != nil {
		return nil, fmt.Errorf("module: %w", err)
	}
	// Idempotent re-publish: an existing version with the same semver is
	// accepted ONLY when the content checksum matches (versions are
	// immutable — different content requires a version bump).
	var versionID string
	existingID, existingChecksum, lookupErr := c.lookupVersion(ctx, moduleID, opts.Version)
	if lookupErr != nil {
		// Not found — create.
		versionID, err = c.CreateModuleVersion(ctx, moduleID, opts.Version, opts.Checksum, opts.Signature, opts.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("version: %w", err)
		}
	} else {
		if existingChecksum != opts.Checksum {
			return nil, fmt.Errorf(
				"version %s already published with a different checksum (%s) — versions are immutable, bump the version",
				opts.Version, existingChecksum)
		}
		versionID = existingID
	}

	// ── Server-side signature verify (13.3.3) — authoritative di native
	// binary (cmd/formspec-registry). Registry dev (formspec dev) tidak
	// punya native handler → endpoint 404 → skip (client-side verify saat
	// install tetap melindungi konsumen).
	if verified, available, err := c.VerifySignatureServer(ctx, opts.Checksum, opts.Signature, opts.PublicKey); err != nil {
		return nil, fmt.Errorf("server-side verify: %w", err)
	} else if available && !verified {
		return nil, fmt.Errorf(
			"server-side signature verification FAILED — publish refused " +
				"(signature does not match the checksum under the vendor public key)")
	}

	if err := c.UploadTarball(ctx, versionID, opts.TarballPath); err != nil {
		return nil, fmt.Errorf("upload tarball: %w", err)
	}
	return &PublishResult{
		VendorID: vendorID, ModuleID: moduleID, VersionID: versionID,
		URL: fmt.Sprintf("%s/default/_admin", c.BaseURL),
	}, nil
}

// VerifySignatureServer calls the registry's signature-verify service
// (13.3.3 — native handler, only present in cmd/formspec-registry).
// Returns (verified, available, error): available=false when the registry
// does not expose the service (dev mode) — caller skips gracefully.
func (c *RegistryClient) VerifySignatureServer(ctx context.Context, checksum, signature, publicKey string) (verified, available bool, err error) {
	body, _ := json.Marshal(map[string]any{
		"checksum": checksum, "signature": signature, "public_key": publicKey,
	})
	data, err := c.do(ctx, "POST", "/registry/signature-verify/verify", bytes.NewReader(body), "application/json")
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return false, false, nil // dev registry — no native handler
		}
		return false, true, err
	}
	var res struct {
		Valid bool   `json:"valid"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return false, true, fmt.Errorf("decode verify response: %w", err)
	}
	return res.Valid, true, nil
}

// lookupVersion finds a version record by module id + semver ("" when
// absent). No server-side filter on module_id — list and filter client-side.
func (c *RegistryClient) lookupVersion(ctx context.Context, moduleID, semver string) (id, checksum string, err error) {
	data, err := c.do(ctx, "GET", "/registry/module-versions?per_page=100", nil, "")
	if err != nil {
		return "", "", err
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return "", "", fmt.Errorf("decode versions: %w", err)
	}
	for _, row := range rows {
		if fmt.Sprint(row["module_id"]) == moduleID && fmt.Sprint(row["semver"]) == semver {
			return fmt.Sprint(row["id"]), fmt.Sprint(row["checksum"]), nil
		}
	}
	return "", "", fmt.Errorf("not found")
}

// PublishOptions configures PublishModule.
type PublishOptions struct {
	VendorName  string
	ModuleName  string
	Version     string
	Checksum    string
	Signature   string
	PublicKey   string
	TarballPath string
}

// PublishResult reports the created registry records.
type PublishResult struct {
	VendorID  string
	ModuleID  string
	VersionID string
	URL       string
}

// ─── helpers ───

// findByField lists a resource and filters client-side by a field (the
// registry's list endpoint doesn't support arbitrary server-side filters).
func (c *RegistryClient) findByField(ctx context.Context, resource, field, value string) (string, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/%s?per_page=100", resource), nil, "")
	if err != nil {
		return "", err
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return "", fmt.Errorf("decode %s: %w", resource, err)
	}
	for _, row := range rows {
		if fmt.Sprint(row[field]) == value {
			raw, _ := json.Marshal(row)
			return extractID(raw)
		}
	}
	return "", fmt.Errorf("%s with %s=%q not found", resource, field, value)
}
func extractID(data json.RawMessage) (string, error) {
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		return "", err
	}
	if id, ok := row["id"]; ok {
		return fmt.Sprint(id), nil
	}
	return "", fmt.Errorf("response has no id: %s", truncateStr(string(data), 200))
}

func readAll(r io.Reader) []byte {
	b, _ := io.ReadAll(io.LimitReader(r, 256<<20))
	return b
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
