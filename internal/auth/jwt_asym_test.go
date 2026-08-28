package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Tests for asymmetric JWT validation (todo 8.1.2): RS256 and ES256 must be
// fully functional — sign with the private key, validate with
// NewJWTValidatorWithKey — and cross-algorithm tokens must be rejected.

func makeAsymClaims(issuer string) jwt.MapClaims {
	return jwt.MapClaims{
		"sub": "user-asym",
		"ws":  "ws-asym",
		"iss": issuer,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	}
}

func TestJWTValidator_RS256(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	issuer := "formspec-asym"
	validator := NewJWTValidatorWithKey(&key.PublicKey, issuer, "")

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, makeAsymClaims(issuer))
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign RS256: %v", err)
	}

	id, err := validator.Validate(context.Background(), signed)
	if err != nil {
		t.Fatalf("validate RS256: %v", err)
	}
	if id.UserID != "user-asym" || id.WorkspaceID != "ws-asym" {
		t.Errorf("identity = %q/%q, want user-asym/ws-asym", id.UserID, id.WorkspaceID)
	}
}

func TestJWTValidator_ES256(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}
	issuer := "formspec-asym"
	validator := NewJWTValidatorWithKey(&key.PublicKey, issuer, "")

	token := jwt.NewWithClaims(jwt.SigningMethodES256, makeAsymClaims(issuer))
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign ES256: %v", err)
	}

	id, err := validator.Validate(context.Background(), signed)
	if err != nil {
		t.Fatalf("validate ES256: %v", err)
	}
	if id.UserID != "user-asym" {
		t.Errorf("UserID = %q, want user-asym", id.UserID)
	}
}

func TestJWTValidator_RejectsWrongAlgorithm(t *testing.T) {
	// An HMAC-signed token must NOT validate against an asymmetric-only
	// validator (algorithm confusion hardening).
	issuer := "formspec-asym"
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	asymValidator := NewJWTValidatorWithKey(&rsaKey.PublicKey, issuer, "")

	hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, makeAsymClaims(issuer))
	signed, err := hsToken.SignedString([]byte("some-secret"))
	if err != nil {
		t.Fatalf("sign HS256: %v", err)
	}
	if _, err := asymValidator.Validate(context.Background(), signed); err == nil {
		t.Fatal("HMAC token validated against asymmetric validator — algorithm confusion")
	}

	// And an RS256 token must NOT validate against an HMAC-only validator.
	hmacValidator := NewJWTValidator("secret", issuer, "")
	rsToken := jwt.NewWithClaims(jwt.SigningMethodRS256, makeAsymClaims(issuer))
	rsSigned, err := rsToken.SignedString(rsaKey)
	if err != nil {
		t.Fatalf("sign RS256: %v", err)
	}
	if _, err := hmacValidator.Validate(context.Background(), rsSigned); err == nil {
		t.Fatal("RS256 token validated against HMAC validator — algorithm confusion")
	}
}

func TestJWTValidator_RejectsWrongKey(t *testing.T) {
	key1, _ := rsa.GenerateKey(rand.Reader, 2048)
	key2, _ := rsa.GenerateKey(rand.Reader, 2048)
	issuer := "formspec-asym"
	validator := NewJWTValidatorWithKey(&key2.PublicKey, issuer, "")

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, makeAsymClaims(issuer))
	signed, err := token.SignedString(key1) // signed with a different key
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := validator.Validate(context.Background(), signed); err == nil {
		t.Fatal("token signed by another key validated — signature check broken")
	}
}
