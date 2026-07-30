package tosql

import (
	"bytes"
	"testing"
	"text/template"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Postgres has no FULLTEXT index type. Its equivalent is a GIN index over
// to_tsvector(...), so a FULLTEXT declaration has to be rendered as an
// expression index rather than a column list.
//
// It previously fell through to a plain `CREATE INDEX ... (col)`, i.e. a btree,
// which is wrong in two ways:
//
//  1. A btree over text cannot serve full-text search, and cannot serve
//     LIKE '%term%' either, so the index was unusable by any text query while
//     still costing writes and storage.
//  2. A btree index entry cannot exceed ~2704 bytes (a third of an 8KB page), so
//     it turns a missed optimisation into a *write failure*:
//     `index row size 3216 exceeds btree version 4 maximum 2704`. The whole row
//     fails to insert, not just the index entry.
//
// (2) is easy to miss because Postgres compresses index tuples: repeat('a',4000)
// fits, and only incompressible content trips the limit. Real prose compresses
// roughly by half, putting the practical threshold around 5-6KB of text rather
// than 2704 bytes, which is well within a realistic richtext body.
//
// The MySQL side was always correct and must stay that way -- this is a
// dialect-specific render, not a change to what FULLTEXT means.

const ftFieldUUID = "11111111-1111-1111-1111-111111111111"
const ftFieldUUID2 = "22222222-2222-2222-2222-222222222222"
const ftIDUUID = "33333333-3333-3333-3333-333333333333"

// fullTextEntity builds a standalone entity with an id plus one or two text
// fields, and a single FULLTEXT index covering the text fields.
func fullTextEntity(fieldUUIDs ...string) *nemgen.Entity {
	fields := []*nemgen.Field{
		{
			Uuid:       ftIDUUID,
			Identifier: "id",
			Type:       nemgen.FieldType_FIELD_TYPE_UUID,
			Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
			Key:        true,
		},
	}
	idxFields := []*nemgen.IndexField{}
	for n, u := range fieldUUIDs {
		identifier := "field_notes"
		if u == ftFieldUUID2 {
			identifier = "summary"
		}
		fields = append(fields, &nemgen.Field{
			Uuid:       u,
			Identifier: identifier,
			Type:       nemgen.FieldType_FIELD_TYPE_RICHTEXT,
			Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
		})
		idxFields = append(idxFields, &nemgen.IndexField{
			FieldUuid: u,
			Priority:  int64(n),
		})
	}

	return &nemgen.Entity{
		Uuid:       "44444444-4444-4444-4444-444444444444",
		Identifier: "recording",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Status:     nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
		Fields:     fields,
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: []*nemgen.Index{
					{
						Uuid:       "55555555-5555-5555-5555-555555555555",
						Identifier: "ft_recording_field_notes",
						Type:       nemgen.IndexType_INDEX_TYPE_FULLTEXT,
						Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
						Fields:     idxFields,
					},
				},
			},
		},
	}
}

// renderCreate runs the dialect's create template over a single entity, which is
// the only place the index DDL is actually spelled.
func renderCreate(t *testing.T, e *nemgen.Entity, dbType db.DBType) string {
	t.Helper()

	pv := &nemgen.ProjectVersion{Entities: []*nemgen.Entity{e}}
	se, err := MapEntityToSchemaEntity(e, pv, dbType, false)
	require.NoError(t, err)

	name := "create_postgres"
	if dbType == db.MYSQLDBType {
		name = "create_mysql"
	}
	tmplBytes, err := templates.ReadFile("templates/" + name + ".tmpl")
	require.NoError(t, err)

	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"inc": func(i int) int { return i + 1 },
	}).Parse(string(tmplBytes))
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, tmpl.Execute(&buf, SchemaTemplate{Entities: []SchemaEntity{se}}))
	return buf.String()
}

func TestFullTextIndexIsGINOnPostgres(t *testing.T) {
	got := renderCreate(t, fullTextEntity(ftFieldUUID), db.PGDBType)

	assert.Contains(t, got,
		`CREATE INDEX "ft_recording_field_notes" ON "recording" USING gin (to_tsvector('simple', coalesce("field_notes"::text, '')));`,
		"FULLTEXT on Postgres must be a GIN index over to_tsvector")

	// The regression: a bare column list is a btree, which blocks large writes.
	assert.NotContains(t, got,
		`ON "recording" ("field_notes")`,
		"FULLTEXT must not degrade to a plain btree over the column")
	assert.NotContains(t, got, "btree")
}

func TestFullTextIndexStaysFullTextOnMySQL(t *testing.T) {
	got := renderCreate(t, fullTextEntity(ftFieldUUID), db.MYSQLDBType)

	assert.Contains(t, got, "FULLTEXT INDEX `ft_recording_field_notes` (`field_notes`)",
		"MySQL has a real FULLTEXT index type and must keep using it")
	// to_tsvector is Postgres-only and would be a syntax error here.
	assert.NotContains(t, got, "to_tsvector")
	assert.NotContains(t, got, "gin")
}

func TestFullTextExpressionConcatenatesMultipleColumns(t *testing.T) {
	got := renderCreate(t, fullTextEntity(ftFieldUUID, ftFieldUUID2), db.PGDBType)

	// Each column is coalesced: concatenating a NULL yields NULL, which would
	// erase the tsvector for the whole row and drop it from the index.
	assert.Contains(t, got,
		`USING gin (to_tsvector('simple', coalesce("field_notes"::text, '') || ' ' || coalesce("summary"::text, '')));`)
}

func TestFullTextExpressionOnlyAppliesToPostgresFullText(t *testing.T) {
	pv := &nemgen.ProjectVersion{Entities: []*nemgen.Entity{fullTextEntity(ftFieldUUID)}}
	e := fullTextEntity(ftFieldUUID)

	pgEntity, err := MapEntityToSchemaEntity(e, pv, db.PGDBType, false)
	require.NoError(t, err)
	myEntity, err := MapEntityToSchemaEntity(e, pv, db.MYSQLDBType, false)
	require.NoError(t, err)

	findIndex := func(entity SchemaEntity, indexType string) (SchemaIndex, bool) {
		for _, i := range entity.Indexes {
			if i.Type == indexType {
				return i, true
			}
		}
		return SchemaIndex{}, false
	}

	pgFT, found := findIndex(pgEntity, "fulltext")
	require.True(t, found, "expected a fulltext index on the postgres mapping")
	assert.NotEmpty(t, pgFT.FullTextExpression())

	// Same index, MySQL dialect: no expression, because MySQL renders TypePrefix.
	myFT, found := findIndex(myEntity, "fulltext")
	require.True(t, found, "expected a fulltext index on the mysql mapping")
	assert.Empty(t, myFT.FullTextExpression())

	// A primary index is never a fulltext expression, on either dialect.
	if pk, found := findIndex(pgEntity, "primary"); found {
		assert.Empty(t, pk.FullTextExpression())
	}
}
