package tosql

import (
	"fmt"

	nemgen "github.com/nuzur/nem/idl/gen"
)

// SortStandaloneEntities reorders pv.Entities so every standalone entity appears
// AFTER the entities it references via a foreign key. Emitting CREATE TABLE with
// inline FK constraints requires the referenced table to already exist, so a
// topological order is mandatory for the generated DDL to be replayable (e.g.
// when materializing a schema into a temp database for a diff).
//
// This is a proper topological sort (Kahn's algorithm) over the FK dependency
// graph, with a deterministic tie-break so the output is stable across runs.
// Foreign-key cycles can't be expressed with inline constraints; if one is
// present the remaining entities are appended in their existing order rather
// than dropped, so we still produce output (the DB will reject a true cycle,
// which is the correct signal). Non-standalone entities are preserved.
func SortStandaloneEntities(pv *nemgen.ProjectVersion) {
	standalone := make(map[string]*nemgen.Entity)
	inputOrder := []*nemgen.Entity{} // standalone entities, in their current order
	for _, e := range pv.Entities {
		if e.Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			standalone[e.Uuid] = e
			inputOrder = append(inputOrder, e)
		}
	}

	// deps[e] = set of entity UUIDs that e references and that must be created
	// first. Self-references are ignored (a table can reference its own column).
	deps := make(map[string]map[string]bool, len(standalone))
	for uuid := range standalone {
		deps[uuid] = map[string]bool{}
	}
	for _, r := range pv.Relationships {
		if r.From == nil || r.To == nil {
			continue
		}
		if r.From.Type != nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY ||
			r.To.Type != nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY {
			continue
		}
		if r.From.TypeConfig == nil || r.From.TypeConfig.Entity == nil ||
			r.To.TypeConfig == nil || r.To.TypeConfig.Entity == nil {
			continue
		}
		from := r.From.TypeConfig.Entity.EntityUuid // the entity that holds the FK
		to := r.To.TypeConfig.Entity.EntityUuid     // the referenced entity
		if from == "" || to == "" || from == to {
			continue
		}
		if _, ok := standalone[from]; !ok {
			continue
		}
		if _, ok := standalone[to]; !ok {
			continue
		}
		deps[from][to] = true
	}

	// Kahn's algorithm with a FIFO frontier. pending[e] counts e's not-yet-emitted
	// targets; dependents[t] are the entities waiting on t. Seed the queue with the
	// dependency-free entities in input order, then emit breadth-first — so all
	// "root" tables come first, then the tables that reference them, etc. Input
	// order is the tie-break, keeping the output stable across runs.
	pending := make(map[string]int, len(standalone))
	dependents := make(map[string][]string, len(standalone))
	for uuid, targets := range deps {
		pending[uuid] = len(targets)
		for target := range targets {
			dependents[target] = append(dependents[target], uuid)
		}
	}

	queue := []*nemgen.Entity{}
	for _, e := range inputOrder {
		if pending[e.Uuid] == 0 {
			queue = append(queue, e)
		}
	}

	sortedEntities := make([]*nemgen.Entity, 0, len(pv.Entities))
	emitted := make(map[string]bool, len(standalone))
	for len(queue) > 0 {
		e := queue[0]
		queue = queue[1:]
		if emitted[e.Uuid] {
			continue
		}
		sortedEntities = append(sortedEntities, e)
		emitted[e.Uuid] = true
		for _, dep := range dependents[e.Uuid] {
			pending[dep]--
			if pending[dep] == 0 {
				queue = append(queue, standalone[dep])
			}
		}
	}

	// FK cycle (or unresolved dep): append any leftovers in input order so we
	// don't drop entities. A genuine cycle can't be inline-emitted anyway.
	if len(emitted) < len(standalone) {
		for _, e := range inputOrder {
			if !emitted[e.Uuid] {
				sortedEntities = append(sortedEntities, e)
				emitted[e.Uuid] = true
			}
		}
	}

	// Preserve non-standalone entities (order unchanged), appended after.
	for _, e := range pv.Entities {
		if e.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			sortedEntities = append(sortedEntities, e)
		}
	}

	pv.Entities = sortedEntities
}

func PrintEntities(message string, entities []*nemgen.Entity) {
	fmt.Printf("%s: ", message)
	for _, e := range entities {
		fmt.Printf(" %s", e.Identifier)
	}
	fmt.Printf("\n")
}
