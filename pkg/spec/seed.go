package spec

import "fmt"

// Seed is a platform-level kind: it declares initial data for an application —
// master rows, reference tables, role/grant rows, and demo records — in YAML
// instead of a bespoke script.
//
// Why it is a kind at all: `formspec seed` (cmd/formspec/seed.go) always
// accepted `kind: Seed` manifests, but the kind was never registered, so
// `formspec validate` rejected every seed file with `unknown kind "Seed"` and
// there was no JSON schema for editors to complete against. A file that the
// tool runs but the gate rejects is worse than no file: it teaches authors to
// ship unvalidated manifests.
//
// Seed is NOT a business Entity: it has no CRUD surface, no module ownership
// beyond grouping, and it participates in no state machine. Like kind: App and
// kind: Workspace it is a boot/CLI-time declaration. Records are inserted
// through the same EntityStore the API uses, so natural-key generation, field
// defaults, and validation all apply — a seed cannot create a record that the
// API would reject.
//
// Two markers extend a record beyond a plain payload, both because a value the
// author cannot know has to be filled in by the engine: `$ref` (a related
// record's generated ID) and `$asset` (a file field's canonical object key,
// after uploading the source file through the storage service). See SeedEntity.
const KindSeed Kind = "Seed"

// SeedSpec is the kind-specific body of a `kind: Seed` manifest.
type SeedSpec struct {
	// Entities declares records for one or more entities, grouped so a reader
	// can see "this block seeds ingredients, that block seeds menu items".
	Entities []SeedEntity `yaml:"entities" json:"entities"`
}

// SeedEntity declares the records to insert for a single entity.
type SeedEntity struct {
	// Entity is the entity name inside metadata.module (not module-qualified —
	// the module comes from the manifest itself, like every other kind).
	// @schema {example: "menu-item", description: "Entity name inside metadata.module"}
	Entity string `yaml:"entity" json:"entity"`
	// Records are raw entity payloads, shaped exactly like an API create body.
	//
	// Two seed-only markers may appear in a record. `$ref` resolves a relation
	// field by natural value (`{$ref: "module.entity:field=value"}`), because a
	// related record's generated ID cannot be written by hand. `$asset` uploads
	// a file field's source file through the storage service
	// (`{$asset: "menu/sate-ayam.jpg"}`, relative to `<module-dir>/assets/`) and
	// writes the resulting canonical object key — necessary because that key
	// embeds the record ID, which does not exist before insert
	// (docs/cli-tools/02-formspec-cli.md §6).
	//
	// Idempotency is by natural key: an existing record is not duplicated.
	// Fields that differ from the seed are updated (the run reports them), so a
	// corrected seed reaches an existing database — except write-only fields
	// (`masked: true`, e.g. `password`) and records whose `doc_status` is no
	// longer `draft`, which are left alone.
	// @schema {description: "Raw entity payloads (same shape as an API create body); supports `$ref` for relations and `$asset` for file fields"}
	Records []map[string]any `yaml:"records,omitempty" json:"records,omitempty"`
}

// ValidateSeedSpec checks the manifest-level shape of a Seed.
//
// It deliberately does NOT validate record payloads against the target entity:
// that requires the entity registry (cross-manifest), and the engine already
// rejects bad payloads at insert time (unknown fields, missing required,
// natural-key violations). Validating here would duplicate that logic and
// drift from it.
func ValidateSeedSpec(s *SeedSpec) error {
	if s == nil {
		return fmt.Errorf("seed: spec is required")
	}
	if len(s.Entities) == 0 {
		return fmt.Errorf("seed: spec.entities must declare at least one entity")
	}
	seen := map[string]bool{}
	for i, se := range s.Entities {
		if se.Entity == "" {
			return fmt.Errorf("seed: spec.entities[%d]: entity is required", i)
		}
		if seen[se.Entity] {
			// Two blocks for one entity would make insert order — and therefore
			// which records exist after a partial failure — depend on YAML
			// ordering in a way readers do not expect. One block per entity.
			return fmt.Errorf("seed: spec.entities[%d]: entity %q declared twice; merge the records into one block", i, se.Entity)
		}
		seen[se.Entity] = true
		// An entity with no records is a no-op that looks like work; require
		// either records or removal of the block.
		if len(se.Records) == 0 {
			return fmt.Errorf("seed: spec.entities[%d] (%s): records must not be empty", i, se.Entity)
		}
	}
	return nil
}
