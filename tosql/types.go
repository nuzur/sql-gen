package tosql

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

// postgresTextSearchConfig is the text search configuration used when rendering
// a FULLTEXT index as a Postgres GIN index.
//
// 'simple' is deliberate: it lower-cases and splits on non-word characters but
// applies no stemming and strips no stop words, so it behaves the same for every
// language. A language-specific configuration such as 'english' gives better
// recall, but nem carries no language metadata on a field or index, so choosing
// one would silently impose English stemming on non-English content. Queries
// must use the same configuration to match this index.
const postgresTextSearchConfig = "simple"

type SchemaTemplate struct {
	Entities []SchemaEntity
}

// entity
type SchemaEntity struct {
	DBType           db.DBType
	ForGolang        bool
	Name             string
	NameTitle        string
	PrimaryKeys      []string
	Fields           []SchemaField
	Indexes          []SchemaIndex
	Constraints      []SchemaConstraint
	SelectStatements []SchemaSelectStatement
}

func (e SchemaEntity) NumOfNonePKFields() int {
	count := 0
	for _, f := range e.Fields {
		if !e.IsPrimaryKey(f.Field.Identifier) {
			count++
		}
	}
	return count
}

func (e SchemaEntity) IsPrimaryKey(fieldIdentifier string) bool {
	// e.PrimaryKeys holds quoted identifiers (`col` / "col") while callers pass
	// the raw field identifier, so both sides are normalized before comparing.
	return slices.ContainsFunc(e.PrimaryKeys, func(pk string) bool {
		return unquoteIdentifier(pk) == unquoteIdentifier(fieldIdentifier)
	})
}

func unquoteIdentifier(identifier string) string {
	return strings.Trim(identifier, "`\"")
}

func (e SchemaEntity) PrimaryKeysIdentifiers() string {
	return strings.Join(e.PrimaryKeys, ", ")
}

func (e SchemaEntity) PrimaryKeysWhereClause() string {
	return e.PrimaryKeysWhereClauseParam(e.ForGolang, false)
}

func (e SchemaEntity) PrimaryKeysWhereClauseForUpdate() string {
	return e.PrimaryKeysWhereClauseParam(e.ForGolang, true)
}

// PrimaryKeysWhereClauseParam renders the primary key WHERE clause. When update
// is true the clause trails a SET clause built by UpdateFields, so its
// placeholders continue that clause's numbering instead of restarting at $1.
func (e SchemaEntity) PrimaryKeysWhereClauseParam(forGolang bool, update bool) string {
	offset := 0
	if update {
		offset = e.UpdateFieldsParamCount(false, nil)
	}
	return e.PrimaryKeysWhereClauseParamWithOffset(forGolang, offset)
}

