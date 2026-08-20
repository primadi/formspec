package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost is the work factor for password hashing. Default cost (10) is a
// good balance of security and latency for interactive login.
const bcryptCost = bcrypt.DefaultCost

// HashPassword returns a bcrypt hash of the plaintext password.
// The returned string is safe to store and compare with VerifyPassword.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword reports whether plain matches the stored bcrypt hash.
// A malformed hash returns false (never an error to the caller).
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
