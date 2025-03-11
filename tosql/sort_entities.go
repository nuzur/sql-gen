package tosql

import (
	"fmt"
	"sort"

	nemgen "github.com/nuzur/nem/idl/gen"
)

func SortStandaloneEntities(pv *nemgen.ProjectVersion) {
	relsFromEntity := make(map[string][]*nemgen.Relationship)
	relsToEntity := make(map[string][]*nemgen.Relationship)
	relCount := make(map[string]int)
	for _, r := range pv.Relationships {
		if r.From.Type == nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY {
			fromEntity := r.From.TypeConfig.Entity.EntityUuid
			relsFromEntity[fromEntity] = append(relsFromEntity[fromEntity], r)
			relCount[fromEntity]++
		}

		if r.To.Type == nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY {
			toEntity := r.To.TypeConfig.Entity.EntityUuid
			relsToEntity[toEntity] = append(relsToEntity[toEntity], r)
			relCount[toEntity]++
		}
	}

	sortedEntities := []*nemgen.Entity{}
	addedEntities := make(map[string]bool)
	entitiesMap := make(map[string]*nemgen.Entity)

	// first add the entities with no relationships
	for _, e := range pv.Entities {
		if e.Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			entitiesMap[e.Uuid] = e
			_, relsFromEntityFound := relsFromEntity[e.Uuid]
			if !relsFromEntityFound {
				sortedEntities = append(sortedEntities, e)
				addedEntities[e.Uuid] = true
			}
		}
	}

	// then add the entities with relationships to the already added entities
	for _, e := range pv.Entities {
		_, addedEntitiesFound := addedEntities[e.Uuid]
		if !addedEntitiesFound {
			if e.Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE {
				relsFromEntity, relsFromEntityFound := relsFromEntity[e.Uuid]
				if relsFromEntityFound {
					allFound := true
					for _, r := range relsFromEntity {
						if r.To != nil && r.To.TypeConfig != nil && r.To.TypeConfig.Entity != nil {
							if _, addedEntitiesFound := addedEntities[r.To.TypeConfig.Entity.EntityUuid]; addedEntitiesFound {
								if !addedEntitiesFound {
									allFound = false
									break
								}
							}
						}
					}
					if allFound {
						sortedEntities = append(sortedEntities, e)
						addedEntities[e.Uuid] = true
					}
				}
			}
		}
	}

	if len(sortedEntities) == len(pv.Entities) {
		pv.Entities = sortedEntities
		return
	}

	// add the remaining entities sorted by max number of relationships
	sortedRemaining := make([]string, 0, len(relCount))
	for key := range relCount {
		if _, found := addedEntities[key]; !found {
			sortedRemaining = append(sortedRemaining, key)
		}
	}
	sort.Slice(sortedRemaining, func(i, j int) bool { return relCount[sortedRemaining[i]] < relCount[sortedRemaining[j]] })

	for _, e := range sortedRemaining {
		relsFromEntity, relsFromEntityFound := relsFromEntity[e]
		if relsFromEntityFound {
			allFound := true
			for _, r := range relsFromEntity {
				if r.To != nil && r.To.TypeConfig != nil && r.To.TypeConfig.Entity != nil {
					if _, addedEntitiesFound := addedEntities[r.To.TypeConfig.Entity.EntityUuid]; addedEntitiesFound {
						if !addedEntitiesFound {
							allFound = false
							break
						}
					}
				}
			}
			entity, entityFound := entitiesMap[e]
			if allFound && entityFound {
				sortedEntities = append(sortedEntities, entity)
				addedEntities[e] = true
			}
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
