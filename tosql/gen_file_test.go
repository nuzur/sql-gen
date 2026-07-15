package tosql

import (
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
)

func constraintNamed(name, relationshipUUID string) SchemaConstraint {
	return SchemaConstraint{
		Name:         name,
		Relationship: &nemgen.Relationship{Uuid: relationshipUUID},
	}
}

func entityWithIndex(entityName, indexName string) SchemaEntity {
	return SchemaEntity{Name: entityName, Indexes: []SchemaIndex{{Name: indexName}}}
}

// TestDeduplicateIndexNames guards the Postgres index-naming churn: nem index
// names are unique only per entity (every table has a "created_at"/"status"
// index), but Postgres index names are unique per schema. A position-dependent
// counter renumbered the same physical index differently from run to run, so
// pg-schema-diff emitted a spurious rename → recreate → drop on every diff of an
// otherwise-unchanged schema. Dedup suffixes colliding names with the owning
// table name, which is stable and unique (index names are unique within a table).
func TestDeduplicateIndexNames(t *testing.T) {
	t.Run("unique names are left untouched", func(t *testing.T) {
		entities := []SchemaEntity{
			entityWithIndex("season_has_drag", "season_id"),
			entityWithIndex("season", "wiki_page"),
		}
		deduplicateIndexNames(entities)
		if got := entities[0].Indexes[0].Name; got != "season_id" {
			t.Fatalf("Name = %q, want unchanged %q", got, "season_id")
		}
		if got := entities[1].Indexes[0].Name; got != "wiki_page" {
			t.Fatalf("Name = %q, want unchanged %q", got, "wiki_page")
		}
	})

	t.Run("colliding names across entities are suffixed with the table name", func(t *testing.T) {
		entities := []SchemaEntity{
			entityWithIndex("drag_show", "created_at"),
			entityWithIndex("episode", "created_at"),
		}
		deduplicateIndexNames(entities)

		got := []string{entities[0].Indexes[0].Name, entities[1].Indexes[0].Name}
		want := []string{"created_at_drag_show", "created_at_episode"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Indexes[%d].Name = %q, want %q", i, got[i], want[i])
			}
		}
		if got[0] == got[1] {
			t.Fatalf("colliding indexes still share a name after dedup: %q", got[0])
		}
	})

	t.Run("distinct indexes that share a uuid still get unique names", func(t *testing.T) {
		// Legacy models can carry two different indexes with the same uuid; the
		// table-name suffix must still disambiguate them (the uuid could not).
		entities := []SchemaEntity{
			{Name: "single_key", Indexes: []SchemaIndex{{Name: "nuevo_indice", Index: &nemgen.Index{Uuid: "a525362d-0000-0000-0000-000000000000"}}}},
			{Name: "post", Indexes: []SchemaIndex{{Name: "nuevo_indice", Index: &nemgen.Index{Uuid: "a525362d-0000-0000-0000-000000000000"}}}},
		}
		deduplicateIndexNames(entities)
		if a, b := entities[0].Indexes[0].Name, entities[1].Indexes[0].Name; a == b {
			t.Fatalf("indexes sharing a uuid still collide after dedup: %q", a)
		}
	})

	t.Run("suffix is deterministic across repeated runs regardless of slice order", func(t *testing.T) {
		build := func(order []int) []SchemaEntity {
			base := []string{"drag_show", "episode", "season_has_drag"}
			entities := make([]SchemaEntity, len(order))
			for i, idx := range order {
				entities[i] = entityWithIndex(base[idx], "created_at")
			}
			return entities
		}

		byTable := func(entities []SchemaEntity) map[string]string {
			result := make(map[string]string)
			for _, e := range entities {
				result[e.Name] = e.Indexes[0].Name
			}
			return result
		}

		runA := build([]int{0, 1, 2})
		runB := build([]int{2, 0, 1})
		deduplicateIndexNames(runA)
		deduplicateIndexNames(runB)

		for table, name := range byTable(runA) {
			if other := byTable(runB)[table]; other != name {
				t.Fatalf("table %s index got name %q in one run and %q in another - not deterministic", table, name, other)
			}
		}
	})
}

// TestDeduplicateConstraintNames guards against the FK-constraint-naming bug where
// two relationships sharing an identifier (legacy data predating identifier
// uniqueness enforcement, e.g. two FKs from "episode" to "drag") produced two SQL
// constraints with the identical name - invalid DDL for both Postgres (unique per
// table) and MySQL (unique per database).
func TestDeduplicateConstraintNames(t *testing.T) {
	t.Run("unique names are left untouched", func(t *testing.T) {
		entities := []SchemaEntity{
			{Constraints: []SchemaConstraint{constraintNamed("episode_drag", "11111111-aaaa-aaaa-aaaa-aaaaaaaaaaaa")}},
			{Constraints: []SchemaConstraint{constraintNamed("season_drag", "22222222-bbbb-bbbb-bbbb-bbbbbbbbbbbb")}},
		}
		deduplicateConstraintNames(entities)
		if got := entities[0].Constraints[0].Name; got != "episode_drag" {
			t.Fatalf("Name = %q, want unchanged %q", got, "episode_drag")
		}
		if got := entities[1].Constraints[0].Name; got != "season_drag" {
			t.Fatalf("Name = %q, want unchanged %q", got, "season_drag")
		}
	})

	t.Run("colliding names are suffixed with their relationship uuid, both instances", func(t *testing.T) {
		entities := []SchemaEntity{
			{Constraints: []SchemaConstraint{
				constraintNamed("episode_drag", "11111111-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				constraintNamed("episode_drag", "22222222-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
			}},
		}
		deduplicateConstraintNames(entities)

		got := []string{entities[0].Constraints[0].Name, entities[0].Constraints[1].Name}
		want := []string{"episode_drag_11111111", "episode_drag_22222222"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Constraints[%d].Name = %q, want %q", i, got[i], want[i])
			}
		}
		if got[0] == got[1] {
			t.Fatalf("colliding constraints still share a name after dedup: %q", got[0])
		}
	})

	t.Run("suffix is deterministic across repeated runs regardless of slice order", func(t *testing.T) {
		build := func(order []int) []SchemaEntity {
			base := []SchemaConstraint{
				constraintNamed("episode_drag", "11111111-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				constraintNamed("episode_drag", "22222222-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
				constraintNamed("episode_drag", "33333333-cccc-cccc-cccc-cccccccccccc"),
			}
			reordered := make([]SchemaConstraint, len(order))
			for i, idx := range order {
				reordered[i] = base[idx]
			}
			return []SchemaEntity{{Constraints: reordered}}
		}

		byUUID := func(entities []SchemaEntity) map[string]string {
			result := make(map[string]string)
			for _, c := range entities[0].Constraints {
				result[c.Relationship.Uuid] = c.Name
			}
			return result
		}

		runA := build([]int{0, 1, 2})
		runB := build([]int{2, 0, 1})
		deduplicateConstraintNames(runA)
		deduplicateConstraintNames(runB)

		namesA := byUUID(runA)
		namesB := byUUID(runB)
		for uuid, name := range namesA {
			if namesB[uuid] != name {
				t.Fatalf("relationship %s got name %q in one run and %q in another - not deterministic", uuid, name, namesB[uuid])
			}
		}
	})
}
