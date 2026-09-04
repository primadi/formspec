// Package oauth provides external authentication providers (auth redesign
// Fase 5 — OAuth multi-provider). It wraps golang.org/x/oauth2 with a small
// Provider abstraction: authorize URL generation, code exchange, and
// normalized userinfo (ID/Email/Name).
//
// Providers are declared via `settings.auth.providers` (kind: Config).
// Well-known presets (google, github, microsoft) fill in endpoint URLs;
// custom providers declare them explicitly. OIDC providers resolve their
// endpoints via discovery ({issuer}/.well-known/openid-configuration).
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// UserInfo is the normalized identity returned by a provider.
type UserInfo struct {
	ID    string
	Email string
	Name  string
	// EmailVerified reports whether the provider verified the email address.
	// OIDC providers (Google, Microsoft) return an explicit `email_verified`
	// claim; providers without the claim (e.g. GitHub, which only exposes
	// verified primary emails) default to true. It is false only when the
	// provider explicitly says the email is unverified.
	EmailVerified bool
}

// Provider is a configured external auth provider.
type Provider interface {
	// Name returns the provider name (e.g. "google").
	Name() string
	// AuthorizeURL returns the provider's authorization URL for the given
	// CSRF state and redirect URL. The redirect URL is per-request so the
	// callback lands on the caller's workspace.
	AuthorizeURL(state, redirectURL string) string
	// Exchange trades an authorization code for a token and fetches the
	// normalized userinfo.
	Exchange(ctx context.Context, code string) (*UserInfo, error)
}

// Config is the resolved provider configuration.
type Config struct {
	Name         string
	Type         string // oidc (default) | oauth2
	ClientID     string
	ClientSecret string
	Scopes       []string
	// OIDC
	Issuer string
	// OAuth2 (custom)
	AuthorizeURL string
	TokenURL     string
	UserInfoURL  string
	// RedirectURL is the callback URL the provider redirects to
	// (/{ws}/_ui/auth/oauth/{name}/callback).
	RedirectURL string
}

// DefaultScopes is used when a provider declares none.
var DefaultScopes = []string{"openid", "email", "profile"}

// preset endpoints for well-known providers.
var presets = map[string]struct {
	Type         string
	Issuer       string
	AuthorizeURL string
	TokenURL     string
	UserInfoURL  string
}{
	"google": {
		Type:   "oidc",
		Issuer: "https://accounts.google.com",
	},
	"microsoft": {
		Type:   "oidc",
		Issuer: "https://login.microsoftonline.com/common/v2.0",
	},
	"github": {
		Type:         "oauth2",
		AuthorizeURL: "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		UserInfoURL:  "https://api.github.com/user",
	},
}

// New builds a Provider from a Config, applying preset defaults and resolving
// OIDC endpoints via discovery.
func New(cfg Config) (Provider, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("oauth: provider name is required")
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("oauth: provider %q requires client_id and client_secret", cfg.Name)
	}

	// Apply preset defaults.
	if p, ok := presets[cfg.Name]; ok {
		if cfg.Type == "" {
			cfg.Type = p.Type
		}
		if cfg.Issuer == "" {
			cfg.Issuer = p.Issuer
		}
		if cfg.AuthorizeURL == "" {
			cfg.AuthorizeURL = p.AuthorizeURL
		}
		if cfg.TokenURL == "" {
			cfg.TokenURL = p.TokenURL
		}
		if cfg.UserInfoURL == "" {
			cfg.UserInfoURL = p.UserInfoURL
		}
	}
	if cfg.Type == "" {
		cfg.Type = "oidc"
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = DefaultScopes
	}

	// OIDC: resolve endpoints via discovery.
	if cfg.Type == "oidc" {
		if cfg.Issuer == "" {
			return nil, fmt.Errorf("oauth: provider %q (oidc) requires issuer", cfg.Name)
		}
		disc, err := discover(cfg.Issuer)
		if err != nil {
			return nil, fmt.Errorf("oauth: provider %q discovery: %w", cfg.Name, err)
		}
		if cfg.AuthorizeURL == "" {
			cfg.AuthorizeURL = disc.AuthorizationEndpoint
		}
		if cfg.TokenURL == "" {
			cfg.TokenURL = disc.TokenEndpoint
		}
		if cfg.UserInfoURL == "" {
			cfg.UserInfoURL = disc.UserinfoEndpoint
		}
	}

	if cfg.AuthorizeURL == "" || cfg.TokenURL == "" {
		return nil, fmt.Errorf("oauth: provider %q requires authorize_url and token_url", cfg.Name)
	}

	oc := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthorizeURL,
			TokenURL: cfg.TokenURL,
		},
		RedirectURL: cfg.RedirectURL,
		Scopes:      cfg.Scopes,
	}

	return &provider{cfg: cfg, oauth: oc}, nil
}

