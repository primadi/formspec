// Active workspace resolution for `formspec dev` (kafe ledger 2.8 / gap #48).
//
// A `kind: Workspace` manifest is a **seed declaration**: it registers a slug, it
// does not select one. The workspace actually used at runtime comes from
// `--workspace-id` (or the config file), defaulting to `default`. Reading a
// declared workspace as "the active one" is an easy and silent mistake:
//
//	spec/workspaces/kafe.yaml exists  →  dev announces /default/_admin
//	POST /default/...                 →  stored with tenant_id "default"
//	GET  /kafe/...                    →  200 OK, total=0   ← looks healthy, is not
//
// So the run either adopts the only declared workspace, or says out loud which
// tenant it is using and which ones the tree declares. The data was never lost —
// it just lived somewhere the author did not expect.
package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/primadi/formspec/internal/manifest"
	"github.com/primadi/formspec/pkg/spec"
)

// declaredWorkspaces returns the slugs registered by `kind: Workspace`
// manifests under specPath, sorted. A tree that fails to load yields nil: the
// warning is a courtesy, and a broken spec tree is reported by validate/dev
// itself.
func declaredWorkspaces(specPath string) []string {
	res, err := manifest.NewLoader(specPath).LoadAll()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range res.Manifests {
		if spec.Kind(m.Kind) != spec.KindWorkspace || m.Metadata.Name == "" {
			continue
		}
		if seen[m.Metadata.Name] {
			continue
		}
		seen[m.Metadata.Name] = true
		out = append(out, m.Metadata.Name)
	}
	sort.Strings(out)
	return out
}

// resolveActiveWorkspace applies the #48 rule to cfg, printing what it decided.
//
//   - explicit (`--workspace-id` or config file): used as given; if it is not
//     among the declared workspaces, that is said out loud (a typo would send
//     every write to a tenant nobody declared).
//   - not explicit, exactly one declared: that one is adopted — the case where
//     the manifest reads like a choice and the default would be surprising.
//   - not explicit, several declared: stays `default`, and the list is printed
//     so the mismatch is visible at startup rather than at "why is my data
//     empty?".
func resolveActiveWorkspace(cfg DevConfig) DevConfig {
	declared := declaredWorkspaces(cfg.SpecPath)
	if len(declared) == 0 {
		return cfg
	}
	declaredList := joinQuoted(declared)

	if cfg.WorkspaceIDExplicit {
		if !containsString(declared, cfg.WorkspaceID) {
			fmt.Fprintf(os.Stderr,
				"[formspec] warning: workspace %q is not declared by this spec tree (declared: %s) — everything will be stored under tenant %q. Workspace manifests register slugs; --workspace-id selects one.\n",
				cfg.WorkspaceID, declaredList, cfg.WorkspaceID)
		}
		return cfg
	}

	if len(declared) == 1 {
		cfg.WorkspaceID = declared[0]
		fmt.Printf("[formspec] workspace: %s (the only one declared under spec/workspaces; override with --workspace-id)\n", cfg.WorkspaceID)
		return cfg
	}

	fmt.Fprintf(os.Stderr,
		"[formspec] warning: this spec tree declares %d workspaces (%s) but the active one is %q — pass --workspace-id to choose; workspace manifests register slugs, they do not select one.\n",
		len(declared), declaredList, cfg.WorkspaceID)
	return cfg
}

func joinQuoted(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%q", s)
	}
	return out
}
