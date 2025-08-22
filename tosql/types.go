package tosql

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

type SchemaTemplate struct {
	Entities []SchemaEntity
}

// entity
type SchemaEntity struct {
	DBType           db.DBType
	Name             string
	NameTitle        string
	PrimaryKeys      []string
	Fields           []SchemaField
	Indexes          []SchemaIndex
	Constraints      []SchemaConstraint
	SelectStatements []SchemaSelectStatement
}

func (e SchemaEntity) IsPrimaryKey(fieldIdentifier string) bool {
	return slices.Contains(e.PrimaryKeys, fieldIdentifier)
}

func (e SchemaEntity) PrimaryKeysIdentifiers() string {
	return strings.Join(e.PrimaryKeys, ", ")
}

func (e SchemaEntity) PrimaryKeysWhereClause() string {
	keys := []string{}
	for _, pk := range e.PrimaryKeys {
		// quotes already added to name
		keys = append(keys, fmt.Sprintf("%s = ?", pk))
	}
	return strings.Join(keys, " AND ")
}

func (e SchemaEntity) PrimaryKeysWhereClauseWithValues(values map[string]string) string {
	keys := []string{}
	for _, pk := range e.PrimaryKeys {
		if value, ok := values[pk]; ok {
			// quotes already added to name and values already escaped and quoted
			keys = append(keys, fmt.Sprintf("%s = %s", pk, value))
		}
	}
	return strings.Join(keys, " AND ")
}

func (e SchemaEntity) UpdateFields() string {
	fields := []string{}
	for _, f := range e.Fields {
		if !slices.Contains(e.PrimaryKeys, f.Name) {
			switch e.DBType {
			case db.MYSQLDBType:
				fields = append(fields, fmt.Sprintf("`%s` = ?", f.Name))
			case db.PGDBType:
				fields = append(fields, fmt.Sprintf(`"%s" = ?`, f.Name))
			}
		}
	}
	return strings.Join(fields, ", ")
}

func (e SchemaEntity) UpdateFieldsWithValues(values map[string]string) string {
	fields := []string{}
	for _, f := range e.Fields {
		if !slices.Contains(e.PrimaryKeys, f.Name) {
			if value, ok := values[f.Field.Uuid]; ok {
				switch e.DBType {
				case db.MYSQLDBType:
					fields = append(fields, fmt.Sprintf("`%s` = '%s'", f.Name, EscapeValue(value)))
				case db.PGDBType:
					fields = append(fields, fmt.Sprintf(`"%s" = '%s'`, f.Name, EscapeValue(value)))
				}
			}
		}
	}
	return strings.Join(fields, ", ")
}

// field
type SchemaField struct {
	Name      string
	NameTitle string
	Type      string
	Field     *nemgen.Field
	Null      string
	HasComma  bool
	Default   string
	Unique    string
}

func (f SchemaField) Postfix() string {
	res := []string{}
	if f.Null != "" {
		res = append(res, f.Null)
	}
	if f.Default != "" {
		res = append(res, f.Default)
	}
	return strings.Join(res, " ")
}

// index
type SchemaIndex struct {
	DBType     db.DBType
	Name       string
	FieldNames map[string]string
	Index      *nemgen.Index
	TypePrefix string
	Type       string
	TypeSort   int
	HasComma   bool
}

func (i SchemaIndex) FieldNamesIdentifiers() string {
	fields := i.Index.Fields
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Priority < fields[j].Priority
	})

	fieldsStr := []string{}
	for _, f := range fields {

		if i.DBType == db.MYSQLDBType {
			order := ""
			if f.Order == nemgen.IndexFieldOrder_INDEX_FIELD_ORDER_ASC {
				order = "ASC"
			} else if f.Order == nemgen.IndexFieldOrder_INDEX_FIELD_ORDER_DESC {
				order = "DESC"
			}
			if f.Length > 0 {
				fieldsStr = append(fieldsStr, fmt.Sprintf("`%s`(%d) %s", i.FieldNames[f.FieldUuid], f.Length, order))
			} else {
				fieldsStr = append(fieldsStr, fmt.Sprintf("`%s` %s", i.FieldNames[f.FieldUuid], order))
			}
		} else if i.DBType == db.PGDBType {
			if f.Length > 0 {
				fieldsStr = append(fieldsStr, fmt.Sprintf(`"%s"(%d)`, i.FieldNames[f.FieldUuid], f.Length))
			} else {
				fieldsStr = append(fieldsStr, fmt.Sprintf(`"%s"`, i.FieldNames[f.FieldUuid]))
			}
		}
	}

	return fmt.Sprintf("(%s)", strings.Join(fieldsStr, ", "))
}

// select
type SchemaSelectStatement struct {
	Name             string
	Identifier       string
	EntityIdentifier string
	Fields           []SchemaSelectStatementField
	CombinedIndexes  bool
	IsPrimary        bool
	TimeFields       []SchemaField
	SortSupported    bool
}

type SchemaSelectStatementField struct {
	Name   string
	Field  SchemaField
	IsLast bool
}

// contraints
type SchemaConstraint struct {
	DBType       db.DBType
	Name         string
	Relationship *nemgen.Relationship
	TableName    string
	FromFields   []SchemaField
	ToFields     []SchemaField
	HasComma     bool
}

func (sc SchemaConstraint) ForeignKeyFields() string {
	sort.Slice(sc.FromFields, func(i, j int) bool {
		return strings.Compare(sc.FromFields[i].Name, sc.FromFields[j].Name) < 1
	})
	fields := []string{}
	for _, f := range sc.FromFields {
		if sc.DBType == db.MYSQLDBType {
			fields = append(fields, fmt.Sprintf("`%s`", f.Name))
		} else if sc.DBType == db.PGDBType {
			fields = append(fields, fmt.Sprintf(`"%s"`, f.Name))
		}
	}

	return strings.Join(fields, ", ")
}

func (sc SchemaConstraint) ReferenceFields() string {
	sort.Slice(sc.ToFields, func(i, j int) bool {
		return strings.Compare(sc.ToFields[i].Name, sc.ToFields[j].Name) < 1
	})
	fields := []string{}
	for _, f := range sc.ToFields {
		if sc.DBType == db.MYSQLDBType {
			fields = append(fields, fmt.Sprintf("`%s`", f.Name))
		} else if sc.DBType == db.PGDBType {
			fields = append(fields, fmt.Sprintf(`"%s"`, f.Name))
		}
	}

	return strings.Join(fields, ", ")
}
