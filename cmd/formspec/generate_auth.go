package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/primadi/formspec/internal/auth"
)

// runGenerateAuth scaffolds the bundled auth module (formspec.core entities)
// into the target directory so the user can customize it. The default target
// is external/auth — a user-customized module that is committed to git and
// wins over the built-in formspec.core defaults (todo 6.1 merge strategy).
//
// The scaffold is copied from the embedded module (internal/auth/module), so
// it stays in sync with the bundled entities — including any added later
// (app-membership, api-key, workspace).
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

// generateAuthModule copies the bundled auth module (embedded in the auth
// package) into dir. It fails if a target file already exists unless force
// is set.
func generateAuthModule(dir string, force bool) error {
	moduleFS := auth.ModuleFS()
	return fs.WalkDir(moduleFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		dst := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(path, "module/")))
		if _, err := os.Stat(dst); err == nil && !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", dst)
		}

		src, err := moduleFS.Open(path)
		if err != nil {
			return fmt.Errorf("open embedded %s: %w", path, err)
		}
		defer src.Close()

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
		}
		out, err := os.Create(dst)
		if err != nil {
			return fmt.Errorf("create %s: %w", dst, err)
		}
		if _, err := io.Copy(out, src); err != nil {
			out.Close()
			return fmt.Errorf("copy %s: %w", dst, err)
		}
		if err := out.Close(); err != nil {
			return fmt.Errorf("close %s: %w", dst, err)
		}
		return nil
	})
}
