package tosql

import (
	"sort"

	nemgen "github.com/nuzur/nem/idl/gen"
)

func SortStandaloneEntities(pv *nemgen.ProjectVersion) {
	relsFromEntity := make(map[string][]*nemgen.Relationship)
	relsToEntity := make(map[string][]*nemgen.Relationship)
	relCount := make(map[string]int)
	for _, r := range pv.Relationships {
		if r.From.Type == nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY {
			relsFromEntity[r.From.TypeConfig.Entity.EntityUuid] = append(relsFromEntity[r.From.Uuid], r)
			relCount[r.From.TypeConfig.Entity.EntityUuid]++
		}

		if r.To.Type == nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY {
			relsToEntity[r.To.TypeConfig.Entity.EntityUuid] = append(relsToEntity[r.To.Uuid], r)
			relCount[r.To.TypeConfig.Entity.EntityUuid]++
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
			_, relToEntityFound := relsToEntity[e.Uuid]
			if !relsFromEntityFound && !relToEntityFound {
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
				_, relToEntityFound := relsToEntity[e.Uuid]
				if !relToEntityFound && relsFromEntityFound {
					allFound := true
					for _, r := range relsFromEntity {
						if _, addedEntitiesFound := addedEntities[r.To.TypeConfig.Entity.EntityUuid]; addedEntitiesFound {
							if !addedEntitiesFound {
								allFound = false
								break
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

	sortedRemaining := make([]string, 0, len(relCount))
	for key := range relCount {
		if _, found := addedEntities[key]; !found {
			sortedRemaining = append(sortedRemaining, key)
		}
	}
	sort.Slice(sortedRemaining, func(i, j int) bool { return relCount[sortedRemaining[i]] < relCount[sortedRemaining[j]] })

	for len(sortedEntities) < len(pv.Entities) {
		for _, e := range sortedRemaining {
			relsFromEntity, relsFromEntityFound := relsFromEntity[e]
			_, relToEntityFound := relsToEntity[e]
			if !relToEntityFound && relsFromEntityFound {
				allFound := true
				for _, r := range relsFromEntity {
					if _, addedEntitiesFound := addedEntities[r.To.TypeConfig.Entity.EntityUuid]; addedEntitiesFound {
						if !addedEntitiesFound {
							allFound = false
							break
						}
					}
				}
				if allFound {
					sortedEntities = append(sortedEntities, entitiesMap[e])
					addedEntities[e] = true
				}
			}
		}
	}

	pv.Entities = sortedEntities
}
