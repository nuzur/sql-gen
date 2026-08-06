package tosql

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

func ResolveSelectStatements(e *nemgen.Entity, dbType db.DBType) []SchemaSelectStatement {
	selects := []SchemaSelectStatement{}
	if e.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
		return selects
	}

	// Query names are unique per entity for the whole file, and the primary
	// select is minted first, so it has to be in the set before any index-derived
	// name is tested against it. go-code-gen's resolver seeds the same way; without
	// it a hand-modeled UNIQUE/INDEX over exactly the key column(s) resolves to the
	// same "<Entity>By<Key>" name and emits it a second time into
	// select_indexed_simple.sql, where `sqlc generate` aborts on the duplicate query
	// name and takes the whole generated app with it. EnsureUniqueFieldIndexes
	// already refuses to synthesize an index over a key field for this reason — this
	// closes the remaining, hand-modeled case.
	seenNames := map[string]bool{}

	// add select by primary key(s)
	primaryKeys := EntityPrimaryKeys(e)
	if len(primaryKeys) > 0 {
		primaryIdentifiers := []string{}
		for _, pk := range primaryKeys {
			primaryIdentifiers = append(primaryIdentifiers, ToCamelCase(pk.Identifier))
		}
		finalPKName := strings.Join(primaryIdentifiers, "And")

		nameByID := fmt.Sprintf("%sBy%s", ToCamelCase(e.Identifier), finalPKName)
		selects = append(selects, SchemaSelectStatement{
			Name:             nameByID,
			Identifier:       strcase.ToSnake(nameByID),
			EntityIdentifier: e.Identifier,
			Fields:           mapFieldsToSelectFields(primaryKeys, dbType),
			IsPrimary:        true,
			SortSupported:    false,
		})
		seenNames[nameByID] = true
	}

	// if there are not indexes return
	if e.TypeConfig == nil || e.TypeConfig.Standalone == nil || len(e.TypeConfig.Standalone.Indexes) == 0 {
		return selects
	}

	fieldMap := make(map[string]*nemgen.Field)
	for _, f := range e.Fields {
		fieldMap[f.Uuid] = f
	}

	// filter out indexes that are not datetime
	indexes := e.TypeConfig.Standalone.Indexes
	indexIds := []string{}
	indexMap := make(map[string]*nemgen.Index)
	timeFields := []SchemaField{}
	for _, i := range indexes {
		if len(i.Fields) == 1 {
			field, found := fieldMap[i.Fields[0].FieldUuid]
			if found {
				ft := field.Type
				if ft == nemgen.FieldType_FIELD_TYPE_DATETIME || ft == nemgen.FieldType_FIELD_TYPE_DATE {
					// A time field only earns an ORDER BY variant if the column is
					// really indexed: the index has to be one the schema emits as an
					// INDEX/UNIQUE (every other branch below already requires it, this
					// one silently did not), and the column has to be one the mapper
					// emits at all — an ORDER BY over a column that never made it into
					// CREATE TABLE is a query sqlc cannot parse.
					if i.Type == nemgen.IndexType_INDEX_TYPE_INDEX || i.Type == nemgen.IndexType_INDEX_TYPE_UNIQUE {
						if usableIndexMember(field) {
							mappedField := mapField(field, dbType)
							if mappedField != nil {
								timeFields = append(timeFields, *mappedField)
							}
						}
					}
				} else {
					if i.Type == nemgen.IndexType_INDEX_TYPE_INDEX || i.Type == nemgen.IndexType_INDEX_TYPE_UNIQUE {
						if len(i.Fields) > 0 {
							indexIds = append(indexIds, i.Uuid)
							indexMap[i.Uuid] = i
						}
					}
				}
			} else {
				if i.Type == nemgen.IndexType_INDEX_TYPE_INDEX || i.Type == nemgen.IndexType_INDEX_TYPE_UNIQUE {
					if len(i.Fields) > 0 {
						indexIds = append(indexIds, i.Uuid)
						indexMap[i.Uuid] = i
					}
				}
			}
		} else {
			if i.Type == nemgen.IndexType_INDEX_TYPE_INDEX || i.Type == nemgen.IndexType_INDEX_TYPE_UNIQUE {
				if len(i.Fields) > 0 {
					indexIds = append(indexIds, i.Uuid)
					indexMap[i.Uuid] = i
				}
			}
		}
	}

	// combine all indexes
	//
	// Combinations is the power set of the index set: 2^N - 1 subsets. For an
	// entity with many indexes this explodes (20 indexes ≈ 1M subsets), and each
	// subset allocates a SchemaSelectStatement — enough to OOM the process. Above
	// maxPowerSetIndexes we fall back to one select per individual index, which
	// is linear. Entities at or below the threshold keep the full behavior.
	var combinations [][]string
	if len(indexIds) > maxPowerSetIndexes {
		combinations = singleIndexSubsets(indexIds)
	} else {
		combinations = Combinations(indexIds)
	}

	// A real index must always claim its own query name; a combination may only
	// take a name no index wanted.
	//
	// Names are deduped first-emission-wins below, and Combinations enumerates by
	// bit pattern, so the pair {0,1} (subsetBits=3) is visited before the singleton
	// {2} (subsetBits=4). Because datetime/date members are dropped from the name,
	// a pair of composite indexes can union to exactly the field set of some later
	// single index, steal its name, and — being tagged CombinedIndexes — render only
	// into select_indexed_combined.sql, which go-code-gen throws away. The real
	// index's query then exists nowhere while go-code-gen's own resolver still
	// emits a module wrapper calling it, and the generated app fails to compile.
	// Emitting every singleton first makes that unreachable.
	//
	// Combinations never yields an empty subset (subsetBits starts at 1), so this is
	// a total partition and nothing is lost. singleIndexSubsets — the
	// >maxPowerSetIndexes fallback — is all singletons, so this is an identity on it.
	combinations = singlesFirst(combinations)

	// Two combinations over the same field set produce byte-identical SQL under two
	// different names (names accrete in index traversal order, WHERE fields are
	// sorted alphabetically), so combinations are additionally deduped by field set.
	// Singles are never dropped by field set — go-code-gen mints one wrapper per
	// index and every one of them must find its query — but they do register their
	// field set so a later combination cannot re-emit it under a permuted name. The
	// primary select is deliberately not registered: it is rendered from a different
	// template branch and an index over the key columns is a different statement.
	seenFieldSets := map[string]bool{}

	for _, combination := range combinations {
		name := fmt.Sprintf("%sBy", ToCamelCase(e.Identifier))
		fields := map[string]SchemaSelectStatementField{}
		first := true

		// for each combination of indexes
		for _, indexUUID := range combination {

			// get the fields of the index
			indexFields := indexMap[indexUUID].Fields
			for _, indexField := range indexFields {
				_, exists := fields[indexField.FieldUuid]
				if !exists {
					field := fieldMap[indexField.FieldUuid]
					if !usableIndexMember(field) {
						continue
					}
					if field.Type == nemgen.FieldType_FIELD_TYPE_DATETIME || field.Type == nemgen.FieldType_FIELD_TYPE_DATE {
						// datetime/date fields inside composite indexes are excluded from
						// the WHERE clause, matching go-code-gen's module select resolver
						// (which names its fetch methods without them)
						continue
					}
					mappedField := mapField(field, dbType)
					if mappedField != nil {
						fields[indexField.FieldUuid] = SchemaSelectStatementField{
							Name:   field.Identifier,
							Field:  *mappedField,
							IsLast: false,
						}
						if first {
							first = false
							name = fmt.Sprintf("%s%s", name, ToCamelCase(field.Identifier))
						} else {
							name = fmt.Sprintf("%sAnd%s", name, ToCamelCase(field.Identifier))
						}
					}
				}
			}

		}

		finalFields := []SchemaSelectStatementField{}
		for _, f := range fields {
			finalFields = append(finalFields, f)
		}
		sort.Slice(finalFields, func(i, j int) bool {
			return strings.Compare(finalFields[i].Name, finalFields[j].Name) < 0
		})

		// an index whose fields were all excluded (datetime/date, or a column the
		// mapper does not emit) contributes no WHERE clause; emitting it would
		// produce an invalid, unnamed select. Two indexes can also collapse to the
		// same field set after the exclusion — only the first emission survives
		// (sqlc rejects duplicate query names).
		//
		// The field-set test is the same rule one level deeper: PostByTitleAndSlug
		// and PostBySlugAndTitle are distinct names but the same query. It applies to
		// combinations only, per seenFieldSets above.
		combined := len(combination) > 1
		fieldSetKey := selectFieldSetKey(finalFields)
		if len(finalFields) == 0 || seenNames[name] || (combined && seenFieldSets[fieldSetKey]) {
			continue
		}
		seenNames[name] = true
		seenFieldSets[fieldSetKey] = true
		finalFields[len(finalFields)-1].IsLast = true

		sortSupported := false
		if len(timeFields) > 0 {
			sortSupported = true
		}

		selects = append(selects, SchemaSelectStatement{
			Name:             name,
			Identifier:       strcase.ToSnake(name),
			EntityIdentifier: e.Identifier,
			Fields:           finalFields,
			TimeFields:       timeFields,
			SortSupported:    sortSupported,
			CombinedIndexes:  combined,
		})
	}

	return selects
}

