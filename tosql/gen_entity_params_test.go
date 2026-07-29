package tosql

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Entities from testdata/project_version.json. `user` has a composite primary
// key (uuid + version) and 7 non-key fields; `post` has a single primary key,
// 12 non-key fields and a JSON column (media).
const (
	testUserEntityUUID = "b8629dd5-f6e5-483f-893a-842357e171fc"
	testPostEntityUUID = "6f9ca9c7-6af3-4301-82d2-739ec84eab83"
)

const (
	userFieldUUID      = "c574a76b-8ddc-4a04-81f0-0493dfe2c396"
	userFieldVersion   = "243d41f7-9e81-411d-90fc-2cba16ef8a21"
	userFieldEmail     = "2d7a5810-5537-4c4b-b126-9598753f404f"
	userFieldPassword  = "4e80777e-dccc-41c1-a0c1-ed85860786ea"
	userFieldStatus    = "0c06f255-6223-4639-ada2-5706a1354856"
	userFieldCreatedAt = "41c16015-b85f-4019-b8b0-9d8cff43dcb6"
	userFieldUpdatedAt = "4b5cca79-0f3e-419e-af4d-87f82df57888"
	userFieldCreatedBy = "21f31a2b-00e5-415f-9d49-f0728d9f6884"
	userFieldUpdatedBy = "e996512e-876f-4c18-a3f5-8381884488fb"

	postFieldUUID  = "b1758fa3-17ca-471d-af79-d2377c35412b"
	postFieldTitle = "4fbafa30-db80-4d8a-8f5f-718f79570418"
	postFieldMedia = "cb31c929-71f9-4b40-b176-ce508adf14dc"
)

func loadTestProjectVersion(t *testing.T) *nemgen.ProjectVersion {
	t.Helper()
	pvdata, err := os.ReadFile("./testdata/project_version.json")
	require.NoError(t, err)
	pv := &nemgen.ProjectVersion{}
	require.NoError(t, json.Unmarshal(pvdata, pv))
	return pv
}

func testEntity(t *testing.T, pv *nemgen.ProjectVersion, entityUUID string) *nemgen.Entity {
	t.Helper()
	for _, e := range pv.Entities {
		if e.Uuid == entityUUID {
			return e
		}
	}
	t.Fatalf("entity %s not found in testdata", entityUUID)
	return nil
}

var pgPlaceholderRegexp = regexp.MustCompile(`\$(\d+)`)

// assertPGParamsContiguous is the invariant that broke when applying data change
// requests to postgres: the placeholders a statement references must be exactly
// $1..$n for the n parameters that get bound, each used once. When the SET and
// WHERE clauses number themselves independently the statement ends up
// referencing placeholders nothing binds, and postgres rejects it at parse time
// with "could not determine data type of parameter $N".
func assertPGParamsContiguous(t *testing.T, sql string, paramCount int) {
	t.Helper()
	found := []int{}
	for _, match := range pgPlaceholderRegexp.FindAllStringSubmatch(sql, -1) {
		index, err := strconv.Atoi(match[1])
		require.NoError(t, err)
		found = append(found, index)
	}
	sort.Ints(found)

	expected := []int{}
	for i := 1; i <= paramCount; i++ {
		expected = append(expected, i)
	}
	assert.Equal(t, expected, found, "placeholders in %q must be exactly $1..$%d, one each", sql, paramCount)
}

