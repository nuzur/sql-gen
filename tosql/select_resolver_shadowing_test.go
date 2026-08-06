package tosql

import (
	"reflect"
	"strings"
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The select resolver enumerates the power set of an entity's indexes and names
// each subset after the union of its non-datetime columns. Two independent things
// can then go wrong, and both did:
//
//   - a *combination* of indexes can union to exactly the column set of a real,
//     later index, claim its name, and — being tagged CombinedIndexes — render
//     only into select_indexed_combined.sql, which go-code-gen discards. The real
//     index's query then exists nowhere while go-code-gen's own resolver still
//     emits a module wrapper calling it, and the generated app fails to compile
//     (`queries.FetchNotificationByUserUUIDAndStatus undefined`).
//   - two combinations over the *same* column set produce two names for one
//     query, because names accrete in index traversal order while the WHERE
//     clause is sorted alphabetically.
//
// These tests pin both, plus the two smaller emission rules the same pass fixes:
// the primary select owns its name, and a column the mapper does not emit never
// appears in a select.

// selectFixtureField builds an ACTIVE field the mapper will emit as a column.
// CHAR is given a type config because FieldTypeToMYSQL dereferences it.
func selectFixtureField(uuid string, identifier string, fieldType nemgen.FieldType) *nemgen.Field {
	f := &nemgen.Field{
		Uuid:       uuid,
		Identifier: identifier,
		Type:       fieldType,
		Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
	}
	if fieldType == nemgen.FieldType_FIELD_TYPE_CHAR {
		f.TypeConfig = &nemgen.FieldTypeConfig{
			Char: &nemgen.FieldTypeCharConfig{MaxSize: 32},
		}
	}
	return f
}

// selectFixtureIndex builds an ACTIVE index over the given fields, in the order
// given — index field order is what the generated name is built from.
func selectFixtureIndex(uuid string, identifier string, indexType nemgen.IndexType, fieldUUIDs ...string) *nemgen.Index {
	fields := make([]*nemgen.IndexField, 0, len(fieldUUIDs))
	for n, fu := range fieldUUIDs {
		fields = append(fields, &nemgen.IndexField{
			FieldUuid: fu,
			Priority:  int64(n),
			Order:     nemgen.IndexFieldOrder_INDEX_FIELD_ORDER_ASC,
		})
	}
	return &nemgen.Index{
		Uuid:       uuid,
		Identifier: identifier,
		Type:       indexType,
		Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
		Fields:     fields,
	}
}

func selectFixtureEntity(identifier string, fields []*nemgen.Field, indexes []*nemgen.Index) *nemgen.Entity {
	return &nemgen.Entity{
		Uuid:       "entity-" + identifier,
		Identifier: identifier,
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Status:     nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
		Fields:     fields,
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: indexes,
			},
		},
	}
}

// notificationFixture is the schema that broke the aburrides deploy, reduced to
// what matters. The index ORDER is load-bearing: Combinations visits the pair
// {0,1} (subsetBits=3) before the singleton {2} (subsetBits=4), and after the
// datetime members are dropped from the name that pair unions to exactly
// {user_uuid, status} — the column set of index 2.
func notificationFixture() *nemgen.Entity {
	fields := []*nemgen.Field{
		selectFixtureField("f-user", "user_uuid", nemgen.FieldType_FIELD_TYPE_UUID),
		selectFixtureField("f-status", "status", nemgen.FieldType_FIELD_TYPE_CHAR),
		selectFixtureField("f-dedupe", "dedupe_key", nemgen.FieldType_FIELD_TYPE_CHAR),
		selectFixtureField("f-created", "created_at", nemgen.FieldType_FIELD_TYPE_DATETIME),
		selectFixtureField("f-scheduled", "scheduled_for", nemgen.FieldType_FIELD_TYPE_DATETIME),
		selectFixtureField("f-read", "read_at", nemgen.FieldType_FIELD_TYPE_DATETIME),
	}
	indexes := []*nemgen.Index{
		selectFixtureIndex("i-user-created", "idx_notification_user_created",
			nemgen.IndexType_INDEX_TYPE_INDEX, "f-user", "f-created"),
		selectFixtureIndex("i-status-scheduled", "idx_notification_status_scheduled",
			nemgen.IndexType_INDEX_TYPE_INDEX, "f-status", "f-scheduled"),
		selectFixtureIndex("i-user-status-read", "idx_notification_user_status_read",
			nemgen.IndexType_INDEX_TYPE_INDEX, "f-user", "f-status", "f-read"),
		selectFixtureIndex("i-user-dedupe", "idx_notification_user_dedupe",
			nemgen.IndexType_INDEX_TYPE_UNIQUE, "f-user", "f-dedupe"),
	}
	return selectFixtureEntity("notification", fields, indexes)
}

