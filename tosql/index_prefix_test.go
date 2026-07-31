package tosql

import (
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/stretchr/testify/assert"
)

// MySQL refuses to index a TEXT/BLOB column without a prefix length (error
// 1170), and the rejected INDEX clause sits inside CREATE TABLE — so the table
// is not created either and a first deploy produces no schema at all. Postgres
// accepts the same index and has no prefix syntax whatsoever: `"col"(10)` parses
// as a function call there, which either errors or silently builds a btree over
// a constant.

const (
	ixEntityUUID = "bbbbbbbb-0000-0000-0000-000000000001"
	ixIDUUID     = "bbbbbbbb-0000-0000-0000-000000000002"
	ixBodyUUID   = "bbbbbbbb-0000-0000-0000-000000000003"
)

// indexedEntity builds a table with an id plus one field of the given type, and
// one index over that field.
func indexedEntity(fieldType nemgen.FieldType, typeConfig *nemgen.FieldTypeConfig, indexType nemgen.IndexType, length int64) *nemgen.Entity {
	if typeConfig == nil {
		typeConfig = &nemgen.FieldTypeConfig{}
	}
	return &nemgen.Entity{
		Uuid:       ixEntityUUID,
		Identifier: "tasting_note",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Status:     nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
		Fields: []*nemgen.Field{
			{
				Uuid:       ixIDUUID,
				Identifier: "id",
				Type:       nemgen.FieldType_FIELD_TYPE_UUID,
				Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
				Key:        true,
				Required:   true,
			},
			{
				Uuid:       ixBodyUUID,
				Identifier: "body",
				Type:       fieldType,
				TypeConfig: typeConfig,
				Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
			},
		},
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: []*nemgen.Index{
					{
						Uuid:       "bbbbbbbb-0000-0000-0000-000000000004",
						Identifier: "idx_body",
						Type:       indexType,
						Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
						Fields: []*nemgen.IndexField{
							{FieldUuid: ixBodyUUID, Length: length},
						},
					},
				},
			},
		},
	}
}

func binaryFileConfig(fieldType nemgen.FieldType) *nemgen.FieldTypeConfig {
	c := &nemgen.FieldTypeFileConfig{
		StorageType: nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY,
	}
	switch fieldType {
	case nemgen.FieldType_FIELD_TYPE_IMAGE:
		return &nemgen.FieldTypeConfig{Image: c}
	case nemgen.FieldType_FIELD_TYPE_AUDIO:
		return &nemgen.FieldTypeConfig{Audio: c}
	case nemgen.FieldType_FIELD_TYPE_VIDEO:
		return &nemgen.FieldTypeConfig{Video: c}
	default:
		return &nemgen.FieldTypeConfig{File: c}
	}
}

