package tosql

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gofrs/uuid"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A field's `unique: true` used to be metadata the database never saw: it set a
// SchemaField.Unique nothing rendered, so the column accepted duplicates and no
// fetch-by-that-column query existed. EnsureUniqueFieldIndexes desugars it into
// the single-field UNIQUE index the modeler meant — but only when nothing already
// enforces the same rule, because a second, overlapping declaration of one
// constraint is what makes a schema diff churn forever.

const (
	ufEntityUUID = "aaaa1111-0000-0000-0000-000000000001"
	ufIDUUID     = "aaaa1111-0000-0000-0000-000000000002"
	ufEmailUUID  = "aaaa1111-0000-0000-0000-000000000003"
	ufNameUUID   = "aaaa1111-0000-0000-0000-000000000004"
)

// uniqueFieldEntity is a standalone "member" table with a key field flagged
// unique (the redundant flag real models carry on uuid primary keys), a
// unique-flagged "email", and a plain "name". The caller supplies the indexes.
func uniqueFieldEntity(indexes ...*nemgen.Index) *nemgen.Entity {
	e := &nemgen.Entity{
		Uuid:       ufEntityUUID,
		Identifier: "member",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Status:     nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
		Fields: []*nemgen.Field{
			{
				Uuid:       ufIDUUID,
				Identifier: "uuid",
				Type:       nemgen.FieldType_FIELD_TYPE_UUID,
				Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
				Key:        true,
				Required:   true,
				Unique:     true,
			},
			{
				Uuid:       ufEmailUUID,
				Identifier: "email",
				Type:       nemgen.FieldType_FIELD_TYPE_EMAIL,
				TypeConfig: &nemgen.FieldTypeConfig{},
				Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
				Unique:     true,
			},
			{
				Uuid:       ufNameUUID,
				Identifier: "name",
				Type:       nemgen.FieldType_FIELD_TYPE_VARCHAR,
				TypeConfig: &nemgen.FieldTypeConfig{},
				Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
			},
		},
	}
	if len(indexes) > 0 {
		e.TypeConfig = &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{Indexes: indexes},
		}
	}
	return e
}

func index(identifier string, indexType nemgen.IndexType, status nemgen.IndexStatus, fieldUUIDs ...string) *nemgen.Index {
	fields := make([]*nemgen.IndexField, 0, len(fieldUUIDs))
	for i, fu := range fieldUUIDs {
		fields = append(fields, &nemgen.IndexField{FieldUuid: fu, Priority: int64(i)})
	}
	return &nemgen.Index{
		Uuid:       "index-" + identifier,
		Identifier: identifier,
		Type:       indexType,
		Status:     status,
		Fields:     fields,
	}
}

func ensure(e *nemgen.Entity) []*nemgen.Index {
	EnsureUniqueFieldIndexes(&nemgen.ProjectVersion{Entities: []*nemgen.Entity{e}})
	return e.GetTypeConfig().GetStandalone().GetIndexes()
}

// synthesized returns the indexes this feature added, i.e. those the input did
// not already carry.
func synthesized(before []*nemgen.Index, after []*nemgen.Index) []*nemgen.Index {
	return after[len(before):]
}