// selectFieldNames is the WHERE-clause columns of a statement, in emission order
// (which the resolver sorts alphabetically).
func selectFieldNames(s SchemaSelectStatement) []string {
	names := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		names = append(names, f.Name)
	}
	return names
}

func findSelect(selects []SchemaSelectStatement, name string) (SchemaSelectStatement, bool) {
	for _, s := range selects {
		if s.Name == name {
			return s, true
		}
	}
	return SchemaSelectStatement{}, false
}

func assertNoDuplicateNames(t *testing.T, selects []SchemaSelectStatement) {
	t.Helper()
	seen := map[string]bool{}
	for _, s := range selects {
		assert.Falsef(t, seen[s.Name], "duplicate select name %q — sqlc rejects the whole file", s.Name)
		seen[s.Name] = true
	}
}

// The regression: the real (user_uuid, status, read_at) index must own
// NotificationByUserUUIDAndStatus, as a NON-combined select, because
// select_indexed_simple.sql is the only indexed-select file go-code-gen ships and
// it renders CombinedIndexes == false only.
func TestResolveSelectStatements_SingleIndexNotShadowedByCombination(t *testing.T) {
	// Both dialects, because the resolver's name/field choices must not depend on
	// the column types the mapper picks.
	for _, dbType := range []db.DBType{db.MYSQLDBType, db.PGDBType} {
		t.Run(string(dbType), func(t *testing.T) {
			selects := ResolveSelectStatements(notificationFixture(), dbType)

			stmt, found := findSelect(selects, "NotificationByUserUUIDAndStatus")
			require.True(t, found,
				"the (user_uuid, status, read_at) index lost its query name to a combination")

			assert.False(t, stmt.CombinedIndexes,
				"a real index must be emitted as a simple select; combined ones are discarded by go-code-gen")
			assert.Equal(t, []string{"status", "user_uuid"}, selectFieldNames(stmt),
				"datetime members are excluded from the WHERE clause and the rest are sorted")
			require.NotEmpty(t, stmt.Fields)
			assert.True(t, stmt.Fields[len(stmt.Fields)-1].IsLast,
				"the last WHERE field drives the template's AND separator")

			assertNoDuplicateNames(t, selects)
		})
	}
}

// Every index gets a select before any combination does, which is what makes the
// shadowing above unreachable rather than merely unlikely.
func TestResolveSelectStatements_SinglesEmittedBeforeCombinations(t *testing.T) {
	selects := ResolveSelectStatements(notificationFixture(), db.MYSQLDBType)

	sawCombined := false
	for _, s := range selects {
		if s.IsPrimary {
			// The primary select is minted before the index loop and is not part
			// of the partition.
			continue
		}
		if s.CombinedIndexes {
			sawCombined = true
			continue
		}
		assert.Falsef(t, sawCombined,
			"single-index select %q was emitted after a combined one", s.Name)
	}
	assert.True(t, sawCombined, "fixture should still produce combined selects")
}

// The >maxPowerSetIndexes fallback already returns only singletons, so the
// reordering must be provably an identity on it — otherwise the two code paths
// could disagree about emission order.
func TestSinglesFirst_NoOpForSingleIndexSubsets(t *testing.T) {
	in := []string{"idx-a", "idx-b", "idx-c", "idx-d"}
	subsets := singleIndexSubsets(in)
	assert.True(t, reflect.DeepEqual(subsets, singlesFirst(singleIndexSubsets(in))),
		"singlesFirst must not reorder an all-singleton input")
}