// PrimaryKeysWhereClauseParamWithOffset renders the primary key WHERE clause
// with positional placeholders starting at offset+1. Callers that build the SET
// clause from a subset of the entity's fields (data change requests) must pass
// the number of placeholders that clause actually emitted — see
// UpdateFieldsParamCount. Deriving the offset from anything else leaves the
// statement referencing parameters that are never bound, which postgres rejects
// with "could not determine data type of parameter $N".
func (e SchemaEntity) PrimaryKeysWhereClauseParamWithOffset(forGolang bool, offset int) string {
	keys := []string{}
	for _, pk := range e.PrimaryKeys {
		// quotes already added to name
		switch e.DBType {
		case db.MYSQLDBType:
			keys = append(keys, fmt.Sprintf("%s = ?", pk))
		case db.PGDBType:
			if forGolang {
				keys = append(keys, fmt.Sprintf("%s = $%d", pk, len(keys)+1+offset))
			} else {
				keys = append(keys, fmt.Sprintf("%s = ?", pk))
			}
		}
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
	return e.UpdateFieldsParam(e.ForGolang, false, nil)
}

func (e SchemaEntity) UpdateFieldsParam(forGolang bool, onlyWithValue bool, values map[string]string) string {
	fields := []string{}
	paramIndex := 0
	for _, entry := range e.updateFieldEntries(onlyWithValue, values) {
		if entry.isNull {
			switch e.DBType {
			case db.MYSQLDBType:
				fields = append(fields, fmt.Sprintf("`%s` = NULL", entry.name))
			case db.PGDBType:
				fields = append(fields, fmt.Sprintf(`"%s" = NULL`, entry.name))
			}
			continue
		}
		paramIndex++
		switch e.DBType {
		case db.MYSQLDBType:
			fields = append(fields, fmt.Sprintf("`%s` = ?", entry.name))
		case db.PGDBType:
			if forGolang {
				fields = append(fields, fmt.Sprintf(`"%s" = $%d`, entry.name, paramIndex))
			} else {
				fields = append(fields, fmt.Sprintf(`"%s" = ?`, entry.name))
			}
		}
	}
	return strings.Join(fields, ", ")
}

// UpdateFieldsParamCount is the number of bound parameters UpdateFieldsParam
// emits for the same inputs — columns rendered as a literal NULL don't count.
// This is the offset the trailing primary key WHERE clause must continue from.
func (e SchemaEntity) UpdateFieldsParamCount(onlyWithValue bool, values map[string]string) int {
	count := 0
	for _, entry := range e.updateFieldEntries(onlyWithValue, values) {
		if !entry.isNull {
			count++
		}
	}
	return count
}

// updateFieldEntry is a single column of a SET clause: either a bound parameter
// or a literal NULL.
type updateFieldEntry struct {
	name   string
	isNull bool
}

// updateFieldEntries walks the entity's non-key fields in the order the SET
// clause renders them. UpdateFieldsParam and UpdateFieldsParamCount both build
// on it so the rendered clause and its parameter count can never disagree.
func (e SchemaEntity) updateFieldEntries(onlyWithValue bool, values map[string]string) []updateFieldEntry {
	entries := []updateFieldEntry{}
	for _, f := range e.Fields {
		if f.Field.Key {
			continue
		}
		value, ok := values[f.Field.Uuid]
		if !ok && onlyWithValue {
			continue
		}
		// Only treat as NULL when the value was explicitly provided and is blank
		// for a column where '' is not a valid literal (see blankMeansNull). When
		// ok is false we're in generic template generation (onlyWithValue=false,
		// no values map) and must use a placeholder so the generated template is
		// reusable.
		entries = append(entries, updateFieldEntry{
			name:   f.Name,
			isNull: ok && blankMeansNull(f.Field) && value == "",
		})
	}
	return entries
}

func (e SchemaEntity) UpdateFieldsWithValues(values map[string]string) string {
	fields := []string{}
	for _, f := range e.Fields {
		if !f.Field.Key {
			if value, ok := values[f.Field.Uuid]; ok {
				if blankMeansNull(f.Field) && value == "" {
					switch e.DBType {
					case db.MYSQLDBType:
						fields = append(fields, fmt.Sprintf("`%s` = NULL", f.Name))
					case db.PGDBType:
						fields = append(fields, fmt.Sprintf(`"%s" = NULL`, f.Name))
					}
				} else {
					switch e.DBType {
					case db.MYSQLDBType:
						fields = append(fields, fmt.Sprintf("`%s` = '%s'", f.Name, EscapeValue(value)))
					case db.PGDBType:
						fields = append(fields, fmt.Sprintf(`"%s" = '%s'`, f.Name, EscapeValue(value)))
					}
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
			orderStr := ""
			if f.Order == nemgen.IndexFieldOrder_INDEX_FIELD_ORDER_DESC {
				orderStr = " DESC"
			}
			if f.Length > 0 {
				fieldsStr = append(fieldsStr, fmt.Sprintf("`%s`(%d)%s", i.FieldNames[f.FieldUuid], f.Length, orderStr))
			} else {
				if orderStr != "" {
					fieldsStr = append(fieldsStr, fmt.Sprintf("`%s` %s", i.FieldNames[f.FieldUuid], strings.TrimSpace(orderStr)))
				} else {
					fieldsStr = append(fieldsStr, fmt.Sprintf("`%s`", i.FieldNames[f.FieldUuid]))
				}
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

// FullTextExpression renders the indexable expression for a FULLTEXT index on
// Postgres, which has no FULLTEXT index type of its own. The equivalent is a GIN
// index over to_tsvector(...), so the column list becomes a single tsvector
// expression.
//
// Three details matter:
//
//   - The two-argument to_tsvector(regconfig, text) form is required. The
//     one-argument form resolves the search configuration from a GUC at call
//     time, making it STABLE rather than IMMUTABLE, and Postgres refuses to
//     index a non-IMMUTABLE expression.
//   - Every column is cast to text. A FULLTEXT index is only meaningful over
//     text, but nothing upstream restricts the declared field types, and
//     to_tsvector has no overload for e.g. integer. The cast is a no-op for
//     text columns.
//   - Each column is wrapped in coalesce. Concatenating a NULL yields NULL, so
//     a single NULL column would otherwise erase the whole tsvector for that
//     row and silently drop it from the index.
//
// Returns "" for any non-fulltext index or for a non-Postgres dialect, so the
// template can use a non-empty result as the signal to take the GIN path.
func (i SchemaIndex) FullTextExpression() string {
	if i.Type != "fulltext" || i.DBType != db.PGDBType {
		return ""
	}

	fields := i.Index.Fields
	sort.Slice(fields, func(a, b int) bool {
		return fields[a].Priority < fields[b].Priority
	})

	parts := []string{}
	for _, f := range fields {
		name, found := i.FieldNames[f.FieldUuid]
		if !found {
			continue
		}
		parts = append(parts, fmt.Sprintf(`coalesce("%s"::text, '')`, name))
	}

	if len(parts) == 0 {
		return ""
	}

	return fmt.Sprintf("to_tsvector('%s', %s)",
		postgresTextSearchConfig,
		strings.Join(parts, " || ' ' || "),
	)
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
