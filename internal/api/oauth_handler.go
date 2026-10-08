package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/primadi/formspec/internal/auth"
)

// ─── OAuth state store ───
//
// The authorize step generates a random CSRF state, stores it briefly, and
// the callback validates it before exchanging the code. In-memory with TTL —
// sufficient for a single-server deployment (the state is short-lived and
// single-use).

type oauthState struct {
	Workspace string
	Provider  string
	// App is the App the login was started from (plan app-scoped-login.md D1):
	// the callback issues an App-scoped session and lands the user on that
	// App's surface.
	App string
	// Mode distinguishes the two authorize flows: "" = login (the callback
	// runs OAuthLogin and redirects with a token pair), "link" = explicit
	// account linking (the callback passes the code through to the SPA link
	// callback, which POSTs it to the authenticated link endpoint).
	Mode string
	// Assignment is the session context the caller chose BEFORE the provider
	// round-trip, carried through `state` because the round-trip has no step
	// where a choice could be collected (backend §8.7). Empty means "no choice
	// yet": the automatic rule applies, and a principal with several contexts
	// gets redirected back to the picker.
	Assignment string
	Expires    time.Time
}

var (
	oauthStateMu    sync.Mutex
	oauthStates     = map[string]oauthState{}
	oauthStateTTL   = 10 * time.Minute
	oauthStateClean = time.Now()
)

func newOAuthState(workspace, provider, mode, app, assignment string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := hex.EncodeToString(b)
	oauthStateMu.Lock()
	oauthStates[state] = oauthState{
		Workspace:  workspace,
		Provider:   provider,
		App:        app,
		Mode:       mode,
		Assignment: assignment,
		Expires:    time.Now().Add(oauthStateTTL),
	}
	// Opportunistic cleanup.
	if time.Since(oauthStateClean) > time.Minute {
		oauthStateClean = time.Now()
		for k, v := range oauthStates {
			if time.Now().After(v.Expires) {
				delete(oauthStates, k)
			}
		}
	}
	oauthStateMu.Unlock()
	return state
}

func consumeOAuthState(state string) (oauthState, bool) {
	oauthStateMu.Lock()
	defer oauthStateMu.Unlock()
	v, ok := oauthStates[state]
	if !ok {
		return oauthState{}, false
	}
	delete(oauthStates, state) // single-use
	if time.Now().After(v.Expires) {
		return oauthState{}, false
	}
	return v, true
}

// ─── Handlers ───

// HandleOAuthAuthorize serves GET /{ws}/_ui/auth/oauth/{provider}/authorize —
// starts the external auth flow: generates a CSRF state, stores it, and
// redirects to the provider's authorization URL. Public endpoint.
func (b *RouterBuilder) HandleOAuthAuthorize() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		providerName := chi.URLParam(r, "provider")
		prov := authService.OAuthProvider(providerName)
		if prov == nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"oauth provider not configured")
			return
		}
		workspaceID := workspaceFromContext(r.Context())
		// ?mode=link starts the explicit account-linking flow (todo 5.2.21):
		// the callback redirects to the SPA link callback instead of running
		// OAuthLogin. Any other value (or none) is the normal login flow.
		mode := r.URL.Query().Get("mode")
		// ?app= names the App being signed into. Login is per-App (plan
		// app-scoped-login.md D1), so it is required and validated here — a
		// public or unknown App never starts a flow that cannot finish.
		app := r.URL.Query().Get("app")
		if app == "" {
			writeError(w, http.StatusBadRequest, "APP_REQUIRED",
				"app is required — login is scoped to one App")
			return
		}
		if code, msg := b.loginAppCheck(workspaceID, app); code != "" {
			writeError(w, http.StatusBadRequest, code, msg)
			return
		}
		state := newOAuthState(workspaceID, providerName, mode, app, r.URL.Query().Get("assignment"))
		redirectURL := "/" + workspaceID + "/_ui/auth/oauth/" + providerName + "/callback"
		http.Redirect(w, r, prov.AuthorizeURL(state, redirectURL), http.StatusFound)
	}
}