// maxPowerSetIndexes is the largest index count for which ResolveSelectStatements
// computes the full power set of index combinations. 2^8 = 256 subsets is a
// safe upper bound on the work/memory per entity; above it we degrade to one
// select per index to avoid a combinatorial memory blowup.
const maxPowerSetIndexes = 8

// usableIndexMember mirrors the column-emission filter the mapper applies
// (MapEntityToTypes only maps ACTIVE fields, and mapField drops a field with no
// identifier or an invalid type): a field the mapper will not emit as a column
// must not appear in a select's name or its WHERE clause, because the column it
// names is not in the table and sqlc cannot parse the query against it.
//
// LOCKSTEP: go-code-gen/core/repo.ResolveSelectStatements is an independent
// implementation of "which selects does an entity get" and mints one module
// wrapper per index; the two must agree on the exact set of names, or the
// generated app calls a query that does not exist. Its usableIndexMember twin
// applies this identical predicate at the identical point in its member loop.
// The paired TestResolveSelectStatements_InactiveIndexMemberDropped tests in
// both repos are the executable contract, and go-code-gen's
// verifySelectContract fails generation outright if the name sets ever drift.
func usableIndexMember(f *nemgen.Field) bool {
	return f != nil &&
		f.Identifier != "" &&
		f.Type != nemgen.FieldType_FIELD_TYPE_INVALID &&
		f.Status == nemgen.FieldStatus_FIELD_STATUS_ACTIVE
}

