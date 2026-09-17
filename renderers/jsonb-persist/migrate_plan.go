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
		alterDDL, added, err := r.diffExistingTable(ctx, ti, em.EntitySpec)
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
	if len(allowed) == 0 {
		return plan, nil
	}

	ddl, err := r.buildChangeDDL(ctx, ti, &em.EntitySpec, old, desired, allowed)
	if err != nil {
		return plan, fmt.Errorf("plan migrations: build %s: %w", ti.TableName, err)
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
