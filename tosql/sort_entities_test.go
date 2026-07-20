package tosql

import (
	"fmt"
	"strings"
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
)

// buildFKPV returns a project version whose standalone entities form a fan-out
// FK graph: several roots, each with several entities depending on it. That
// shape is what exposed the ordering instability — the order in which an
// emitted entity releases its dependents decides the final order.
func buildFKPV(roots, childrenPerRoot int) *nemgen.ProjectVersion {
	entities := []*nemgen.Entity{}
	relationships := []*nemgen.Relationship{}
	for r := 0; r < roots; r++ {
		rootUUID := fmt.Sprintf("root-%d", r)
		entities = append(entities, &nemgen.Entity{
			Uuid:       rootUUID,
			Identifier: fmt.Sprintf("root_%d", r),
			Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		})
		for c := 0; c < childrenPerRoot; c++ {
			childUUID := fmt.Sprintf("child-%d-%d", r, c)
			entities = append(entities, &nemgen.Entity{
				Uuid:       childUUID,
				Identifier: fmt.Sprintf("child_%d_%d", r, c),
				Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
			})
			relationships = append(relationships, fkRelationship(childUUID, rootUUID))
		}
	}
	return &nemgen.ProjectVersion{Entities: entities, Relationships: relationships}
}

// fkRelationship models "from holds an FK pointing at to".
func fkRelationship(from, to string) *nemgen.Relationship {
	node := func(uuid string) *nemgen.RelationshipNode {
		return &nemgen.RelationshipNode{
			Type: nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY,
			TypeConfig: &nemgen.RelationshipNodeTypeConfig{
				Entity: &nemgen.RelationshipNodeTypeEntityConfig{EntityUuid: uuid},
			},
		}
	}
	return &nemgen.Relationship{From: node(from), To: node(to)}
}

func identifiers(pv *nemgen.ProjectVersion) string {
	names := make([]string, 0, len(pv.Entities))
	for _, e := range pv.Entities {
		names = append(names, e.Identifier)
	}
	return strings.Join(names, ",")
}

// The same input must always sort to the same output. Go randomizes map
// iteration, so a sort that walks its dependency maps produces a different
// order on nearly every run — and therefore reorders the generated DDL.
func TestSortStandaloneEntities_Deterministic(t *testing.T) {
	var want string
	for i := 0; i < 50; i++ {
		pv := buildFKPV(4, 4)
		SortStandaloneEntities(pv)
		got := identifiers(pv)
		if i == 0 {
			want = got
			continue
		}
		if got != want {
			t.Fatalf("run %d produced a different order:\n got: %s\nwant: %s", i, got, want)
		}
	}
}

// Determinism is worthless if the order is wrong: every referenced entity must
// still be emitted before the entity holding the FK.
func TestSortStandaloneEntities_DependenciesFirst(t *testing.T) {
	pv := buildFKPV(4, 4)
	SortStandaloneEntities(pv)

	position := map[string]int{}
	for i, e := range pv.Entities {
		position[e.Uuid] = i
	}
	for _, r := range pv.Relationships {
		from := r.From.TypeConfig.Entity.EntityUuid
		to := r.To.TypeConfig.Entity.EntityUuid
		if position[to] >= position[from] {
			t.Errorf("%s (pos %d) references %s (pos %d): referenced entity must come first",
				from, position[from], to, position[to])
		}
	}
}

// Entities that aren't standalone are preserved (appended after), and nothing
// is dropped along the way.
func TestSortStandaloneEntities_PreservesNonStandalone(t *testing.T) {
	pv := buildFKPV(2, 2)
	pv.Entities = append(pv.Entities, &nemgen.Entity{
		Uuid:       "dependent-1",
		Identifier: "dependent_1",
		Type:       nemgen.EntityType_ENTITY_TYPE_DEPENDENT,
	})
	before := len(pv.Entities)

	SortStandaloneEntities(pv)

	if len(pv.Entities) != before {
		t.Fatalf("got %d entities after sort, want %d", len(pv.Entities), before)
	}
	last := pv.Entities[len(pv.Entities)-1]
	if last.Identifier != "dependent_1" {
		t.Errorf("non-standalone entity should be appended last, got %q", last.Identifier)
	}
}

// A FK cycle can't be topologically ordered; the entities involved must still
// be emitted rather than silently dropped.
func TestSortStandaloneEntities_CycleKeepsAllEntities(t *testing.T) {
	pv := &nemgen.ProjectVersion{
		Entities: []*nemgen.Entity{
			{Uuid: "a", Identifier: "a", Type: nemgen.EntityType_ENTITY_TYPE_STANDALONE},
			{Uuid: "b", Identifier: "b", Type: nemgen.EntityType_ENTITY_TYPE_STANDALONE},
		},
		Relationships: []*nemgen.Relationship{
			fkRelationship("a", "b"),
			fkRelationship("b", "a"),
		},
	}

	SortStandaloneEntities(pv)

	if len(pv.Entities) != 2 {
		t.Fatalf("got %d entities, want 2 (cycle must not drop entities)", len(pv.Entities))
	}
}
