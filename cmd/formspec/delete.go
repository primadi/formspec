// Command `formspec delete` — remove a resource from a spec tree
// (docs/cli-tools/02-formspec-cli.md §3). The Control Plane is deferred, so
// this operates against local manifests: it removes the manifest for the
// given kind+name.
//
//	formspec delete entity menu-item --confirm
//
// A single-document file is deleted entirely; a multi-document file has only
// the matching document removed (the rest are preserved). --confirm is
// required — without it nothing is deleted.
package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/primadi/formspec/internal/manifest"
)

func runDelete(args []string) {
	specPath := "spec"
	confirm := false
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--spec", "-spec":
			if i+1 < len(args) {
				specPath = args[i+1]
				i++
			}
		case "--confirm", "-confirm":
			confirm = true
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "Usage: formspec delete <kind> <name> --confirm [--spec <path>]\n")
			os.Exit(0)
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: formspec delete <kind> <name> --confirm [--spec <path>]\n")
		os.Exit(2)
	}
	kind := positional[0]
	name := positional[1]

	if !confirm {
		fmt.Fprintf(os.Stderr, "Error: --confirm is required to delete %s %q (destructive)\n", kind, name)
		os.Exit(1)
	}

	manifests := loadManifestsOrExit(specPath)

	// Find the matching manifest.
	var match *manifest.RawManifest
	for i := range manifests {
		m := &manifests[i]
		if kindMatches(m.Kind, kind) && m.Metadata.Name == name {
			match = m
			break
		}
	}
	if match == nil {
		fmt.Fprintf(os.Stderr, "No resource found for kind %q name %q under %s\n", kind, name, specPath)
		os.Exit(1)
	}

	file := strings.SplitN(match.Source, "#", 2)[0]
	docIndex := 0
	if len(strings.SplitN(match.Source, "#", 2)) == 2 {
		fmt.Sscanf(strings.SplitN(match.Source, "#", 2)[1], "%d", &docIndex)
	}

	// Count documents in the file.
	docCount, err := countYAMLDocs(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if docCount <= 1 {
		// Single-document file — delete the whole file.
		if err := os.Remove(file); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot delete %s: %v\n", file, err)
			os.Exit(1)
		}
		fmt.Printf("Deleted %s %q (%s)\n", match.Kind, name, file)
		return
	}

	// Multi-document file — remove only the matching document.
	if err := removeYAMLDoc(file, docIndex); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Deleted %s %q (document %d of %s)\n", match.Kind, name, docIndex, file)
}

// countYAMLDocs returns the number of documents in a YAML file.
func countYAMLDocs(file string) (int, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	count := 0
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return 0, err
		}
		count++
	}
	return count, nil
}

// removeYAMLDoc removes the document at index from a multi-document YAML file
// and writes the remaining documents back.
func removeYAMLDoc(file string, index int) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
		docs = append(docs, &doc)
	}
	if index < 0 || index >= len(docs) {
		return fmt.Errorf("document index %d out of range (file has %d docs)", index, len(docs))
	}

	var out strings.Builder
	for i, doc := range docs {
		if i == index {
			continue
		}
		if out.Len() > 0 {
			out.WriteString("---\n")
		}
		b, err := yaml.Marshal(doc)
		if err != nil {
			return err
		}
		out.Write(b)
	}
	return os.WriteFile(file, []byte(out.String()), 0644)
}