// A change request that updates a subset of an entity's fields — by far the most
// common shape — used to number the primary key from the entity's field count
// instead of from the SET clause, producing `SET "title" = $1 WHERE "uuid" = $13`
// with only two parameters bound.
func TestGenerateUpdatePGPartialFields(t *testing.T) {
	pv := loadTestProjectVersion(t)
	res, err := GenerateUpdateForEntityWithValues(context.Background(), GenerateUpdateForEntityWithValuesParams{
		Entity:         testEntity(t, pv, testPostEntityUUID),
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Values:         map[string]string{postFieldTitle: "hello"},
		Keys:           map[string]string{postFieldUUID: "9b2c1c2e-0000-4000-8000-000000000001"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Contains(t, res.ParametrizedSQL, `"title" = $1`)
	assert.Contains(t, res.ParametrizedSQL, `"uuid" = $2`)
	assert.Equal(t, []string{"hello", "9b2c1c2e-0000-4000-8000-000000000001"}, res.Params)
	assertPGParamsContiguous(t, res.ParametrizedSQL, len(res.Params))
}

func TestGenerateUpdatePGCompositePrimaryKey(t *testing.T) {
	pv := loadTestProjectVersion(t)
	res, err := GenerateUpdateForEntityWithValues(context.Background(), GenerateUpdateForEntityWithValuesParams{
		Entity:         testEntity(t, pv, testUserEntityUUID),
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Values: map[string]string{
			userFieldEmail:  "user@nuzur.dev",
			userFieldStatus: "1",
		},
		Keys: map[string]string{
			userFieldUUID:    "9b2c1c2e-0000-4000-8000-000000000002",
			userFieldVersion: "3",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Contains(t, res.ParametrizedSQL, `"email" = $1`)
	assert.Contains(t, res.ParametrizedSQL, `"status" = $2`)
	assert.Contains(t, res.ParametrizedSQL, `"uuid" = $3 AND "version" = $4`)
	assert.Len(t, res.Params, 4)
	assertPGParamsContiguous(t, res.ParametrizedSQL, len(res.Params))
}

// Even updating every non-key field, a composite primary key used to be numbered
// one slot too high because the offset was derived from the entity's total field
// count rather than from the SET clause.
func TestGenerateUpdatePGAllFieldsCompositePrimaryKey(t *testing.T) {
	pv := loadTestProjectVersion(t)
	res, err := GenerateUpdateForEntityWithValues(context.Background(), GenerateUpdateForEntityWithValuesParams{
		Entity:         testEntity(t, pv, testUserEntityUUID),
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Values: map[string]string{
			userFieldEmail:     "user@nuzur.dev",
			userFieldPassword:  "secret",
			userFieldStatus:    "1",
			userFieldCreatedAt: "2026-07-28T00:00:00Z",
			userFieldUpdatedAt: "2026-07-28T00:00:00Z",
			userFieldCreatedBy: "9b2c1c2e-0000-4000-8000-000000000003",
			userFieldUpdatedBy: "9b2c1c2e-0000-4000-8000-000000000004",
		},
		Keys: map[string]string{
			userFieldUUID:    "9b2c1c2e-0000-4000-8000-000000000002",
			userFieldVersion: "3",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Contains(t, res.ParametrizedSQL, `"uuid" = $8 AND "version" = $9`)
	assert.Len(t, res.Params, 9)
	assertPGParamsContiguous(t, res.ParametrizedSQL, len(res.Params))
}

// A JSON column set to an empty value renders as a literal NULL and binds no
// parameter, so it must not consume a placeholder index either.
func TestGenerateUpdatePGEmptyJSONFieldConsumesNoParam(t *testing.T) {
	pv := loadTestProjectVersion(t)
	res, err := GenerateUpdateForEntityWithValues(context.Background(), GenerateUpdateForEntityWithValuesParams{
		Entity:         testEntity(t, pv, testPostEntityUUID),
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Values: map[string]string{
			postFieldTitle: "hello",
			postFieldMedia: "",
		},
		Keys: map[string]string{postFieldUUID: "9b2c1c2e-0000-4000-8000-000000000001"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Contains(t, res.ParametrizedSQL, `"media" = NULL`)
	assert.Contains(t, res.ParametrizedSQL, `"title" = $1`)
	assert.Contains(t, res.ParametrizedSQL, `"uuid" = $2`)
	assert.Equal(t, []string{"hello", "9b2c1c2e-0000-4000-8000-000000000001"}, res.Params)
	assertPGParamsContiguous(t, res.ParametrizedSQL, len(res.Params))
}

func TestGenerateInsertPGParamsContiguous(t *testing.T) {
	pv := loadTestProjectVersion(t)
	res, err := GenerateInsertForEntityWithValues(context.Background(), GenerateInsertForEntityWithValuesParams{
		Entity:         testEntity(t, pv, testPostEntityUUID),
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Values: map[string]string{
			postFieldUUID:  "9b2c1c2e-0000-4000-8000-000000000001",
			postFieldTitle: "hello",
			postFieldMedia: "",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	// media is JSON with an empty value: NULL literal, no bound parameter.
	assert.Len(t, res.Params, 2)
	assertPGParamsContiguous(t, res.ParametrizedSQL, len(res.Params))
}

func TestGenerateDeletePGParamsContiguous(t *testing.T) {
	pv := loadTestProjectVersion(t)
	res, err := GenerateDeleteForEntityWithValues(context.Background(), GenerateDeleteForEntityWithValuesParams{
		Entity:         testEntity(t, pv, testUserEntityUUID),
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Keys: map[string]string{
			userFieldUUID:    "9b2c1c2e-0000-4000-8000-000000000002",
			userFieldVersion: "3",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Contains(t, res.ParametrizedSQL, `"uuid" = $1 AND "version" = $2`)
	assert.Len(t, res.Params, 2)
	assertPGParamsContiguous(t, res.ParametrizedSQL, len(res.Params))
}

// mysql binds positionally with `?`, so it never had the numbering problem —
// guard against a fix for postgres leaking into it.
func TestGenerateUpdateMySQLUsesQuestionMarks(t *testing.T) {
	pv := loadTestProjectVersion(t)
	res, err := GenerateUpdateForEntityWithValues(context.Background(), GenerateUpdateForEntityWithValuesParams{
		Entity:         testEntity(t, pv, testUserEntityUUID),
		ProjectVersion: pv,
		DBType:         db.MYSQLDBType,
		Values:         map[string]string{userFieldEmail: "user@nuzur.dev"},
		Keys: map[string]string{
			userFieldUUID:    "9b2c1c2e-0000-4000-8000-000000000002",
			userFieldVersion: "3",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.NotContains(t, res.ParametrizedSQL, "$")
	assert.Equal(t, len(res.Params), strings.Count(res.ParametrizedSQL, "?"))
}

// setGenerated marks fields as `generated: true` on a freshly loaded fixture.
// Nothing in testdata/project_version.json carries the flag, but it is the
// documented convention for created_at/updated_at, so the insert behavior that
// depends on it has to be set up explicitly.
func setGenerated(t *testing.T, e *nemgen.Entity, fieldUUIDs ...string) {
	t.Helper()
	wanted := make(map[string]bool, len(fieldUUIDs))
	for _, id := range fieldUUIDs {
		wanted[id] = true
	}
	found := 0
	for _, f := range e.Fields {
		if wanted[f.Uuid] {
			f.Generated = true
			found++
		}
	}
	require.Equal(t, len(fieldUUIDs), found, "not every field to mark generated was found on %q", e.Identifier)
}

// A `created_at datetime, required: true, generated: true` column is emitted by
// this package's own DDL as `NOT NULL DEFAULT CURRENT_TIMESTAMP`, and upstream
// validation excuses the caller from supplying a value for it. The insert used to
// name every column of the entity regardless, binding an explicit NULL for the
// ones the change request left out — which overrides the default and fails the
// NOT NULL constraint on postgres (and, on mysql, writes a zero datetime or
// errors under strict mode). Generated columns must not appear at all.
func TestGenerateInsertOmitsGeneratedColumns(t *testing.T) {
	pv := loadTestProjectVersion(t)
	entity := testEntity(t, pv, testUserEntityUUID)
	setGenerated(t, entity, userFieldCreatedAt, userFieldUpdatedAt)

	res, err := GenerateInsertForEntityWithValues(context.Background(), GenerateInsertForEntityWithValuesParams{
		Entity:         entity,
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Values: map[string]string{
			userFieldUUID:    "9b2c1c2e-0000-4000-8000-000000000003",
			userFieldVersion: "1",
			userFieldEmail:   "user@nuzur.dev",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	for _, sql := range []string{res.SQL, res.ParametrizedSQL} {
		assert.NotContains(t, sql, `"created_at"`, "generated column must not be named in %q", sql)
		assert.NotContains(t, sql, `"updated_at"`, "generated column must not be named in %q", sql)
	}
	assert.Equal(t, []string{"9b2c1c2e-0000-4000-8000-000000000003", "1", "user@nuzur.dev"}, res.Params)
	assertPGParamsContiguous(t, res.ParametrizedSQL, len(res.Params))
}

// The counterpart to the above: a column the database won't fill on its own keeps
// its explicit NULL when the change request omits it. This matters because
// mapField gives *every* datetime a DEFAULT CURRENT_TIMESTAMP, so dropping absent
// columns wholesale would silently populate an optional deleted_at and create rows
// that are already soft-deleted.
func TestGenerateInsertKeepsAbsentNonGeneratedColumnsAsNull(t *testing.T) {
	pv := loadTestProjectVersion(t)
	entity := testEntity(t, pv, testUserEntityUUID)
	setGenerated(t, entity, userFieldCreatedAt)

	res, err := GenerateInsertForEntityWithValues(context.Background(), GenerateInsertForEntityWithValuesParams{
		Entity:         entity,
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Values: map[string]string{
			userFieldUUID:    "9b2c1c2e-0000-4000-8000-000000000003",
			userFieldVersion: "1",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	// updated_at is the same datetime type as created_at and equally absent — the
	// only difference is the generated flag.
	assert.NotContains(t, res.ParametrizedSQL, `"created_at"`)
	assert.Contains(t, res.ParametrizedSQL, `"updated_at"`)
	assert.Contains(t, res.ParametrizedSQL, "NULL")
	assert.Len(t, res.Params, 2)
	assertPGParamsContiguous(t, res.ParametrizedSQL, len(res.Params))
}

// An auto-increment key is filled by its sequence, and upstream validation grants
// it the same exemption as a generated field, so it gets the same treatment.
func TestGenerateInsertOmitsAutoIncrementKey(t *testing.T) {
	pv := loadTestProjectVersion(t)
	entity := testEntity(t, pv, testUserEntityUUID)
	for _, f := range entity.Fields {
		if f.Uuid == userFieldVersion {
			f.KeyAutoIncrement = true
		}
	}

	res, err := GenerateInsertForEntityWithValues(context.Background(), GenerateInsertForEntityWithValuesParams{
		Entity:         entity,
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Values:         map[string]string{userFieldUUID: "9b2c1c2e-0000-4000-8000-000000000003"},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.NotContains(t, res.ParametrizedSQL, `"version"`)
	assert.Equal(t, []string{"9b2c1c2e-0000-4000-8000-000000000003"}, res.Params)
}

// mysql binds positionally, so the omitted columns must drop out of the column
// list and the parameter list together or every remaining value shifts one slot.
func TestGenerateInsertMySQLOmitsGeneratedColumns(t *testing.T) {
	pv := loadTestProjectVersion(t)
	entity := testEntity(t, pv, testUserEntityUUID)
	setGenerated(t, entity, userFieldCreatedAt, userFieldUpdatedAt)

	res, err := GenerateInsertForEntityWithValues(context.Background(), GenerateInsertForEntityWithValuesParams{
		Entity:         entity,
		ProjectVersion: pv,
		DBType:         db.MYSQLDBType,
		Values: map[string]string{
			userFieldUUID:    "9b2c1c2e-0000-4000-8000-000000000003",
			userFieldVersion: "1",
			userFieldEmail:   "user@nuzur.dev",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.NotContains(t, res.ParametrizedSQL, "`created_at`")
	assert.NotContains(t, res.ParametrizedSQL, "`updated_at`")
	assert.NotContains(t, res.ParametrizedSQL, "$")
	assert.Equal(t, len(res.Params), strings.Count(res.ParametrizedSQL, "?"))
	assert.Len(t, res.Params, 3)
}

// `INSERT INTO t () VALUES ()` is invalid on postgres. Upstream validation makes
// this unreachable, so fail loudly rather than emit a broken statement.
func TestGenerateInsertWithNothingToInsertErrors(t *testing.T) {
	pv := loadTestProjectVersion(t)
	entity := testEntity(t, pv, testUserEntityUUID)
	fieldUUIDs := []string{}
	for _, f := range entity.Fields {
		fieldUUIDs = append(fieldUUIDs, f.Uuid)
	}
	setGenerated(t, entity, fieldUUIDs...)

	_, err := GenerateInsertForEntityWithValues(context.Background(), GenerateInsertForEntityWithValuesParams{
		Entity:         entity,
		ProjectVersion: pv,
		DBType:         db.PGDBType,
		Values:         map[string]string{},
	})
	assert.ErrorContains(t, err, "no insertable values")
}

// The generic (non-value) template path feeds go-code-gen's generated postgres
// queries, where every non-key field is in the SET clause. Composite keys used
// to be numbered one slot too high here as well.
func TestUpdateTemplateNumberingForGolang(t *testing.T) {
	pv := loadTestProjectVersion(t)
	entityTemplate, err := MapEntityToSchemaEntity(testEntity(t, pv, testUserEntityUUID), pv, db.PGDBType, true)
	require.NoError(t, err)

	statement := entityTemplate.UpdateFields() + " WHERE " + entityTemplate.PrimaryKeysWhereClauseForUpdate()

	assert.Contains(t, statement, `"email" = $1`)
	assert.Contains(t, statement, `"uuid" = $8 AND "version" = $9`)
	assertPGParamsContiguous(t, statement, 9)
}

// IsPrimaryKey compared raw field identifiers against quoted ones, so it never
// matched and NumOfNonePKFields counted every field.
func TestPrimaryKeyIdentificationIgnoresQuoting(t *testing.T) {
	pv := loadTestProjectVersion(t)
	entityTemplate, err := MapEntityToSchemaEntity(testEntity(t, pv, testUserEntityUUID), pv, db.PGDBType, true)
	require.NoError(t, err)

	assert.True(t, entityTemplate.IsPrimaryKey("uuid"))
	assert.True(t, entityTemplate.IsPrimaryKey(`"uuid"`))
	assert.False(t, entityTemplate.IsPrimaryKey("email"))
	assert.Equal(t, 7, entityTemplate.NumOfNonePKFields())
}
