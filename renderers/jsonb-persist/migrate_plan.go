package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/primadi/formspec/pkg/spec"
)

// createDDLFor is the full statement set for a brand-new entity: the table, its
// child tables, its indexes, and any declared raw_ddl. A new table makes every
// one of those additive, so none of them passes the gate.
func createDDLFor(ti *TableInfo, entity *spec.EntitySpec, driver DriverType) string {
	parts := []string{ti.CreateTableSQL}
	for _, ct := range ti.ChildTables {
		parts = append(parts, ct.CreateTableSQL)
	}
	parts = append(parts, ti.CreateIndexSQL...)
	if entity != nil && entity.Persist != nil {
		for i := range entity.Persist.RawDDL {
			if stmt, ok := entity.Persist.RawDDL[i].DDLForDialect(string(driver)); ok {
				parts = append(parts, stmt)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// planEntityChange rates what changed for one entity whose table already exists.
//
// When no snapshot was ever recorded the plan bootstraps instead of refusing:
// a database created before snapshots existed has no earlier shape to have
// changed away from, so a refusal there would block a deployment over a change
// nobody can see and nobody can declare.
func (r *MigrationRunner) planEntityChange(ctx context.Context, em EntityMigration, ti *TableInfo, desc, checksum string, desired *EntitySnapshot, decls Declarations) (MigrationPlan, error) {
	module, entity := em.Metadata.Module, em.Metadata.Name
	plan := MigrationPlan{
		Result:   DDLResult{TableInfo: ti, Checksum: checksum, Description: desc},
		Snapshot: desired,
		Module:   module,
		Entity:   entity,
	}

	old, _, found, err := r.LoadSnapshot(ctx, r.db, module, entity)
	if err != nil {
		return plan, fmt.Errorf("plan migrations: %w", err)
	}
	if !found {
		plan.Bootstrap = true
		alterDDL, added, err := r.diffExistingTable(ctx, ti, em.EntitySpec, nil)
		if err != nil {
			return plan, fmt.Errorf("plan migrations: diff %s: %w", ti.TableName, err)
		}
		if added > 0 {
			plan.Result.DDL = alterDDL
		}
		return plan, nil
	}

	changes := DiffShapes(desc, old, desired, decls)
	changes, err = r.measureChanges(ctx, ti, &em.EntitySpec, changes)
	if err != nil {
		return plan, fmt.Errorf("plan migrations: %w", err)
	}
	plan.Changes = changes

	// Only allowed changes contribute statements. A refused change must not
	// leave a half-prepared migration behind: the caller decides whether to
	// refuse the whole run, and the DDL has to match that decision.
	allowed := make([]Change, 0, len(changes))
	for _, c := range changes {
		if !c.Required() {
			allowed = append(allowed, c)
		}
	}

	// Storage can differ from BOTH the manifest and the recorded snapshot: the
	// snapshot records what was *intended* when the manifest was applied, not
	// what the database actually holds. An index that was never rebuilt when its
	// definition changed (kafe TODO 3.9), or a derived column that exists as a
	// plain column because it predates the generated form (kafe TODO 3.11), is
	// invisible to a snapshot-vs-manifest diff — `DiffShapes` sees nothing, no
	// DDL is built, and `migrate plan` answers "nothing to do" while the
	// database keeps enforcing the old rule.
	//
	// Reconciling storage is the same call in both cases, and it must happen
	// exactly once: `buildChangeDDL` already runs the reconciliation as its last
	// step (it has to, so the DDL it emits sees the end state), so running it
	// again here would emit the same `ALTER TABLE ... DROP COLUMN` twice and the
	// apply would fail.
	var ddl string
	if len(allowed) > 0 {
		ddl, err = r.buildChangeDDL(ctx, ti, &em.EntitySpec, old, desired, allowed)
		if err != nil {
			return plan, fmt.Errorf("plan migrations: build %s: %w", ti.TableName, err)
		}
	} else {
		ddl, _, err = r.diffExistingTable(ctx, ti, em.EntitySpec, nil)
		if err != nil {
			return plan, fmt.Errorf("plan migrations: reconcile %s: %w", ti.TableName, err)
		}
	}
	if ddl == "" {
		return plan, nil
	}
	plan.Result.DDL = ddl
	return plan, nil
}

// planForgotten finds entities that have a recorded snapshot but no manifest.
//
// Two outcomes, and the difference is the whole point: if the table is still
// there, the change is refused — a manifest never drops a table, and the blast
// radius is every row of that entity. If the table is gone as well, the operator
// already did both steps by hand, so the stale snapshot is dropped rather than
// refusing the same deployment forever with no way to satisfy it.
//
// The forget itself is deferred to apply: `formspec diff` is documented as a pure
// dry run and must stay one.
func (r *MigrationRunner) planForgotten(ctx context.Context, entities []EntityMigration) ([]MigrationPlan, error) {
	snaps, err := r.AllSnapshots(ctx, r.db)
	if err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return nil, nil
	}

	declared := make(map[string]bool, len(entities))
	for _, em := range entities {
		if em.EntitySpec.ExtendStorage != nil {
			continue
		}
		declared[em.Metadata.Module+"/"+em.Metadata.Name] = true
	}

	var plans []MigrationPlan
	for key, row := range snaps {
		if declared[key] {
			continue
		}
		// Framework-owned modules (formspec.core) are registered by the server
		// at runtime, never by a user manifest. Their absence from the caller's
		// entity list says nothing about a manifest being removed, so treating
		// them as a forgotten entity would refuse every run against a database
		// the server had migrated (kafe TODO 3.10).
		if r.frameworkModules[row.Module] {
			continue
		}
		desc := "entity:" + key

		exists, err := r.db.HasTable(ctx, row.Shape.Schema, row.Shape.Table)
		if err != nil {
			return nil, fmt.Errorf("plan migrations: check table %s: %w", row.Shape.Table, err)
		}
		if !exists {
			plans = append(plans, MigrationPlan{
				Result:     DDLResult{Checksum: row.Checksum, Description: desc},
				Module:     row.Module,
				Entity:     row.Entity,
				ForgetOnly: true,
			})
			continue
		}

		plans = append(plans, MigrationPlan{
			Result: DDLResult{Checksum: row.Checksum, Description: desc},
			Changes: []Change{{
				Class:  ClassNever,
				Kind:   ChangeTableRemoved,
				Entity: desc,
				Name:   row.Shape.Table,
				Detail: "the manifests no longer declare this entity, but its table still exists",
				Remedy: "back up the table, drop it by hand, then run apply again to forget the snapshot",
			}},
		})
	}
	return plans, nil
}
