// Command `formspec override` — shadow copy management (todo 13.2,
// docs/spec/platform/08-project-layout.md §6.4, technical note §5).
//
//	formspec override adopt <module> <kind> <name> [--spec <path>] [--project <dir>]
//	formspec override diff <module> <kind> <name> [--project <dir>]
//	formspec override list [--project <dir>]
//
// adopt copies the upstream manifest to overrides/{module}/{kind}.{name}.yaml
// and records the fork-base checksum in formspec.lock (drift detection).
// Only presentation kinds (Form, VisualSpecKind) are shadow-copyable —
// enforced here AND at boot.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/primadi/formspec/internal/vendor"
)

func runOverride(args []string) {
	if len(args) < 1 {
		usageOverride()
		os.Exit(2)
	}
	switch args[0] {
	case "adopt":
		runOverrideAdopt(args[1:])
	case "diff":
		runOverrideDiff(args[1:])
	case "list":
		runOverrideList(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "formspec override: unknown action %q (want adopt|diff|list)\n", args[0])
		os.Exit(2)
	}
}

func usageOverride() {
	fmt.Fprintf(os.Stderr, "Usage: formspec override <adopt|diff|list> [flags]\n")
}

func runOverrideAdopt(args []string) {
	fs := flag.NewFlagSet("override adopt", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	specPath := fs.String("spec", "spec", "spec directory")
	projectRoot := fs.String("project", ".", "project root (formspec.lock, overrides/)")
	positional := reorderFlags(fs, args, nil)
	if len(positional) < 3 {
		fmt.Fprintln(os.Stderr, "formspec override adopt: usage: override adopt <module> <kind> <name>")
		os.Exit(2)
	}

	res, err := vendor.Adopt(*projectRoot, *specPath, positional[0], positional[1], positional[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec override adopt: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("adopted: %s/%s (module %s)\n", res.Kind, res.Name, res.Module)
	fmt.Printf("  upstream: %s\n", res.Source)
	fmt.Printf("  shadow:   %s\n", res.OverridePath)
	fmt.Printf("  base:     %s\n", res.BaseChecksum)
	fmt.Println("\nEdit the shadow copy freely — it replaces the upstream file at boot (full-replace).")
}

func runOverrideDiff(args []string) {
	fs := flag.NewFlagSet("override diff", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	projectRoot := fs.String("project", ".", "project root")
	specPath := fs.String("spec", "spec", "spec directory")
	positional := reorderFlags(fs, args, nil)
	if len(positional) < 3 {
		fmt.Fprintln(os.Stderr, "formspec override diff: usage: override diff <module> <kind> <name>")
		os.Exit(2)
	}

	diff, err := vendor.DiffOverride(*projectRoot, *specPath, positional[0], positional[1], positional[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec override diff: %v\n", err)
		os.Exit(1)
	}
	if diff.Drift {
		fmt.Printf("⚠ upstream changed since adopt — your shadow copy does NOT automatically receive upstream changes\n")
		fmt.Printf("  upstream: %s\n\n", diff.Upstream)
	}
	if diff.Unified == "" {
		fmt.Println("shadow copy identical to upstream.")
		return
	}
	fmt.Printf("--- upstream\n+++ overrides/%s/%s.%s.yaml\n\n%s\n",
		diff.Module, diff.Kind, diff.Name, diff.Unified)
}

func runOverrideList(args []string) {
	fs := flag.NewFlagSet("override list", flag.ExitOnError)
	fs.SetOutput(os.Stderr)
	projectRoot := fs.String("project", ".", "project root")
	reorderFlags(fs, args, nil)

	lock, err := vendor.LoadLock(filepath.Join(*projectRoot, "formspec.lock"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "formspec override list: %v\n", err)
		os.Exit(1)
	}
	count := 0
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "MODULE\tKIND\tNAME\tORIGIN\tADOPTED")
	for _, m := range lock.Modules {
		for _, ov := range m.Overrides {
			count++
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", m.EffectiveName(), ov.Kind, ov.Name, ov.Origin, ov.AdoptedAt)
		}
	}
	w.Flush()
	if count == 0 {
		fmt.Println("tidak ada shadow copy (formspec override adopt <module> <kind> <name>).")
	}
}