// Which existing index shapes already enforce single-column uniqueness — and
// which only look like they do.
func TestEnsureUniqueFieldIndexes_Suppression(t *testing.T) {
	for _, tc := range []struct {
		name        string
		indexes     []*nemgen.Index
		wantEmail   bool // an index is synthesized for the unique-flagged email
		explanation string
	}{
		{
			name:        "no indexes at all",
			indexes:     nil,
			wantEmail:   true,
			explanation: "nothing enforces the flag",
		},
		{
			name:        "active single field unique",
			indexes:     []*nemgen.Index{index("uk_email", nemgen.IndexType_INDEX_TYPE_UNIQUE, nemgen.IndexStatus_INDEX_STATUS_ACTIVE, ufEmailUUID)},
			wantEmail:   false,
			explanation: "the rule is already enforced exactly",
		},
		{
			name:        "active single field primary",
			indexes:     []*nemgen.Index{index("pk_email", nemgen.IndexType_INDEX_TYPE_PRIMARY, nemgen.IndexStatus_INDEX_STATUS_ACTIVE, ufEmailUUID)},
			wantEmail:   false,
			explanation: "a primary key enforces uniqueness too",
		},
		{
			name:        "plain single field index",
			indexes:     []*nemgen.Index{index("idx_email", nemgen.IndexType_INDEX_TYPE_INDEX, nemgen.IndexStatus_INDEX_STATUS_ACTIVE, ufEmailUUID)},
			wantEmail:   true,
			explanation: "a lookup index enforces no uniqueness",
		},
		{
			name:        "fulltext single field index",
			indexes:     []*nemgen.Index{index("ft_email", nemgen.IndexType_INDEX_TYPE_FULLTEXT, nemgen.IndexStatus_INDEX_STATUS_ACTIVE, ufEmailUUID)},
			wantEmail:   true,
			explanation: "a fulltext index enforces no uniqueness",
		},
		{
			name:        "composite unique containing the field",
			indexes:     []*nemgen.Index{index("uk_email_name", nemgen.IndexType_INDEX_TYPE_UNIQUE, nemgen.IndexStatus_INDEX_STATUS_ACTIVE, ufEmailUUID, ufNameUUID)},
			wantEmail:   true,
			explanation: "(email, name) unique still permits duplicate emails",
		},
		{
			name:        "inactive single field unique",
			indexes:     []*nemgen.Index{index("uk_email", nemgen.IndexType_INDEX_TYPE_UNIQUE, nemgen.IndexStatus_INDEX_STATUS_INVALID, ufEmailUUID)},
			wantEmail:   true,
			explanation: "an inactive index is never emitted, so it enforces nothing",
		},
		{
			name:        "disabled single field unique",
			indexes:     []*nemgen.Index{index("uk_email", nemgen.IndexType_INDEX_TYPE_UNIQUE, nemgen.IndexStatus_INDEX_STATUS_DISABLED, ufEmailUUID)},
			wantEmail:   true,
			explanation: "a disabled index is never emitted either",
		},
		{
			name:        "unique on another field",
			indexes:     []*nemgen.Index{index("uk_name", nemgen.IndexType_INDEX_TYPE_UNIQUE, nemgen.IndexStatus_INDEX_STATUS_ACTIVE, ufNameUUID)},
			wantEmail:   true,
			explanation: "a unique on a different column is unrelated",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := uniqueFieldEntity(tc.indexes...)
			before := len(tc.indexes)
			after := ensure(e)

			added := after[before:]
			if !tc.wantEmail {
				assert.Empty(t, added, tc.explanation)
				return
			}
			require.Len(t, added, 1, tc.explanation)
			assert.Equal(t, "uq_member_email", added[0].Identifier)
			assert.Equal(t, nemgen.IndexType_INDEX_TYPE_UNIQUE, added[0].Type)
			assert.Equal(t, nemgen.IndexStatus_INDEX_STATUS_ACTIVE, added[0].Status)
			require.Len(t, added[0].Fields, 1)
			assert.Equal(t, ufEmailUUID, added[0].Fields[0].FieldUuid)
			assert.Equal(t, int64(0), added[0].Fields[0].Priority)
			assert.Equal(t, nemgen.IndexFieldOrder_INDEX_FIELD_ORDER_ASC, added[0].Fields[0].Order)
		})
	}
}

// The key field carries `unique: true` in most real models. The primary key
// already enforces it, and synthesizing an index there would also make the select
// resolver emit a second "MemberByUUID" fetch next to the primary one.
func TestEnsureUniqueFieldIndexes_SkipsKeyField(t *testing.T) {
	e := uniqueFieldEntity()
	for _, idx := range ensure(e) {
		assert.NotEqual(t, ufIDUUID, idx.Fields[0].FieldUuid, "no index may be synthesized for the key field")
	}
}

