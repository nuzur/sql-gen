package tosql

import (
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

func MapEntityToTypes(e *nemgen.Entity, projectVersion *nemgen.ProjectVersion, dbType db.DBType) ([]SchemaField, []SchemaIndex, []SchemaConstraint) {
	fields := []SchemaField{}
	indexes := []SchemaIndex{}
	constraints := []SchemaConstraint{}

	// if not standalone return
	if e.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
		return fields, indexes, constraints
	}

	// field identifiers map
	fieldIdentifers := make(map[string]string)
	for _, f := range e.Fields {
		if f.Status == nemgen.FieldStatus_FIELD_STATUS_ACTIVE {
			fieldIdentifers[f.Uuid] = f.Identifier
			ft := mapField(f, dbType)
			if ft != nil {
				fields = append(fields, *ft)
			}
		}
	}

	if e.TypeConfig != nil && e.TypeConfig.Standalone != nil {
		// map indexes
		indexes = mapIndexes(e, dbType, fieldIdentifers)

		// map relationships to constraints
		constraints = mapRelationships(e, projectVersion, dbType)
	}

	if len(indexes) > 0 {
		for i := 0; i < len(indexes)-1; i++ {
			indexes[i].HasComma = true
		}
	}

	if len(constraints) > 0 {
		for i := 0; i < len(constraints)-1; i++ {
			constraints[i].HasComma = true
		}
	}

	if len(fields) > 0 {
		for i := 0; i < len(fields)-1; i++ {
			fields[i].HasComma = true
		}
	}

	return fields, indexes, constraints
}

func mapField(f *nemgen.Field, dbType db.DBType) *SchemaField {
	if f == nil || f.Identifier == "" || f.Type == nemgen.FieldType_FIELD_TYPE_INVALID {
		return nil
	}

	fieldType := ""
	if dbType == db.MYSQLDBType {
		fieldType = FieldTypeToMYSQL(f)
	} else if dbType == db.PGDBType {
		fieldType = FieldTypeToPG(f)
	}

	notNull := ""
	if f.Required {
		notNull = "NOT NULL"
	}

	ft := SchemaField{
		Name:      f.Identifier,
		NameTitle: strcase.ToCamel(f.Identifier),
		Type:      fieldType,
		Field:     f,
		Null:      notNull,
	}

	if f.Unique {
		ft.Unique = "UNIQUE"
	}

	switch f.Type {
	// TODO add option to disable this in field config
	case nemgen.FieldType_FIELD_TYPE_DATETIME:
		ft.Default = "DEFAULT CURRENT_TIMESTAMP"
	}
	return &ft
}

func mapFieldsToSelectFields(fields []*nemgen.Field, dbType db.DBType) []SchemaSelectStatementField {
	res := []SchemaSelectStatementField{}
	for _, f := range fields {
		sf := mapField(f, dbType)
		if sf == nil {
			continue
		}
		nf := SchemaSelectStatementField{
			Name:   f.Identifier,
			Field:  *sf,
			IsLast: false,
		}
		res = append(res, nf)
	}

	sort.Slice(res, func(i, j int) bool {
		return strings.Compare(res[i].Name, res[j].Name) < 0
	})

	if len(res) > 0 {
		res[len(res)-1].IsLast = true
	}

	return res

}

