package artifact

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Signer creates and verifies ed25519 signatures on artifact envelopes.
type Signer struct {
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
	keyID      string
}

// NewSigner creates a Signer from an ed25519 key pair.
func NewSigner(publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey, keyID string) *Signer {
	return &Signer{
		publicKey:  publicKey,
		privateKey: privateKey,
		keyID:      keyID,
	}
}

// NewDevSigner generates a fresh ed25519 key pair for development use.
// The key is ephemeral — not suitable for production.
func NewDevSigner() (*Signer, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate dev key: %w", err)
	}
	return &Signer{
		publicKey:  pub,
		privateKey: priv,
		keyID:      "dev-self-signed",
	}, nil
}

// KeyID returns the identifier of the signing key.
func (s *Signer) KeyID() string { return s.keyID }

// PublicKey returns the public key bytes.
func (s *Signer) PublicKey() ed25519.PublicKey { return s.publicKey }

// Sign signs the artifact envelope and sets the Signature and SigningKeyID fields.
// The signature is computed over the canonical JSON representation of the envelope
// (with signature fields zeroed), ensuring deterministic output.
func (s *Signer) Sign(envelope *ArtifactEnvelope) error {
	if envelope == nil {
		return fmt.Errorf("envelope is nil")
	}

	// Save and zero signature fields for canonical signing
	origSig := envelope.Signature
	origKeyID := envelope.SigningKeyID
	envelope.Signature = ""
	envelope.SigningKeyID = ""

	// Marshal to canonical JSON
	data, err := json.Marshal(envelope)
	if err != nil {
		envelope.Signature = origSig
		envelope.SigningKeyID = origKeyID
		return fmt.Errorf("marshal envelope for signing: %w", err)
	}

	// Sign
	sig := ed25519.Sign(s.privateKey, data)
	envelope.Signature = hex.EncodeToString(sig)
	envelope.SigningKeyID = s.keyID

	return nil
}

// Verify checks the ed25519 signature on an artifact envelope.
// It returns nil if the signature is valid, or an error describing the failure.
func Verify(envelope *ArtifactEnvelope, publicKey ed25519.PublicKey) error {
	if envelope == nil {
		return fmt.Errorf("envelope is nil")
	}
	if envelope.Signature == "" {
		return fmt.Errorf("envelope has no signature")
	}

	// Decode signature
	sig, err := hex.DecodeString(envelope.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	// Save and zero signature fields for canonical verification
	origSig := envelope.Signature
	origKeyID := envelope.SigningKeyID
	envelope.Signature = ""
	envelope.SigningKeyID = ""

	defer func() {
		envelope.Signature = origSig
		envelope.SigningKeyID = origKeyID
	}()

	// Marshal to canonical JSON (same as during signing)
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope for verification: %w", err)
	}

	// Verify
	if !ed25519.Verify(publicKey, data, sig) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// GenerateKeyPair generates a new ed25519 key pair.
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}