// MySQL cannot index a JSON column with or without a prefix length, so a
// synthesized index over one is DDL that fails inside CREATE TABLE and takes the
// whole table with it. The mapping is evaluated for MySQL on every dialect so a
// schema carries the same indexes everywhere.
func TestEnsureUniqueFieldIndexes_SkipsJSONMappedFields(t *testing.T) {
	for _, tc := range []struct {
		name       string
		fieldType  nemgen.FieldType
		typeConfig *nemgen.FieldTypeConfig
		wantSkip   bool
	}{
		{"json", nemgen.FieldType_FIELD_TYPE_JSON, &nemgen.FieldTypeConfig{}, true},
		{"array", nemgen.FieldType_FIELD_TYPE_ARRAY, &nemgen.FieldTypeConfig{}, true},
		{"json without type config", nemgen.FieldType_FIELD_TYPE_JSON, nil, true},
		{
			"multi valued enum",
			nemgen.FieldType_FIELD_TYPE_ENUM,
			&nemgen.FieldTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{AllowMultiple: true}},
			true,
		},
		{
			"multi valued file",
			nemgen.FieldType_FIELD_TYPE_FILE,
			&nemgen.FieldTypeConfig{File: &nemgen.FieldTypeFileConfig{AllowMultiple: true}},
			true,
		},
		{
			"single valued enum",
			nemgen.FieldType_FIELD_TYPE_ENUM,
			&nemgen.FieldTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{}},
			false,
		},
		{"enum without its config", nemgen.FieldType_FIELD_TYPE_ENUM, &nemgen.FieldTypeConfig{}, false},
		{"varchar", nemgen.FieldType_FIELD_TYPE_VARCHAR, &nemgen.FieldTypeConfig{}, false},
		{"text", nemgen.FieldType_FIELD_TYPE_TEXT, &nemgen.FieldTypeConfig{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := uniqueFieldEntity()
			e.Fields = []*nemgen.Field{{
				Uuid:       ufNameUUID,
				Identifier: "payload",
				Type:       tc.fieldType,
				TypeConfig: tc.typeConfig,
				Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
				Unique:     true,
			}}
			added := ensure(e)
			if tc.wantSkip {
				assert.Empty(t, added, "a JSON column cannot be indexed on MySQL")
				return
			}
			require.Len(t, added, 1)
			assert.Equal(t, "uq_member_payload", added[0].Identifier)
		})
	}
}

// Only standalone entities become tables; a dependent entity has no schema of its
// own to index.
func TestEnsureUniqueFieldIndexes_SkipsNonStandaloneEntity(t *testing.T) {
	e := uniqueFieldEntity()
	e.Type = nemgen.EntityType_ENTITY_TYPE_DEPENDENT
	assert.Empty(t, ensure(e))
	assert.Nil(t, e.TypeConfig, "a skipped entity must not gain a type config")
}

// An inactive field is not emitted as a column, matching the filter the mapper
// applies — an index over a column that does not exist is invalid DDL.
func TestEnsureUniqueFieldIndexes_SkipsInactiveField(t *testing.T) {
	for _, status := range []nemgen.FieldStatus{
		nemgen.FieldStatus_FIELD_STATUS_INACTIVE,
		nemgen.FieldStatus_FIELD_STATUS_INVALID,
	} {
		e := uniqueFieldEntity()
		for _, f := range e.Fields {
			f.Status = status
		}
		assert.Empty(t, ensure(e), "status %v", status)
	}
}

// A field the mapper drops (no identifier, no type) cannot be indexed either.
func TestEnsureUniqueFieldIndexes_SkipsUnmappableField(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field *nemgen.Field
	}{
		{"no identifier", &nemgen.Field{Uuid: ufNameUUID, Type: nemgen.FieldType_FIELD_TYPE_VARCHAR, TypeConfig: &nemgen.FieldTypeConfig{}, Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, Unique: true}},
		{"invalid type", &nemgen.Field{Uuid: ufNameUUID, Identifier: "ghost", TypeConfig: &nemgen.FieldTypeConfig{}, Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, Unique: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := uniqueFieldEntity()
			e.Fields = []*nemgen.Field{tc.field}
			assert.Empty(t, ensure(e))
		})
	}
}

// A normalized project version can reach GenerateSQL after this already ran over
// it, so a second pass must add nothing: the synthesized index satisfies the
// suppression rule itself.
func TestEnsureUniqueFieldIndexes_Idempotent(t *testing.T) {
	e := uniqueFieldEntity()
	pv := &nemgen.ProjectVersion{Entities: []*nemgen.Entity{e}}

	EnsureUniqueFieldIndexes(pv)
	first := e.TypeConfig.Standalone.Indexes
	require.Len(t, first, 1)

	EnsureUniqueFieldIndexes(pv)
	EnsureUniqueFieldIndexes(pv)
	second := e.TypeConfig.Standalone.Indexes
	require.Len(t, second, 1, "a synthesized index must suppress its own re-synthesis")
	assert.Equal(t, first[0].Uuid, second[0].Uuid)
	assert.Equal(t, first[0].Identifier, second[0].Identifier)
}