// provider is the concrete Provider implementation.
type provider struct {
	cfg   Config
	oauth *oauth2.Config
}

func (p *provider) Name() string { return p.cfg.Name }

func (p *provider) AuthorizeURL(state, redirectURL string) string {
	// The redirect URL is per-request (workspace-aware); copy the config so
	// the shared instance keeps its default.
	oc := *p.oauth
	if redirectURL != "" {
		oc.RedirectURL = redirectURL
	}
	return oc.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *provider) Exchange(ctx context.Context, code string) (*UserInfo, error) {
	tok, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth: %s exchange: %w", p.cfg.Name, err)
	}
	return p.fetchUserInfo(ctx, tok)
}

// fetchUserInfo calls the provider's userinfo endpoint and normalizes the
// response. OIDC userinfo returns {sub, email, name}; GitHub returns
// {id, email, name}; Google/Microsoft OIDC return {sub, email, name}.
func (p *provider) fetchUserInfo(ctx context.Context, tok *oauth2.Token) (*UserInfo, error) {
	if p.cfg.UserInfoURL == "" {
		return nil, fmt.Errorf("oauth: provider %q has no userinfo endpoint", p.cfg.Name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	// GitHub requires a User-Agent.
	req.Header.Set("User-Agent", "formspec-registry")
	// GitHub's token endpoint returns form-encoded unless Accept is set.
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: %s userinfo: %w", p.cfg.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("oauth: %s userinfo: status %d: %s", p.cfg.Name, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("oauth: %s userinfo decode: %w", p.cfg.Name, err)
	}

	info := &UserInfo{
		ID:    str(raw, "sub", "id"),
		Email: str(raw, "email"),
		Name:  str(raw, "name", "display_name", "login"),
	}
	// email_verified: absent → true (the provider exposed the email as the
	// user's primary/verified address); explicit false → false.
	info.EmailVerified = !boolFalse(raw, "email_verified")
	if info.ID == "" {
		info.ID = info.Email
	}
	if info.ID == "" {
		return nil, fmt.Errorf("oauth: %s userinfo has no id/email", p.cfg.Name)
	}
	return info, nil
}

// str returns the first non-empty string field from the map.
func str(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// boolFalse reports whether the map has an explicit false boolean at key.
// Absent or non-boolean values return false (callers treat that as "not
// explicitly false").
func boolFalse(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return !v
	}
	return false
}

// ─── OIDC Discovery ───

type discoveryDoc struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

var (
	discMu    sync.Mutex
	discCache = map[string]*discoveryDoc{}
)

// discover fetches and caches an OIDC discovery document.
func discover(issuer string) (*discoveryDoc, error) {
	issuer = strings.TrimSuffix(issuer, "/")
	discMu.Lock()
	if d, ok := discCache[issuer]; ok {
		discMu.Unlock()
		return d, nil
	}
	discMu.Unlock()

	url := issuer + "/.well-known/openid-configuration"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery %s: status %d", url, resp.StatusCode)
	}
	var d discoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" {
		return nil, fmt.Errorf("discovery %s: missing endpoints", url)
	}

	discMu.Lock()
	discCache[issuer] = &d
	discMu.Unlock()
	return &d, nil
}
