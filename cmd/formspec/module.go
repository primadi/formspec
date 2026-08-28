// Command `formspec module` — module vendoring CLI (todo 13.1.2/13.1.5,
// docs/cli-tools/02-formspec-cli.md §9, docs/spec/platform/08-project-layout.md §6).
//
//	formspec module install <source> [--use] [--version <tag>] [--spec <path>] [--project <dir>]
//	formspec module list [--spec <path>] [--project <dir>]
//	formspec module uninstall <effective-name> [--spec <path>] [--project <dir>]
//	formspec verify [--project <dir>]
//
// Sources: git URL, local folder, or .tar.gz (offline-first; registry comes
// with 13.3). Install writes vendors/{effective}/, formspec.lock, and a
// marker block in the App manifest (commented = inactive unless --use).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/primadi/formspec/internal/vendor"
)

func runModule(args []string) {
	if len(args) < 1 {
		usageModule()
		os.Exit(2)
	}
	switch args[0] {
	case "install":
		// `install --from <registry> <module>[@ver]` → registry flow (13.3.8);
		// plain `install <source>` → local flow (13.1.2).
		hasFrom := false
		for i, a := range args {
			if a == "--from" && i+1 < len(args) {
				hasFrom = true
			}
		}
		if hasFrom {
			runModuleInstallFrom(args[1:])
		} else {
			runModuleInstall(args[1:])
		}
	case "publish":
		runModulePublish(args[1:])
	case "list":
		runModuleList(args[1:])
	case "uninstall":
		runModuleUninstall(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "formspec module: unknown action %q (want install|publish|list|uninstall)\n", args[0])
		os.Exit(2)
	}
}

func usageModule() {
	fmt.Fprintf(os.Stderr, "Usage: formspec module <install|list|uninstall> [flags]\n")
	fmt.Fprintf(os.Stderr, "       formspec verify\n")
}

func moduleFlags(fs *flag.FlagSet) (*string, *string) {
	specPath := fs.String("spec", "spec", "spec directory (App manifest discovery + conflict scan)")
	projectRoot := fs.String("project", ".", "project root (formspec.lock, vendors/)")
	return specPath, projectRoot
}

// reorderFlags moves flag tokens before positional tokens. Go's flag package
// stops parsing at the first non-flag argument, but users naturally write
// `module install <source> --use` — flags after the positional. boolFlags
// lists flags that take no value; every other flag consumes the next token.
func reorderFlags(fs *flag.FlagSet, args []string, boolFlags map[string]bool) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			pos = append(pos, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue // value inline (--flag=value)
		}
		if !boolFlags[name] && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	fs.Parse(flags)
	return pos
}

func runModuleInstall(args []string) {
	fs := flag.NewFlagSet("module install", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	specPath, projectRoot := moduleFlags(fs)
	version := fs.String("version", "", "version tag (git ref / tarball label; default auto)")
	use := fs.Bool("use", false, "activate the module immediately (uncomment the marker entry)")
	positional := reorderFlags(fs, args, map[string]bool{"use": true})
	if len(positional) < 1 {
		fmt.Fprintln(os.Stderr, "formspec module install: source is required (git URL, folder, or .tar.gz)")
		os.Exit(2)
	}

	res, err := vendor.Install(context.Background(), positional[0], vendor.Options{
		ProjectRoot: *projectRoot,
		SpecPath:    *specPath,
		Version:     *version,
		Use:         *use,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module install: %v\n", err)
		os.Exit(1)
	}

	state := "inactive"
	if res.Active {
		state = "ACTIVE"
	}
	action := "installed"
	if res.Updated {
		action = "updated"
	}
	fmt.Printf("%s %s (effective name: %s)\n", action, res.Entry.Source, res.Entry.EffectiveName())
	fmt.Printf("  version:  %s\n", res.Entry.Version)
	fmt.Printf("  checksum: %s\n", res.Entry.Checksum)
	fmt.Printf("  trust:    %s\n", res.Entry.TrustTier)
	fmt.Printf("  dir:      %s\n", res.Dir)
	fmt.Printf("  state:    %s (marker in %s)\n", state, res.AppManifest)
	if !res.Active {
		fmt.Printf("\nAktifkan: uncomment entri di %s, atau install ulang dengan --use\n", res.AppManifest)
	}
	fmt.Printf("\nIntegritas: formspec verify\n")
}

func runModuleList(args []string) {
	fs := flag.NewFlagSet("module list", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	specPath, projectRoot := moduleFlags(fs)
	reorderFlags(fs, args, nil)

	lock, err := vendor.LoadLock(filepath.Join(*projectRoot, "formspec.lock"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module list: %v\n", err)
		os.Exit(1)
	}
	active, err := vendor.ActiveModules(*projectRoot, *specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module list: %v\n", err)
		os.Exit(1)
	}
	isActive := map[string]bool{}
	for _, a := range active {
		isActive[a] = true
	}

	if len(lock.Modules) == 0 {
		fmt.Println("tidak ada module vendor terpasang.")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tTRUST\tSTATE\tSOURCE")
	for _, m := range lock.Modules {
		state := "inactive"
		if isActive[m.EffectiveName()] {
			state = "active"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", m.EffectiveName(), m.Version, m.TrustTier, state, m.Source)
	}
	w.Flush()
}

func runModuleUninstall(args []string) {
	fs := flag.NewFlagSet("module uninstall", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	specPath, projectRoot := moduleFlags(fs)
	positional := reorderFlags(fs, args, nil)
	if len(positional) < 1 {
		fmt.Fprintln(os.Stderr, "formspec module uninstall: effective name is required (lihat formspec module list)")
		os.Exit(2)
	}

	removed, err := vendor.Uninstall(*projectRoot, *specPath, positional[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec module uninstall: %v\n", err)
		os.Exit(1)
	}
	if !removed {
		fmt.Fprintf(os.Stderr, "formspec module uninstall: module %q tidak terpasang\n", fs.Arg(0))
		os.Exit(1)
	}
	fmt.Printf("uninstalled: %s (vendors/ + lock + marker dibersihkan)\n", fs.Arg(0))
}

// runVerify implements `formspec verify` (todo 13.1.6): checksum tree
// vendors/ vs lock — manual modifications are detected here.
func runVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	projectRoot := fs.String("project", ".", "project root (formspec.lock, vendors/)")
	reorderFlags(fs, args, nil)

	results, err := vendor.Verify(*projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec verify: %v\n", err)
		os.Exit(1)
	}
	if len(results) == 0 {
		fmt.Println("tidak ada module vendor terpasang — nothing to verify.")
		return
	}
	fails := 0
	for _, r := range results {
		if r.OK {
			fmt.Printf("[OK]   %s\n", r.EffectiveName)
			continue
		}
		fails++
		fmt.Printf("[FAIL] %s — %s\n", r.EffectiveName, r.Reason)
	}
	if fails > 0 {
		fmt.Printf("\n%d module termodifikasi manual — restore via install ulang atau perbaiki checksum di formspec.lock\n", fails)
		os.Exit(1)
	}
	fmt.Printf("\n%d module verified.\n", len(results))
}