// selectFieldSetKey identifies a select by the set of columns it filters on.
// finalFields is already sorted by name, so the key is order-independent by
// construction; NUL is the separator because it cannot occur in an identifier and
// therefore cannot make two different field sets collide.
func selectFieldSetKey(fields []SchemaSelectStatementField) string {
	names := make([]string, 0, len(fields))
	for _, f := range fields {
		names = append(names, f.Name)
	}
	return strings.Join(names, "\x00")
}

// singlesFirst reorders subsets so that every single-index subset comes before
// every multi-index one, preserving the relative order within each group. See the
// call site for why the order is load-bearing.
func singlesFirst(combinations [][]string) [][]string {
	ordered := make([][]string, 0, len(combinations))
	for _, c := range combinations {
		if len(c) == 1 {
			ordered = append(ordered, c)
		}
	}
	for _, c := range combinations {
		if len(c) > 1 {
			ordered = append(ordered, c)
		}
	}
	return ordered
}

// singleIndexSubsets returns one single-element subset per index, matching the
// shape Combinations returns so the caller's loop is unchanged. Used as the
// bounded fallback for entities with many indexes.
func singleIndexSubsets(set []string) [][]string {
	subsets := make([][]string, 0, len(set))
	for _, s := range set {
		subsets = append(subsets, []string{s})
	}
	return subsets
}

func Combinations(set []string) (subsets [][]string) {
	length := uint(len(set))

	// Go through all possible combinations of objects
	// from 1 (only first object in subset) to 2^length (all objects in subset)
	for subsetBits := 1; subsetBits < (1 << length); subsetBits++ {
		var subset []string

		for object := uint(0); object < length; object++ {
			// checks if object is contained in subset
			// by checking if bit 'object' is set in subsetBits
			if (subsetBits>>object)&1 == 1 {
				// add object to subset
				subset = append(subset, set[object])
			}
		}
		// add subset to subsets
		subsets = append(subsets, subset)
	}
	return subsets
}