// HandleOAuthCallback serves GET /{ws}/_ui/auth/oauth/{provider}/callback —
// the provider redirects here after the user authenticates. Validates the
// CSRF state, exchanges the code for a token pair, and redirects to the SPA
// callback route with the tokens in the URL fragment (never sent to the
// server). Public endpoint.
func (b *RouterBuilder) HandleOAuthCallback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		providerName := chi.URLParam(r, "provider")
		state := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")
		if state == "" || code == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"state and code are required")
			return
		}

		st, ok := consumeOAuthState(state)
		if !ok || st.Provider != providerName {
			writeError(w, http.StatusBadRequest, "INVALID_STATE",
				"invalid or expired oauth state")
			return
		}

		// Link mode (todo 5.2.21): the user is already signed in and explicitly
		// linking this provider to their account. Pass the code through to the
		// SPA link callback, which POSTs it to the authenticated link endpoint
		// — do NOT run OAuthLogin (that would fail with ErrAccountLinkRequired
		// for a verified password account).
		if st.Mode == "link" {
			http.Redirect(w, r,
				"/"+st.Workspace+"/_admin/oauth/link-callback#code="+code+
					"&provider="+providerName,
				http.StatusFound)
			return
		}

		pair, err := authService.OAuthLoginAs(r.Context(), st.Workspace, st.App, providerName, code, st.Assignment)
		if err != nil {
			// A principal who holds several contexts must CHOOSE one, and a
			// provider round-trip has no step where that could happen. Sending
			// them to the App login with the choices in the fragment lets the
			// SPA show the picker and restart the flow carrying the choice —
			// rather than reporting a generic failure the caller cannot act on
			// (and rather than picking a boundary on their behalf, which is the
			// rule §8.7 exists to prevent).
			var needContext *auth.ContextRequiredError
			if errors.As(err, &needContext) {
				http.Redirect(w, r, b.oauthContextPath(st.Workspace, st.App, providerName, needContext.Choices), http.StatusFound)
				return
			}
			// Redirect to the App's own login with an error fragment so the SPA
			// can show it. Distinct fragments let the SPA explain the account
			// pre-hijacking cases (unverified email / explicit link required).
			frag := "oauth=error"
			switch {
			case errors.Is(err, auth.ErrEmailUnverified):
				frag = "oauth=email_unverified"
			case errors.Is(err, auth.ErrAccountLinkRequired):
				frag = "oauth=link_required"
			case errors.Is(err, auth.ErrNoAppAccess):
				frag = "oauth=no_app_access"
			}
			http.Redirect(w, r, b.appLoginPath(st.Workspace, st.App)+"#"+frag, http.StatusFound)
			return
		}

		// Deliver tokens via the URL fragment (not sent to the server). The app
		// tells the SPA which App-scoped session to boot and which surface to
		// land on; the callback route itself stays a framework route (D5).
		http.Redirect(w, r,
			"/"+st.Workspace+"/_admin/oauth/callback#token="+pair.AccessToken+
				"&refresh_token="+pair.RefreshToken+
				"&app="+url.QueryEscape(st.App),
			http.StatusFound)
	}
}

// oauthContextPath builds the login redirect that asks the caller to CHOOSE a
// session context (backend §8.7), carrying the choices so the SPA can render the
// picker without another round-trip.
//
// The choices ride in the URL FRAGMENT: it is never sent to the server and never
// lands in access logs, and the payload is not a secret — it is the caller's own
// list of contexts, the same values a 409 body carries over the API. Repeated
// `c` parameters rather than one delimited string, because a context VALUE is
// opaque (a branch code, an outlet id) and could contain whatever delimiter we
// picked.
//
// The provider is included so the picker can RESUME the flow: the choice is only
// useful if it can be handed back to the provider round-trip, which carries it in
// `state`.
func (b *RouterBuilder) oauthContextPath(workspace, app, provider string, choices []auth.ContextChoice) string {
	q := url.Values{}
	q.Set("oauth", "context_required")
	q.Set("provider", provider)
	// The App travels with the choices because resuming the flow means calling
	// the authorize endpoint again, and that endpoint requires `app`. Sending it
	// here spares the client from having to derive the App's login path (which
	// depends on the App's `root_url`, a server-side resolution).
	q.Set("app", app)
	for _, c := range choices {
		q.Add("c", c.ID)
	}
	return b.appLoginPath(workspace, app) + "#" + q.Encode()
}

// appLoginPath builds the in-App login path for a resolved App: the App's own
// root_url + "/login" (plan app-scoped-login.md D1). Falls back to the
// workspace + "/app/login" when the App is unknown — the surface will 404 the
// App anyway, but the redirect must never point at the retired `_admin` login.
func (b *RouterBuilder) appLoginPath(workspace, app string) string {
	root := "/app"
	if a, ok := b.apps[app]; ok && a.Spec != nil && a.Spec.RootURL != "" {
		root = strings.TrimSuffix(a.Spec.RootURL, "/")
	}
	return "/" + workspace + root + "/login"
}

// oauthLinkRequest is the POST /auth/oauth/{provider}/link body.
type oauthLinkRequest struct {
	Code string `json:"code"`
}