// The uuid and identifier must be pure functions of the schema. A random uuid or
// a positional counter would make every regeneration look like a different index
// to anything tracking them by identity — which is exactly what makes a diff
// engine propose a drop and recreate on every plan.
func TestEnsureUniqueFieldIndexes_Deterministic(t *testing.T) {
	first := ensure(uniqueFieldEntity())
	second := ensure(uniqueFieldEntity())
	require.Len(t, first, 1)
	require.Len(t, second, 1)

	assert.Equal(t, first[0].Uuid, second[0].Uuid)
	assert.Equal(t, first[0].Identifier, second[0].Identifier)

	parsed, err := uuid.FromString(first[0].Uuid)
	require.NoError(t, err, "the synthesized uuid must be a real uuid")
	assert.Equal(t, byte(5), parsed.Version(), "derived from the schema, i.e. a v5 name-based uuid")

	// Same field uuid under a different entity is a different index.
	other := uniqueFieldEntity()
	other.Uuid = "aaaa2222-0000-0000-0000-000000000001"
	otherIndexes := ensure(other)
	require.Len(t, otherIndexes, 1)
	assert.NotEqual(t, first[0].Uuid, otherIndexes[0].Uuid)
}

// Two unique fields on one entity get one index each, appended in field order.
func TestEnsureUniqueFieldIndexes_MultipleFields(t *testing.T) {
	e := uniqueFieldEntity()
	e.Fields[2].Unique = true // name

	added := ensure(e)
	require.Len(t, added, 2)
	assert.Equal(t, "uq_member_email", added[0].Identifier)
	assert.Equal(t, "uq_member_name", added[1].Identifier)
}

// A name already spoken for on the entity — including by an inactive index, which
// still owns its name — gets the field's uuid prefix appended rather than
// shadowing it.
func TestEnsureUniqueFieldIndexes_IdentifierCollision(t *testing.T) {
	existing := index("uq_member_email", nemgen.IndexType_INDEX_TYPE_UNIQUE, nemgen.IndexStatus_INDEX_STATUS_INVALID, ufEmailUUID)
	e := uniqueFieldEntity(existing)

	after := ensure(e)
	added := synthesized([]*nemgen.Index{existing}, after)
	require.Len(t, added, 1)
	assert.Equal(t, "uq_member_email_"+ufEmailUUID[:8], added[0].Identifier)
	assert.Equal(t, existing.Identifier, after[0].Identifier, "the existing index keeps its name")
}

// Postgres truncates identifiers at 63 bytes and MySQL rejects them past 64. A
// name that would overflow is trimmed and discriminated by the field uuid, so two
// long names cannot collapse into one index in the database.
func TestEnsureUniqueFieldIndexes_LongIdentifier(t *testing.T) {
	e := uniqueFieldEntity()
	e.Identifier = strings.Repeat("entity_", 8) // 56 chars
	e.Fields[1].Identifier = strings.Repeat("field_", 5)

	added := ensure(e)
	require.Len(t, added, 1)
	assert.LessOrEqual(t, len(added[0].Identifier), maxSynthesizedIndexIdentifierLength)
	assert.True(t, strings.HasSuffix(added[0].Identifier, "_"+ufEmailUUID[:8]), "got %q", added[0].Identifier)
	assert.True(t, strings.HasPrefix(added[0].Identifier, "uq_entity_"), "got %q", added[0].Identifier)
}

// An entity this feature does not touch must come out exactly as it went in. The
// mapper and the select resolver both gate on TypeConfig being non-nil, so
// materializing an empty one would change output for unrelated entities.
func TestEnsureUniqueFieldIndexes_LeavesUntouchedEntityAlone(t *testing.T) {
	e := uniqueFieldEntity()
	e.Fields[1].Unique = false // nothing left to synthesize (uuid is a key field)

	assert.Empty(t, ensure(e))
	assert.Nil(t, e.TypeConfig, "no type config may be created when nothing is synthesized")

	// An entity that already carries an empty standalone config keeps it empty.
	withConfig := uniqueFieldEntity()
	withConfig.Fields[1].Unique = false
	withConfig.TypeConfig = &nemgen.EntityTypeConfig{Standalone: &nemgen.EntityTypeStandaloneConfig{}}
	assert.Empty(t, ensure(withConfig))
}

