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

	// add select by primary key(s)
	primaryKeys := EntityPrimaryKeys(e)
	if len(primaryKeys) > 0 {
		primaryIdentifiers := []string{}
		for _, pk := range primaryKeys {
			primaryIdentifiers = append(primaryIdentifiers, strcase.ToCamel(pk.Identifier))
		}
		finalPKName := strings.Join(primaryIdentifiers, "And")

		nameByID := fmt.Sprintf("%sBy%s", strcase.ToCamel(e.Identifier), finalPKName)
		selects = append(selects, SchemaSelectStatement{
			Name:             nameByID,
			Identifier:       strcase.ToSnake(nameByID),
			EntityIdentifier: e.Identifier,
			Fields:           mapFieldsToSelectFields(primaryKeys, dbType),
			IsPrimary:        true,
			SortSupported:    false,
		})
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
					mappedField := mapField(field, dbType)
					if mappedField != nil {
						timeFields = append(timeFields, *mappedField)
					}
				} else {
					if i.Type == nemgen.IndexType_INDEX_TYPE_INDEX {
						indexIds = append(indexIds, i.Uuid)
						indexMap[i.Uuid] = i
					}
				}
			}
		} else {
			if i.Type == nemgen.IndexType_INDEX_TYPE_INDEX {
				indexIds = append(indexIds, i.Uuid)
				indexMap[i.Uuid] = i
			}
		}
	}

	// combine all indexes
	combinations := Combinations(indexIds)
	for _, combination := range combinations {
		name := fmt.Sprintf("%sBy", strcase.ToCamel(e.Identifier))
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
					mappedField := mapField(field, dbType)
					if mappedField != nil {
						fields[indexField.FieldUuid] = SchemaSelectStatementField{
							Name:   field.Identifier,
							Field:  *mappedField,
							IsLast: false,
						}

						if first {
							first = false
							name = fmt.Sprintf("%s%s", name, strcase.ToCamel(field.Identifier))
						} else {
							name = fmt.Sprintf("%sAnd%s", name, strcase.ToCamel(field.Identifier))
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

		if len(finalFields) > 0 {
			finalFields[len(finalFields)-1].IsLast = true
		}

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
			CombinedIndexes:  len(combination) > 1,
		})
	}

	return selects
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