func TestMySQLIndexPrefixesTextAndBlobColumns(t *testing.T) {
	for _, tc := range []struct {
		name       string
		fieldType  nemgen.FieldType
		typeConfig *nemgen.FieldTypeConfig
		want       string
	}{
		{"text", nemgen.FieldType_FIELD_TYPE_TEXT, nil, "INDEX `idx_body` (`body`(255))"},
		{"richtext", nemgen.FieldType_FIELD_TYPE_RICHTEXT, nil, "INDEX `idx_body` (`body`(255))"},
		{"code", nemgen.FieldType_FIELD_TYPE_CODE, nil, "INDEX `idx_body` (`body`(255))"},
		{"markdown", nemgen.FieldType_FIELD_TYPE_MARKDOWN, nil, "INDEX `idx_body` (`body`(255))"},
		{"binary file", nemgen.FieldType_FIELD_TYPE_FILE, binaryFileConfig(nemgen.FieldType_FIELD_TYPE_FILE), "INDEX `idx_body` (`body`(255))"},
		{"binary image", nemgen.FieldType_FIELD_TYPE_IMAGE, binaryFileConfig(nemgen.FieldType_FIELD_TYPE_IMAGE), "INDEX `idx_body` (`body`(255))"},
		{"binary audio", nemgen.FieldType_FIELD_TYPE_AUDIO, binaryFileConfig(nemgen.FieldType_FIELD_TYPE_AUDIO), "INDEX `idx_body` (`body`(255))"},
		{"binary video", nemgen.FieldType_FIELD_TYPE_VIDEO, binaryFileConfig(nemgen.FieldType_FIELD_TYPE_VIDEO), "INDEX `idx_body` (`body`(255))"},
		// An object-store file field is a VARCHAR of urls, which indexes whole.
		{"object store file", nemgen.FieldType_FIELD_TYPE_FILE, &nemgen.FieldTypeConfig{File: &nemgen.FieldTypeFileConfig{}}, "INDEX `idx_body` (`body`)"},
		{"varchar", nemgen.FieldType_FIELD_TYPE_VARCHAR, nil, "INDEX `idx_body` (`body`)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := indexedEntity(tc.fieldType, tc.typeConfig, nemgen.IndexType_INDEX_TYPE_INDEX, 0)
			assert.Contains(t, renderCreate(t, e, db.MYSQLDBType), tc.want)
		})
	}
}

func TestMySQLIndexHonorsExplicitLength(t *testing.T) {
	e := indexedEntity(nemgen.FieldType_FIELD_TYPE_TEXT, nil, nemgen.IndexType_INDEX_TYPE_INDEX, 64)
	assert.Contains(t, renderCreate(t, e, db.MYSQLDBType), "INDEX `idx_body` (`body`(64))")

	// A length on a column that does not need one is still the caller's choice.
	e = indexedEntity(nemgen.FieldType_FIELD_TYPE_VARCHAR, nil, nemgen.IndexType_INDEX_TYPE_INDEX, 20)
	assert.Contains(t, renderCreate(t, e, db.MYSQLDBType), "INDEX `idx_body` (`body`(20))")
}

// A FULLTEXT index covers whole columns. MySQL accepts a prefix on one and then
// silently drops it — SUB_PART comes back NULL and SHOW CREATE TABLE shows the
// bare column — so an emitted prefix is a permanent disagreement between the
// generated DDL and the live schema, i.e. an index the diff rebuilds forever.
func TestMySQLFullTextIndexTakesNoPrefix(t *testing.T) {
	for _, length := range []int64{0, 255} {
		e := indexedEntity(nemgen.FieldType_FIELD_TYPE_TEXT, nil, nemgen.IndexType_INDEX_TYPE_FULLTEXT, length)
		got := renderCreate(t, e, db.MYSQLDBType)
		assert.Contains(t, got, "FULLTEXT INDEX `idx_body` (`body`)")
		assert.NotContains(t, got, "(`body`(")
	}
}

func TestMySQLUniqueIndexPrefixesTextColumn(t *testing.T) {
	e := indexedEntity(nemgen.FieldType_FIELD_TYPE_TEXT, nil, nemgen.IndexType_INDEX_TYPE_UNIQUE, 0)
	assert.Contains(t, renderCreate(t, e, db.MYSQLDBType), "UNIQUE INDEX `idx_body` (`body`(255))")
}

// Postgres has no prefix-index syntax at all, so a Length set by the model (or
// carried over from a MySQL introspection) must be dropped rather than rendered.
func TestPostgresIgnoresIndexLength(t *testing.T) {
	for _, length := range []int64{0, 10, 255} {
		e := indexedEntity(nemgen.FieldType_FIELD_TYPE_TEXT, nil, nemgen.IndexType_INDEX_TYPE_INDEX, length)
		got := renderCreate(t, e, db.PGDBType)
		assert.Contains(t, got, `CREATE INDEX "idx_body" ON "tasting_note" ("body");`)
		assert.NotContains(t, got, `"body"(`)
	}
}