func TestEnsureUniqueFieldIndexes_NilSafe(t *testing.T) {
	assert.NotPanics(t, func() { EnsureUniqueFieldIndexes(nil) })
	assert.NotPanics(t, func() { EnsureUniqueFieldIndexes(&nemgen.ProjectVersion{}) })
	assert.NotPanics(t, func() {
		EnsureUniqueFieldIndexes(&nemgen.ProjectVersion{Entities: []*nemgen.Entity{nil}})
	})
}

// End to end: the flag has to reach the DDL, as a real UNIQUE index on both
// dialects — this is the behavior that was missing entirely.
func TestEnsureUniqueFieldIndexes_RendersUniqueDDL(t *testing.T) {
	for _, tc := range []struct {
		dbType db.DBType
		want   string
	}{
		{db.MYSQLDBType, "UNIQUE INDEX `uq_member_email` (`email`)"},
		{db.PGDBType, `UNIQUE ("email")`},
	} {
		e := uniqueFieldEntity()
		ensure(e)
		assert.Contains(t, renderCreate(t, e, tc.dbType), tc.want)
	}
}

// A synthesized index is select-eligible exactly like an explicit one, so the
// unique column becomes a fetch — the other half of what `unique: true` never
// delivered.
func TestEnsureUniqueFieldIndexes_ProducesSelect(t *testing.T) {
	e := uniqueFieldEntity()
	ensure(e)

	names := []string{}
	for _, s := range ResolveSelectStatements(e, db.PGDBType) {
		names = append(names, s.Name)
	}
	assert.Contains(t, names, "MemberByEmail")
}

// ...and it counts toward the power-set cap like an explicit one. An entity
// sitting exactly at the threshold degrades to the bounded one-select-per-index
// fallback once synthesis pushes it over, rather than doubling the 2^N work.
func TestEnsureUniqueFieldIndexes_CountsTowardSelectCap(t *testing.T) {
	e := buildEntityWithIndexes(maxPowerSetIndexes)
	for _, f := range e.Fields {
		f.Status = nemgen.FieldStatus_FIELD_STATUS_ACTIVE
	}
	assert.Len(t, ResolveSelectStatements(e, db.MYSQLDBType), (1<<maxPowerSetIndexes)-1,
		"at the threshold the full power set is still generated")

	e.Fields = append(e.Fields, &nemgen.Field{
		Uuid:       ufEmailUUID,
		Identifier: "email",
		Type:       nemgen.FieldType_FIELD_TYPE_EMAIL,
		TypeConfig: &nemgen.FieldTypeConfig{},
		Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
		Unique:     true,
	})
	ensure(e)

	indexes := e.TypeConfig.Standalone.Indexes
	require.Len(t, indexes, maxPowerSetIndexes+1)
	assert.Len(t, ResolveSelectStatements(e, db.MYSQLDBType), maxPowerSetIndexes+1,
		fmt.Sprintf("one select per index above the cap, not %d", (1<<(maxPowerSetIndexes+1))-1))
}

// GenerateSQL is the single entry point every consumer goes through, including
// callers that hand it a raw project version, so the desugaring has to happen
// there rather than in a normalization step upstream.
func TestGenerateSQLDesugarsUniqueFields(t *testing.T) {
	e := uniqueFieldEntity()
	pv := &nemgen.ProjectVersion{Entities: []*nemgen.Entity{e}}

	res, err := GenerateSQL(t.Context(), GenerateRequest{
		ExecutionUUID: uuid.Must(uuid.NewV4()).String(),
		Configvalues: &ConfigValues{
			DBType:   db.PGDBType,
			Entities: []string{ufEntityUUID},
			Actions:  []Action{CreateAction},
		},
		ProjectVersion: pv,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(res.WorkingDir)
		os.RemoveAll(res.ZipFile)
	})

	require.Len(t, res.Results, 1)
	assert.Contains(t, res.Results[0].Data, `UNIQUE ("email")`)
}