func mapIndexes(e *nemgen.Entity, dbType db.DBType, fieldIdentifers map[string]string) []SchemaIndex {
	indexes := []SchemaIndex{}
	for _, i := range e.TypeConfig.Standalone.Indexes {
		if i.Status == nemgen.IndexStatus_INDEX_STATUS_ACTIVE {
			fieldNames := make(map[string]string)
			for _, fi := range i.Fields {
				identifier, found := fieldIdentifers[fi.FieldUuid]
				if found {
					fieldNames[fi.FieldUuid] = identifier
				}
			}

			if len(fieldNames) == 0 {
				continue
			}

			indexTypePrefix := ""
			if i.Type == nemgen.IndexType_INDEX_TYPE_UNIQUE {
				indexTypePrefix = "UNIQUE "
			}
			if i.Type == nemgen.IndexType_INDEX_TYPE_FULLTEXT {
				indexTypePrefix = "FULLTEXT "
			}

			indexType := ""
			indexTypeSort := 0

			switch i.Type {
			case nemgen.IndexType_INDEX_TYPE_UNIQUE:
				indexType = "unique"
				indexTypeSort = 2
			case nemgen.IndexType_INDEX_TYPE_PRIMARY:
				indexType = "primary"
				indexTypeSort = 0
			case nemgen.IndexType_INDEX_TYPE_INDEX:
				indexType = "index"
				indexTypeSort = 1
			case nemgen.IndexType_INDEX_TYPE_FULLTEXT:
				indexType = "fulltext"
				indexTypeSort = 3
			}

			indexes = append(indexes, SchemaIndex{
				DBType:     dbType,
				Name:       i.Identifier,
				Index:      i,
				FieldNames: fieldNames,
				Type:       indexType,
				TypeSort:   indexTypeSort,
				TypePrefix: indexTypePrefix,
			})

			sort.Slice(indexes, func(i, j int) bool {
				return indexes[i].TypeSort < indexes[j].TypeSort
			})
		}
	}
	return indexes
}

func mapRelationships(e *nemgen.Entity, projectVersion *nemgen.ProjectVersion, dbType db.DBType) []SchemaConstraint {
	res := []SchemaConstraint{}

	entityIdentifiers := make(map[string]string)
	entityMap := make(map[string]*nemgen.Entity)
	for _, e := range projectVersion.Entities {
		entityIdentifiers[e.Uuid] = e.Identifier
		entityMap[e.Uuid] = e
	}

	for _, relationship := range projectVersion.Relationships {
		if relationship.To != nil &&
			relationship.To.TypeConfig != nil &&
			relationship.To.TypeConfig.Entity != nil &&
			relationship.From != nil &&
			relationship.From.TypeConfig != nil &&
			relationship.From.TypeConfig.Entity != nil &&
			relationship.UseForeignKey {

			toEntity := relationship.To.TypeConfig.Entity
			fromEntity := relationship.From.TypeConfig.Entity

			if entityMap[toEntity.EntityUuid].Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE &&
				entityMap[fromEntity.EntityUuid].Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE {
				if fromEntity.EntityUuid == e.Uuid {
					toFieldUuids := relationship.To.TypeConfig.Entity.FieldUuids
					fromFieldUuids := relationship.From.TypeConfig.Entity.FieldUuids
					toFieldIdentifers := mapFieldIdentifiers(entityMap[toEntity.EntityUuid])
					fromFieldIdentifers := mapFieldIdentifiers(entityMap[fromEntity.EntityUuid])
					if len(toFieldUuids) > 0 {
						toFields := []SchemaField{}
						for _, fu := range toFieldUuids {
							toFields = append(toFields, SchemaField{
								Name:      toFieldIdentifers[fu],
								NameTitle: strcase.ToCamel(toFieldIdentifers[fu]),
							})
						}

						fromFields := []SchemaField{}
						for _, fu := range fromFieldUuids {
							fromFields = append(fromFields, SchemaField{
								Name:      fromFieldIdentifers[fu],
								NameTitle: strcase.ToCamel(fromFieldIdentifers[fu]),
							})
						}
						res = append(res, SchemaConstraint{
							DBType:       dbType,
							Name:         relationship.Identifier,
							Relationship: relationship,
							TableName:    entityIdentifiers[toEntity.EntityUuid],
							ToFields:     toFields,
							FromFields:   fromFields,
						})
					}
				}
			}
		}
	}

	return res
}

func mapFieldIdentifiers(e *nemgen.Entity) map[string]string {
	fieldIdentifers := make(map[string]string)
	for _, f := range e.Fields {
		if f.Status == nemgen.FieldStatus_FIELD_STATUS_ACTIVE {
			fieldIdentifers[f.Uuid] = f.Identifier
		}
	}
	return fieldIdentifers
}
