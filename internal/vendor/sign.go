package vendor

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ed25519 signing for module distribution (todo 13.3.6, 07-marketplace.md §2):
// the publisher signs the module's tree checksum; the registry (and every
// `module install --from`) verifies the signature against the vendor's
// public key before trusting the tarball.

// KeyPair is an ed25519 keypair encoded as base64 strings.
type KeyPair struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

// GenerateKeyPair creates a new ed25519 keypair (base64-encoded).
func GenerateKeyPair() (KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{
		Public:  base64.StdEncoding.EncodeToString(pub),
		Private: base64.StdEncoding.EncodeToString(priv),
	}, nil
}

// SignChecksum signs a tree checksum string ("sha256:...") with a base64
// private key, returning the base64 signature.
func SignChecksum(privateKeyB64, checksum string) (string, error) {
	priv, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKeyB64))
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("private key size %d, want %d", len(priv), ed25519.PrivateKeySize)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(priv), []byte(checksum))
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifyChecksum verifies a signature over a tree checksum with a base64
// public key.
func VerifyChecksum(publicKeyB64, checksum, signatureB64 string) error {
	pub, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKeyB64))
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signatureB64))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("public key size %d, want %d", len(pub), ed25519.PublicKeySize)
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), []byte(checksum), sig) {
		return fmt.Errorf("signature verification FAILED — tarball does not match the signed checksum")
	}
	return nil
}

// SaveKeyPair writes the keypair to files: <name>.key (private, 0600) and
// <name>.pub (public).
func SaveKeyPair(dir, name string, kp KeyPair) (privPath, pubPath string, err error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", err
	}
	privPath = filepath.Join(dir, name+".key")
	pubPath = filepath.Join(dir, name+".pub")
	if err := os.WriteFile(privPath, []byte(kp.Private+"\n"), 0600); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(pubPath, []byte(kp.Public+"\n"), 0644); err != nil {
		return "", "", err
	}
	return privPath, pubPath, nil
}

// LoadKeyFile reads a base64 key file (single line, trailing newline ok).
func LoadKeyFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("%s is empty", path)
	}
	return key, nil
}

// DerivePublicKey derives the base64 public key from a base64 private key
// (publish sends it alongside the signature so the registry can register
// the vendor identity).
func DerivePublicKey(privateKeyB64 string) (string, error) {
	priv, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKeyB64))
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("private key size %d, want %d", len(priv), ed25519.PrivateKeySize)
	}
	pub := priv[ed25519.PublicKeySize:]
	return base64.StdEncoding.EncodeToString(pub), nil
}