// Two combinations over one column set are one query under two names
// (PostBySlugAndTitle / PostByTitleAndSlug): byte-identical SQL, twice the
// generated surface. Combinations dedupe by column set — singles never do, because
// go-code-gen mints exactly one wrapper per index and each must find its query.
func TestResolveSelectStatements_CombinedTwinCollapse(t *testing.T) {
	fields := []*nemgen.Field{
		selectFixtureField("f-title", "title", nemgen.FieldType_FIELD_TYPE_CHAR),
		selectFixtureField("f-slug", "slug", nemgen.FieldType_FIELD_TYPE_CHAR),
		selectFixtureField("f-status", "status", nemgen.FieldType_FIELD_TYPE_CHAR),
	}
	indexes := []*nemgen.Index{
		// listed (slug, title), so its own name is PostBySlugAndTitle
		selectFixtureIndex("i-slug-title", "idx_post_slug_title",
			nemgen.IndexType_INDEX_TYPE_UNIQUE, "f-slug", "f-title"),
		selectFixtureIndex("i-title", "idx_post_title", nemgen.IndexType_INDEX_TYPE_INDEX, "f-title"),
		selectFixtureIndex("i-slug", "idx_post_slug", nemgen.IndexType_INDEX_TYPE_INDEX, "f-slug"),
		selectFixtureIndex("i-status", "idx_post_status", nemgen.IndexType_INDEX_TYPE_INDEX, "f-status"),
	}
	selects := ResolveSelectStatements(selectFixtureEntity("post", fields, indexes), db.MYSQLDBType)

	assertNoDuplicateNames(t, selects)

	composite, found := findSelect(selects, "PostBySlugAndTitle")
	require.True(t, found, "the composite index must keep its own name")
	assert.False(t, composite.CombinedIndexes, "a single index is never a combination")

	_, twinFound := findSelect(selects, "PostByTitleAndSlug")
	assert.False(t, twinFound,
		"the {title}+{slug} combination is the composite's query under a permuted name")

	// One column set, one statement.
	byFieldSet := map[string]string{}
	for _, s := range selects {
		key := strings.Join(selectFieldNames(s), ",")
		if prev, dup := byFieldSet[key]; dup {
			t.Errorf("selects %q and %q are the same query over [%s]", prev, s.Name, key)
		}
		byFieldSet[key] = s.Name
	}

	// ...and the dedup must not be so broad that it eats real combinations.
	combos := 0
	for _, s := range selects {
		if s.CombinedIndexes {
			combos++
		}
	}
	assert.Greater(t, combos, 0,
		"combinations that introduce a genuinely new column set must survive")
	_, statusCombo := findSelect(selects, "PostByTitleAndStatus")
	assert.True(t, statusCombo, "{title}+{status} is a new column set and must be emitted")
}

// A hand-modeled UNIQUE/INDEX over exactly the key column resolves to the primary
// select's name. Emitting it a second time makes `sqlc generate` abort on the
// duplicate query name, taking the whole generated app with it.
func TestResolveSelectStatements_PrimaryNameSeeded(t *testing.T) {
	idField := selectFixtureField("f-id", "id", nemgen.FieldType_FIELD_TYPE_UUID)
	idField.Key = true
	idField.Required = true

	e := selectFixtureEntity("widget",
		[]*nemgen.Field{
			idField,
			selectFixtureField("f-label", "label", nemgen.FieldType_FIELD_TYPE_CHAR),
		},
		[]*nemgen.Index{
			selectFixtureIndex("i-id", "uq_widget_id", nemgen.IndexType_INDEX_TYPE_UNIQUE, "f-id"),
		},
	)

	selects := ResolveSelectStatements(e, db.MYSQLDBType)
	assertNoDuplicateNames(t, selects)

	matches := []SchemaSelectStatement{}
	for _, s := range selects {
		if s.Name == "WidgetByID" {
			matches = append(matches, s)
		}
	}
	require.Len(t, matches, 1, "the index over the key column must not re-emit the primary select")
	assert.True(t, matches[0].IsPrimary, "the surviving statement is the primary one")
}

// The mapper emits ACTIVE fields only (nem has no DELETED status; a retired
// column is INACTIVE). A select naming a column that never reached CREATE TABLE
// is a query sqlc cannot parse.
func TestResolveSelectStatements_InactiveIndexMemberDropped(t *testing.T) {
	deleted := selectFixtureField("f-deleted", "deleted_col", nemgen.FieldType_FIELD_TYPE_CHAR)
	deleted.Status = nemgen.FieldStatus_FIELD_STATUS_INACTIVE

	e := selectFixtureEntity("thing",
		[]*nemgen.Field{
			selectFixtureField("f-user", "user_uuid", nemgen.FieldType_FIELD_TYPE_UUID),
			deleted,
		},
		[]*nemgen.Index{
			selectFixtureIndex("i-user-deleted", "idx_thing_user_deleted",
				nemgen.IndexType_INDEX_TYPE_INDEX, "f-user", "f-deleted"),
			// every member unusable => no statement at all
			selectFixtureIndex("i-deleted", "idx_thing_deleted",
				nemgen.IndexType_INDEX_TYPE_INDEX, "f-deleted"),
		},
	)

	selects := ResolveSelectStatements(e, db.MYSQLDBType)

	stmt, found := findSelect(selects, "ThingByUserUUID")
	require.True(t, found)
	assert.Equal(t, []string{"user_uuid"}, selectFieldNames(stmt))

	for _, s := range selects {
		assert.NotContainsf(t, s.Name, "DeletedCol",
			"select %q names a column the mapper does not emit", s.Name)
		assert.NotEmptyf(t, s.Fields, "select %q has an empty WHERE clause", s.Name)
	}
}
