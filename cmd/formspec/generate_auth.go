package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// runGenerateAuth scaffolds the auth module (formspec.core user/session
// entities) into the target directory so the user can customize it. The
// default target is external/auth — a user-customized module that is
// committed to git and wins over the built-in formspec.core defaults
// (todo 6.1 merge strategy).
//
// Usage:
//
//	formspec generate auth [--to external/auth] [--force]
func runGenerateAuth(args []string) {
	fs := flag.NewFlagSet("generate auth", flag.ExitOnError)
	to := fs.String("to", "external/auth", "target directory for the auth module")
	force := fs.Bool("force", false, "overwrite existing files")
	fs.Parse(args)

	if err := generateAuthModule(*to, *force); err != nil {
		fmt.Fprintf(os.Stderr, "formspec generate auth: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "formspec: auth module scaffolded at %s\n", *to)
	fmt.Fprintf(os.Stderr, "  - customize the entities there, then set Config.ExternalDir (or auth_config_ref) to use them\n")
}

// generateAuthModule writes the auth module manifests into dir.
func generateAuthModule(dir string, force bool) error {
	files := map[string]string{
		"module.yaml": `apiVersion: formspec.dev/v1
kind: Module
metadata:
  name: auth
  description: "User-customized auth module (cloned from formspec.core)"
spec:
  version: 1.0.0
`,
		"master/user/entity.yaml": `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: user
  module: formspec.core
  description: "User account (auth) — customized copy of the built-in"
spec:
  version: v1
  plural: users
  characteristic: master
  display_field: username
  fields:
    - name: username
      type: string
      required: true
      unique: true
      index: true
    - name: password_hash
      type: string
      required: true
      masked: true
    - name: display_name
      type: string
    - name: email
      type: string
    - name: roles
      type: json
    - name: permissions
      type: json
    - name: active
      type: boolean
      default: true
`,
		"transaction/session/entity.yaml": `apiVersion: formspec.dev/v1
kind: Entity
metadata:
  name: session
  module: formspec.core
  description: "Login session (auth) — customized copy of the built-in"
spec:
  version: v1
  plural: sessions
  characteristic: transaction
  fields:
    - name: transaction_date
      type: date
      required: true
    - name: refresh_jti
      type: string
      required: true
      unique: true
      index: true
    - name: user_id
      type: string
      required: true
      index: true
    - name: expires_at
      type: datetime
      required: true
    - name: ip
      type: string
    - name: user_agent
      type: string
`,
	}

	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); err == nil && !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
