package formspec

import (
	"context"

	"github.com/primadi/formspec/internal/auth"
)

// hashUserPassword is a before-create/update hook for formspec.core.user.
// It reads the plaintext `password` field from the resource, hashes it into
// `password_hash`, and removes `password` so plaintext never reaches the
// store. An empty password (e.g. editing without changing it) leaves the
// existing hash untouched.
func hashUserPassword(_ context.Context, params NativeParams) (any, error) {
	res := params.Resource
	pw, _ := res["password"].(string)
	delete(res, "password")
	if pw == "" {
		return nil, nil
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return nil, err
	}
	res["password_hash"] = hash
	return nil, nil
}
