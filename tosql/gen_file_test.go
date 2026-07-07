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
