// Publish & registry install for `formspec module` (todo 13.3.7/13.3.8).
//
//	formspec module publish <module-dir> --vendor <name> --key <private.key>
//	                       [--registry URL] [--version <semver>] [--api-key <key>]
//	formspec module install --from <registry> <module>[@<version>]
//	                       [--project <dir>] [--spec <path>] [--use]
//
// publish: tar the module → tree checksum → ed25519 sign → find-or-create
// vendor/module → create version → upload tarball (13.3.4 REST surface).
// install --from: download tarball + signature → verify against the
// vendor's registered public key → 13.1 install flow.
package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/internal/vendor"
)

func runModulePublish(args []string) {
	fs := flag.NewFlagSet("module publish", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	vendorName := fs.String("vendor", "", "vendor name (ed25519 identity owner)")
	key := fs.String("key", "", "vendor private key file (base64)")
	registry := fs.String("registry", registryDefault(), "registry base URL")
	version := fs.String("version", "", "semver tag (required)")
	apiKey := fs.String("api-key", os.Getenv("FORMSPEC_REGISTRY_API_KEY"), "registry API key (publish is authenticated, 13.3.4)")
	specPath := fs.String("spec", "spec", "spec directory (module name resolution fallback)")
	positional := reorderFlags(fs, args, nil)
	if len(positional) < 1 || *vendorName == "" || *key == "" || *version == "" {
		fmt.Fprintln(os.Stderr, "formspec module publish: usage: module publish <module-dir> --vendor <name> --key <private.key> --version <semver>")
		os.Exit(2)
	}

	// ── Checksum + signature ──
	checksum, err := vendor.TreeChecksum(positional[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module publish: %v\n", err)
		os.Exit(1)
	}
	priv, err := vendor.LoadKeyFile(*key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module publish: %v\n", err)
		os.Exit(1)
	}
	signature, err := vendor.SignChecksum(priv, checksum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module publish: %v\n", err)
		os.Exit(1)
	}
	publicKey, err := derivePublicKey(priv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module publish: %v\n", err)
		os.Exit(1)
	}

	// ── Tarball ──
	tmp, err := os.MkdirTemp("", "formspec-publish-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module publish: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)
	tarball := filepath.Join(tmp, fmt.Sprintf("%s-%s.tar.gz", filepath.Base(positional[0]), *version))
	if err := createTarball(positional[0], tarball); err != nil {
		fmt.Fprintf(os.Stderr, "formspec module publish: %v\n", err)
		os.Exit(1)
	}

	// ── Module name: from module.yaml (via the vendor package's parser) ──
	moduleName, err := readPublishModuleName(positional[0], *specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module publish: %v\n", err)
		os.Exit(1)
	}

	client := vendor.NewRegistryClient(*registry, "default", *apiKey)
	res, err := client.PublishModule(context.Background(), vendor.PublishOptions{
		VendorName:  *vendorName,
		ModuleName:  moduleName,
		Version:     *version,
		Checksum:    checksum,
		Signature:   signature,
		PublicKey:   publicKey,
		TarballPath: tarball,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module publish: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("published: %s@%s\n", moduleName, *version)
	fmt.Printf("  checksum: %s\n", checksum)
	fmt.Printf("  version id: %s\n", res.VersionID)
	fmt.Printf("  registry: %s\n", *registry)
}

// runModuleInstallFrom implements `formspec module install --from <registry>
// <module>[@<version>]` (todo 13.3.8): download → verify signature against
// the vendor's registered public key → 13.1 install flow.
func runModuleInstallFrom(args []string) {
	fs := flag.NewFlagSet("module install --from", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	specPath, projectRoot := moduleFlags(fs)
	use := fs.Bool("use", false, "activate the module immediately")
	registryFlag := fs.String("registry", "", "registry base URL (overrides --from value)")
	from := fs.String("from", "", "registry base URL (npm-like position: install --from <registry> <module>[@ver])")
	positional := reorderFlags(fs, args, map[string]bool{"use": true})
	registryURL := *registryFlag
	if registryURL == "" {
		registryURL = *from
	}
	var moduleRef string
	for _, a := range positional {
		if a != "" {
			moduleRef = a
		}
	}
	if registryURL == "" {
		registryURL = registryDefault()
	}
	if moduleRef == "" {
		fmt.Fprintln(os.Stderr, "formspec module install: usage: module install --from <registry> <module>[@<version>]")
		os.Exit(2)
	}
	moduleName, version := splitModuleRef(moduleRef)
	if version == "" {
		version = "latest"
	}

	ctx := context.Background()
	client := vendor.NewRegistryClient(registryURL, "default", os.Getenv("FORMSPEC_REGISTRY_API_KEY"))

	// ── Lookup + download ──
	versionID, checksum, signature, publicKey, err := client.LookupVersion(ctx, moduleName, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module install: %v\n", err)
		os.Exit(1)
	}
	tmp, err := os.MkdirTemp("", "formspec-fetch-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module install: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)
	tarball := filepath.Join(tmp, "module.tar.gz")
	if err := client.DownloadTarball(ctx, versionID, tarball); err != nil {
		fmt.Fprintf(os.Stderr, "formspec module install: %v\n", err)
		os.Exit(1)
	}

	// ── Verify signature BEFORE trusting the tarball (13.3.8) ──
	if err := verifyTarballSignature(tarball, checksum, signature, publicKey); err != nil {
		fmt.Fprintf(os.Stderr, "formspec module install: REFUSED — %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("signature verified (registry vendor key)\n")

	// ── 13.1 install flow from the verified tarball ──
	res, err := vendor.Install(ctx, tarball, vendor.Options{
		ProjectRoot:    *projectRoot,
		SpecPath:       *specPath,
		Version:        version,
		Use:            *use,
		SourceOverride: registryURL + "/" + moduleName + "@" + version,
		Signature:      signature,
		TrustTier:      "community",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module install: %v\n", err)
		os.Exit(1)
	}
	state := "inactive"
	if res.Active {
		state = "ACTIVE"
	}
	fmt.Printf("installed from registry: %s@%s (effective name: %s)\n", moduleName, version, res.Entry.EffectiveName())
	fmt.Printf("  checksum: %s\n", res.Entry.Checksum)
	fmt.Printf("  state:    %s (marker in %s)\n", state, res.AppManifest)
}

// verifyTarballSignature extracts the tarball to a temp dir, recomputes its
// tree checksum, and verifies the registry-recorded signature.
func verifyTarballSignature(tarball, checksum, signature, publicKey string) error {
	if signature == "" || publicKey == "" {
		return fmt.Errorf("registry record has no signature/public key — refusing unsigned module")
	}
	tmp, err := os.MkdirTemp("", "formspec-verify-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := extractTarballTo(tarball, tmp); err != nil {
		return err
	}
	root, err := vendor.FindModuleRoot(tmp)
	if err != nil {
		return err
	}
	actual, err := vendor.TreeChecksum(root)
	if err != nil {
		return err
	}
	if actual != checksum {
		return fmt.Errorf("checksum mismatch: registry says %s, tarball is %s", checksum, actual)
	}
	return vendor.VerifyChecksum(publicKey, checksum, signature)
}

// registryDefault returns the default registry base URL.
func registryDefault() string {
	if v := os.Getenv("FORMSPEC_MODULE_REGISTRY"); v != "" {
		return v
	}
	return "https://registry.formspec.dev"
}

// splitModuleRef splits "billing@1.0.0" → ("billing", "1.0.0").
func splitModuleRef(ref string) (string, string) {
	if idx := strings.LastIndex(ref, "@"); idx > 0 {
		return ref[:idx], ref[idx+1:]
	}
	return ref, ""
}

// derivePublicKey derives the base64 public key from a base64 private key.
func derivePublicKey(privateKeyB64 string) (string, error) {
	return vendor.DerivePublicKey(privateKeyB64)
}

// createTarball gzips a directory tree (module root at the archive root).
func createTarball(src, dest string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			hdr.Name = filepath.ToSlash(rel) + "/"
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
}

// extractTarballTo extracts a .tar.gz into dest (path-traversal defended —
// same rules as internal/vendor extractTarball).
func extractTarballTo(tarball, dest string) error {
	f, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		target := filepath.Join(dest, filepath.Clean("/"+hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("tarball entry escapes destination: %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

// readPublishModuleName resolves the module's metadata.name: prefer the
// module.yaml inside the source dir, fall back to the spec tree registry.
func readPublishModuleName(moduleDir, specPath string) (string, error) {
	if _, err := os.Stat(filepath.Join(moduleDir, "module.yaml")); err == nil {
		return vendor.ReadModuleName(moduleDir)
	}
	// Fall back: find the module in the spec tree whose directory matches.
	base := filepath.Base(moduleDir)
	loader := manifest.NewLoader(specPath)
	res, err := loader.LoadAll()
	if err != nil {
		return "", err
	}
	for _, m := range res.Manifests {
		if strings.EqualFold(m.Kind, "Module") && strings.EqualFold(m.Metadata.Name, base) {
			return m.Metadata.Name, nil
		}
	}
	return "", fmt.Errorf("no module.yaml in %s and no kind: Module named %q under %s", moduleDir, base, specPath)
}
