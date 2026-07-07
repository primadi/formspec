package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JWTValidator validates JWT tokens and extracts Identity claims.
//
// Supports HS256 (shared secret) and RS256 (public key) algorithms.
// Token validation checks: signature, expiration, issuer, and audience.
//
// Expected claims in the JWT payload:
//   - sub:      user ID (required)
//   - ws:       workspace ID (required)
//   - perms:    comma-separated permission list (optional)
//   - roles:    comma-separated role list (optional)
//   - iss:      issuer (validated against config)
//   - aud:      audience (validated against config)
//   - exp, nbf: standard time-based claims
type JWTValidator struct {
	secret     string
	publicKey  any // *rsa.PublicKey or *ecdsa.PublicKey
	issuer     string
	audience   string
	signingKey any // for HMAC: []byte(secret); for RSA/ECDSA: publicKey
}

// NewJWTValidator creates a JWT validator with a shared secret (HS256).
func NewJWTValidator(secret, issuer, audience string) *JWTValidator {
	return &JWTValidator{
		secret:     secret,
		issuer:     issuer,
		audience:   audience,
		signingKey: []byte(secret),
	}
}

// NewJWTValidatorWithKey creates a JWT validator with an asymmetric public key.
func NewJWTValidatorWithKey(publicKey any, issuer, audience string) *JWTValidator {
	return &JWTValidator{
		publicKey:  publicKey,
		issuer:     issuer,
		audience:   audience,
		signingKey: publicKey,
	}
}

// Validate parses and validates a JWT token, returning the caller's Identity.
func (v *JWTValidator) Validate(ctx context.Context, tokenString string) (*Identity, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("auth: empty token")
	}

	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"HS256", "HS384", "HS512", "RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}),
	}
	if v.issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.issuer))
	}
	if v.audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.audience))
	}

	parser := jwt.NewParser(parserOpts...)

	token, err := parser.Parse(tokenString, func(token *jwt.Token) (any, error) {
		// Validate signing algorithm
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); ok {
			if v.secret == "" {
				return nil, fmt.Errorf("auth: HMAC not configured")
			}
			return v.signingKey, nil
		}
		if v.publicKey == nil {
			return nil, fmt.Errorf("auth: asymmetric key not configured")
		}
		return v.signingKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: token validation: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("auth: invalid token claims")
	}

	// Extract required claims
	userID, err := claims.GetSubject()
	if err != nil || userID == "" {
		return nil, fmt.Errorf("auth: missing sub claim")
	}

	workspaceID, ok := claims["ws"].(string)
	if !ok || workspaceID == "" {
		return nil, fmt.Errorf("auth: missing ws (workspace) claim")
	}

	// Extract optional permission list
	var permissions []string
	if permsRaw, ok := claims["perms"]; ok {
		switch p := permsRaw.(type) {
		case string:
			permissions = splitCommaList(p)
		case []any:
			for _, item := range p {
				if s, ok := item.(string); ok {
					permissions = append(permissions, s)
				}
			}
		}
	}

	// Extract optional role list
	var roles []string
	if rolesRaw, ok := claims["roles"]; ok {
		switch r := rolesRaw.(type) {
		case string:
			roles = splitCommaList(r)
		case []any:
			for _, item := range r {
				if s, ok := item.(string); ok {
					roles = append(roles, s)
				}
			}
		}
	}

	return &Identity{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Permissions: permissions,
		Roles:       roles,
	}, nil
}

// splitCommaList splits "a, b, c" into ["a", "b", "c"].
func splitCommaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
