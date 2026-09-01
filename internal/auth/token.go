package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Token types embedded in the JWT "typ" claim to distinguish access from
// refresh tokens. A refresh token must never be accepted as an access token
// and vice-versa.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Default token lifetimes.
const (
	DefaultAccessTTL  = 15 * time.Minute
	DefaultRefreshTTL = 7 * 24 * time.Hour
)

// ErrInvalidRefreshToken is returned when a refresh token is malformed,
// expired, or not of type "refresh".
var ErrInvalidRefreshToken = errors.New("auth: invalid refresh token")

// TokenIssuer signs access and refresh JWTs (HS256) and parses refresh tokens.
//
// Claims (todo 6.1.2):
//   - sub:  user ID
//   - ws:   workspace ID
//   - roles: role list
//   - perms: permission list
//   - typ:  "access" | "refresh"
//   - jti:  unique token id (refresh only — used for rotation/session)
//   - iat, exp, iss, aud: standard claims
type TokenIssuer struct {
	secret     []byte
	issuer     string
	audience   string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewTokenIssuer creates a TokenIssuer with a shared HMAC secret.
func NewTokenIssuer(secret, issuer, audience string, accessTTL, refreshTTL time.Duration) *TokenIssuer {
	if accessTTL <= 0 {
		accessTTL = DefaultAccessTTL
	}
	if refreshTTL <= 0 {
		refreshTTL = DefaultRefreshTTL
	}
	return &TokenIssuer{
		secret:     []byte(secret),
		issuer:     issuer,
		audience:   audience,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// accessClaims is the JWT claim set for an access token.
type accessClaims struct {
	Workspace string   `json:"ws"`
	App       string   `json:"app,omitempty"`
	Username  string   `json:"username,omitempty"` // display identity (UserMenu/avatar)
	Roles     []string `json:"roles,omitempty"`
	Perms     []string `json:"perms,omitempty"`
	Type      string   `json:"typ"`
	jwt.RegisteredClaims
}

// refreshClaims is the JWT claim set for a refresh token.
type refreshClaims struct {
	Workspace string `json:"ws"`
	App       string `json:"app,omitempty"`
	Type      string `json:"typ"`
	jwt.RegisteredClaims
}

// IssueAccessToken signs a short-lived access token for the user.
func (t *TokenIssuer) IssueAccessToken(u *User) (string, error) {
	now := time.Now()
	claims := accessClaims{
		Workspace: u.WorkspaceID,
		App:       u.App,
		Username:  u.Username,
		Roles:     u.Roles,
		Perms:     u.Permissions,
		Type:      TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    t.issuer,
			Audience:  jwt.ClaimStrings{t.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.accessTTL)),
		},
	}
	return t.sign(claims)
}

// IssueRefreshToken signs a long-lived refresh token carrying a unique jti.
// The jti is registered in the SessionStore so it can be rotated (todo 6.1.3).
func (t *TokenIssuer) IssueRefreshToken(u *User) (string, error) {
	now := time.Now()
	claims := refreshClaims{
		Workspace: u.WorkspaceID,
		App:       u.App,
		Type:      TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			Issuer:    t.issuer,
			Audience:  jwt.ClaimStrings{t.audience},
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.refreshTTL)),
		},
	}
	return t.sign(claims)
}

// ParseRefreshToken validates a refresh token and returns its claims.
// It rejects tokens that are not of type "refresh".
func (t *TokenIssuer) ParseRefreshToken(tokenString string) (*refreshClaims, error) {
	claims := &refreshClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", token.Header["alg"])
		}
		return t.secret, nil
	}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	if err != nil {
		return nil, ErrInvalidRefreshToken
	}
	if claims.Type != TokenTypeRefresh {
		return nil, ErrInvalidRefreshToken
	}
	if claims.ID == "" {
		return nil, ErrInvalidRefreshToken
	}
	return claims, nil
}

// AccessTTL returns the configured access token lifetime.
func (t *TokenIssuer) AccessTTL() time.Duration { return t.accessTTL }

// sign signs a claim set with the HMAC secret.
func (t *TokenIssuer) sign(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.secret)
}