// HandleOAuthLink serves POST /{ws}/_ui/auth/oauth/{provider}/link — explicit
// account linking (account pre-hijacking protection). The signed-in user
// proves ownership of the existing account (password login), then links an
// external identity to it. The provider email must match the account's
// verified email.
//
// Authenticated: the caller's user ID comes from the session identity (401
// when anonymous).
func (b *RouterBuilder) HandleOAuthLink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		id := IdentityFromContext(r.Context())
		if id == nil || id.UserID == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"authentication required")
			return
		}
		providerName := chi.URLParam(r, "provider")
		if !isOAuthProviderName(providerName) {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"oauth provider not configured")
			return
		}

		var req oauthLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"invalid request body: "+err.Error())
			return
		}
		if req.Code == "" {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
				"code is required")
			return
		}

		workspaceID := workspaceFromContext(r.Context())
		err := authService.LinkOAuthIdentity(r.Context(), workspaceID, id.UserID, providerName, req.Code)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrEmailMismatch):
				writeError(w, http.StatusConflict, "EMAIL_MISMATCH",
					"the provider email does not match your account email")
			case errors.Is(err, auth.ErrEmailUnverified):
				writeError(w, http.StatusConflict, "EMAIL_UNVERIFIED",
					"verify your email before linking an external sign-in method")
			case errors.Is(err, auth.ErrIdentityTaken):
				writeError(w, http.StatusConflict, "IDENTITY_TAKEN",
					"this external identity is already linked to another account")
			case errors.Is(err, auth.ErrInvalidCredentials):
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"oauth provider not configured")
			default:
				writeError(w, http.StatusInternalServerError, "INTERNAL",
					"failed to link account")
			}
			return
		}

		authAuditLog.record(AuthAuditEntry{
			Timestamp: time.Now().UTC(), Method: "oauth-link",
			Username: id.Username, IP: clientIP(r), Result: "success",
		})
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: map[string]any{"linked": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// HandleOAuthUnlink serves POST /{ws}/_ui/auth/oauth/{provider}/unlink —
// explicit account unlinking. The signed-in user removes the external
// identity from their account. The provider must be the one currently linked,
// and the account must keep a usable sign-in method (a pure-OAuth account
// must set a password first).
//
// Authenticated: the caller's user ID comes from the session identity (401
// when anonymous).
func (b *RouterBuilder) HandleOAuthUnlink() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authService == nil {
			writeError(w, http.StatusServiceUnavailable, "AUTH_NOT_CONFIGURED",
				"auth service is not configured")
			return
		}
		id := IdentityFromContext(r.Context())
		if id == nil || id.UserID == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED",
				"authentication required")
			return
		}
		providerName := chi.URLParam(r, "provider")
		if !isOAuthProviderName(providerName) {
			writeError(w, http.StatusNotFound, "NOT_FOUND",
				"oauth provider not configured")
			return
		}

		workspaceID := workspaceFromContext(r.Context())
		err := authService.UnlinkOAuthIdentity(r.Context(), workspaceID, id.UserID, providerName)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrNotLinked):
				writeError(w, http.StatusConflict, "NOT_LINKED",
					"this provider is not linked to your account")
			case errors.Is(err, auth.ErrUnlinkRequiresPassword):
				writeError(w, http.StatusConflict, "UNLINK_REQUIRES_PASSWORD",
					"set a password before unlinking your only sign-in method")
			case errors.Is(err, auth.ErrInvalidCredentials):
				writeError(w, http.StatusNotFound, "NOT_FOUND",
					"oauth provider not configured")
			default:
				writeError(w, http.StatusInternalServerError, "INTERNAL",
					"failed to unlink account")
			}
			return
		}

		authAuditLog.record(AuthAuditEntry{
			Timestamp: time.Now().UTC(), Method: "oauth-unlink",
			Username: id.Username, IP: clientIP(r), Result: "success",
		})
		writeJSON(w, http.StatusOK, SingleResponse{
			Data: map[string]any{"unlinked": true},
			Meta: MetaSingle{
				RequestID: requestIDFromContext(r.Context()),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

// oauthProviderNames returns the configured provider names (for the login
// screen). Empty when none are configured.
func oauthProviderNames() []string {
	if authService == nil {
		return nil
	}
	return authService.OAuthProviders()
}

// isOAuthProviderName reports whether the string is a configured provider.
func isOAuthProviderName(name string) bool {
	for _, n := range oauthProviderNames() {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

var _ = auth.ErrInvalidCredentials // keep auth import when unused in some builds
