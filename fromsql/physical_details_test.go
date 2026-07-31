package fromsql

import (
	"context"
	"os"
	"testing"

	"github.com/gofrs/uuid"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/nuzur/sql-gen/tosql"
)

// The physical-SQL details the model gained: datetime fractional precision,
// column defaults, mysql's ON UPDATE CURRENT_TIMESTAMP, and foreign key
// referential actions. Each has to survive introspection, or the schema plan
// proposes the same change on every deploy and never converges (bug 11).
//
// Every information_schema row below was captured verbatim from stock mysql:8
// and postgres:16 containers after applying the generated DDL the tests compare
// against — so these are what the engines really report, not what we hope they
// report.

const mysqlPhysicalDetailsDDL = "CREATE TABLE IF NOT EXISTS `parent` (\n" +
	"    `id` CHAR(36) NOT NULL,\n" +
	"    `name` VARCHAR(100),\n" +
	"    PRIMARY KEY (`id`)\n" +
	") ENGINE = InnoDB;\n\n" +
	"CREATE TABLE IF NOT EXISTS `child` (\n" +
	"    `id` CHAR(36) NOT NULL,\n" +
	"    `parent_uuid` CHAR(36),\n" +
	"    `status` VARCHAR(20) DEFAULT 'active',\n" +
	"    `qty` INT DEFAULT 5,\n" +
	"    `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),\n" +
	"    `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),\n" +
	"    `plain_ts` DATETIME,\n" +
	"    PRIMARY KEY (`id`),\n" +
	"    CONSTRAINT `fk_child_parent`\n" +
	"        FOREIGN KEY (`parent_uuid`)\n" +
	"        REFERENCES `parent` (`id`)\n" +
	"        ON DELETE SET NULL\n" +
	"        ON UPDATE CASCADE\n" +
	") ENGINE = InnoDB;\n\n"

func mysqlChildColumns() []*mysqlColumnDetails {
	return []*mysqlColumnDetails{
		{Name: "id", DataType: "char", ColumnType: "char(36)", ColumnKey: "PRI", IsNullable: "NO", CharMax: ptrInt64(36)},
		// COLUMN_KEY is MUL because of the index mysql creates for the foreign key.
		{Name: "parent_uuid", DataType: "char", ColumnType: "char(36)", ColumnKey: "MUL", IsNullable: "YES", CharMax: ptrInt64(36)},
		{Name: "status", DataType: "varchar", ColumnType: "varchar(20)", IsNullable: "YES", CharMax: ptrInt64(20), DefaultValue: ptrString("active")},
		{Name: "qty", DataType: "int", ColumnType: "int", IsNullable: "YES", DefaultValue: ptrString("5")},
		{Name: "created_at", DataType: "datetime", ColumnType: "datetime(3)", IsNullable: "YES",
			DefaultValue: ptrString("CURRENT_TIMESTAMP(3)"), DatetimePrecision: ptrInt64(3), Extra: "DEFAULT_GENERATED"},
		{Name: "updated_at", DataType: "datetime", ColumnType: "datetime(3)", IsNullable: "YES",
			DefaultValue: ptrString("CURRENT_TIMESTAMP(3)"), DatetimePrecision: ptrInt64(3),
			Extra: "DEFAULT_GENERATED on update CURRENT_TIMESTAMP(3)"},
		// No default at all: mysql reports COLUMN_DEFAULT NULL and fsp 0.
		{Name: "plain_ts", DataType: "datetime", ColumnType: "datetime", IsNullable: "YES", DatetimePrecision: ptrInt64(0)},
	}
}

func mysqlParentColumns() []*mysqlColumnDetails {
	return []*mysqlColumnDetails{
		{Name: "id", DataType: "char", ColumnType: "char(36)", ColumnKey: "PRI", IsNullable: "NO", CharMax: ptrInt64(36)},
		{Name: "name", DataType: "varchar", ColumnType: "varchar(100)", IsNullable: "YES", CharMax: ptrInt64(100)},
	}
}

// TestMysqlIntrospectionKeepsPhysicalDetails checks the individual values before
// the whole-DDL comparison, so a failure says which detail was lost.
func TestMysqlIntrospectionKeepsPhysicalDetails(t *testing.T) {
	byName := map[string]*nemgen.Field{}
	for _, c := range mysqlChildColumns() {
		f := mapMysqlColumnDetailsToField(c, remoteRows{})
		byName[f.Identifier] = f
	}

	if got := byName["status"].GetDefaultValue(); got != "active" {
		t.Errorf("status default = %q, want \"active\" — mysql reports a literal unquoted", got)
	}
	if byName["status"].GetDefaultValueIsExpression() {
		t.Error("status default came back as an expression; it is a literal")
	}
	if got := byName["qty"].GetDefaultValue(); got != "5" {
		t.Errorf("qty default = %q, want \"5\"", got)
	}
	if got := byName["created_at"].GetTypeConfig().GetDatetime().GetPrecision(); got != 3 {
		t.Errorf("created_at precision = %d, want 3 — DATETIME_PRECISION is the only place it lives", got)
	}
	if !byName["created_at"].GetDefaultValueIsExpression() {
		t.Error("CURRENT_TIMESTAMP(3) must come back as an expression, not a string literal")
	}
	if !byName["updated_at"].GetTypeConfig().GetDatetime().GetOnUpdateCurrentTimestamp() {
		t.Error("ON UPDATE CURRENT_TIMESTAMP lost — EXTRA is the only place information_schema reports it")
	}
	if !byName["plain_ts"].GetTypeConfig().GetDatetime().GetNoDefaultCurrentTimestamp() {
		t.Error("a datetime column with no default must say so, or re-rendering invents DEFAULT CURRENT_TIMESTAMP")
	}
	if got := byName["plain_ts"].GetTypeConfig().GetDatetime().GetPrecision(); got != 0 {
		t.Errorf("plain_ts precision = %d, want 0 (mysql's default fsp, rendered bare)", got)
	}
}

func TestMysqlIntrospectionKeepsReferentialActions(t *testing.T) {
	rel := introspectedMysqlSchema(t).Relationships[0]
	if got := rel.GetOnDelete(); got != nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_SET_NULL {
		t.Errorf("on_delete = %v, want SET_NULL", got)
	}
	if got := rel.GetOnUpdate(); got != nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_CASCADE {
		t.Errorf("on_update = %v, want CASCADE", got)
	}
}

// A foreign key created without an explicit clause is reported as NO ACTION by
// both engines. That has to map back to unset: rendering ON DELETE NO ACTION
// would leave the generated DDL permanently different from what introspection
// reconstructs, which is a constraint the plan drops and recreates forever.
func TestNoActionMapsToUnsetSoExistingForeignKeysDoNotChurn(t *testing.T) {
	fk := &mysqlForeignKeyDetails{
		ConstraintName: "fk", ColumnName: "parent_uuid",
		ReferencedColumnName: "id", ReferencedTableName: "parent",
		DeleteRule: "NO ACTION", UpdateRule: "NO ACTION",
	}
	entities := introspectedMysqlSchema(t).Entities
	rel := mapMysqlFKDetailsToRelationship(fk, "child", entities)
	if rel.GetOnDelete() != nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_INVALID ||
		rel.GetOnUpdate() != nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_INVALID {
		t.Fatalf("NO ACTION must map to unset, got on_delete=%v on_update=%v", rel.GetOnDelete(), rel.GetOnUpdate())
	}
	if got := (tosql.SchemaConstraint{Relationship: rel}).ReferentialActions(); got != "" {
		t.Errorf("unset actions rendered %q, want no clause at all", got)
	}
}

// TestMysqlPhysicalDetailsAreAFixedPoint is the convergence guard: rendering the
// introspected schema back to DDL has to reproduce the DDL that created the
// database, precision, defaults, ON UPDATE and referential actions included.
func TestMysqlPhysicalDetailsAreAFixedPoint(t *testing.T) {
	if got := renderCreateSQLForPV(t, introspectedMysqlSchema(t), db.MYSQLDBType); got != mysqlPhysicalDetailsDDL {
		t.Errorf("re-rendered DDL differs from the DDL that created the database\n got:\n%s\nwant:\n%s",
			got, mysqlPhysicalDetailsDDL)
	}
}

// introspectedMysqlSchema reassembles what buildProjectVersionFromMysql produces
// from the captured rows, without needing a live connection. The index mysql
// creates to support the foreign key is deliberately absent: buildIndexesFromMysql
// drops an index named after a foreign key constraint, because it is not in the
// DDL and the relationship already implies it.
func introspectedMysqlSchema(t *testing.T) *nemgen.ProjectVersion {
	t.Helper()

	build := func(name string, columns []*mysqlColumnDetails, indexes [][]*mysqlIndexDetails) *nemgen.Entity {
		fields := []*nemgen.Field{}
		for _, c := range columns {
			fields = append(fields, mapMysqlColumnDetailsToField(c, remoteRows{}))
		}
		idxs := []*nemgen.Index{}
		for _, group := range indexes {
			if i := mapMysqlIndexDetailsToIndex(group, fields); i != nil {
				idxs = append(idxs, i)
			}
		}
		return &nemgen.Entity{
			Uuid:       uuid.Must(uuid.NewV4()).String(),
			Identifier: name,
			Fields:     fields,
			Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
			Status:     nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
			TypeConfig: &nemgen.EntityTypeConfig{
				Standalone: &nemgen.EntityTypeStandaloneConfig{Indexes: idxs},
			},
		}
	}

	primary := func(column string) [][]*mysqlIndexDetails {
		return [][]*mysqlIndexDetails{{{
			Name: "PRIMARY", Seq: 1, ColumnName: column,
			Collation: nullString("A"), IndexType: "BTREE", ConstraintType: "PRIMARY KEY",
		}}}
	}

	// buildProjectVersionFromMysql sorts entities by identifier.
	entities := []*nemgen.Entity{
		build("child", mysqlChildColumns(), primary("id")),
		build("parent", mysqlParentColumns(), primary("id")),
	}

	rel := mapMysqlFKDetailsToRelationship(&mysqlForeignKeyDetails{
		ConstraintName: "fk_child_parent", ColumnName: "parent_uuid",
		ReferencedColumnName: "id", ReferencedTableName: "parent",
		DeleteRule: "SET NULL", UpdateRule: "CASCADE",
	}, "child", entities)

	return &nemgen.ProjectVersion{Entities: entities, Relationships: []*nemgen.Relationship{rel}}
}

// ── postgres ────────────────────────────────────────────────────────────────

const pgPhysicalDetailsDDL = "CREATE TABLE IF NOT EXISTS \"parent\" (\n" +
	"    \"id\" CHAR(36) NOT NULL,\n" +
	"    \"name\" VARCHAR(100),\n" +
	"    PRIMARY KEY (\"id\")\n" +
	");\n\n" +
	"CREATE TABLE IF NOT EXISTS \"child\" (\n" +
	"    \"id\" CHAR(36) NOT NULL,\n" +
	"    \"parent_uuid\" CHAR(36),\n" +
	"    \"status\" VARCHAR(20) DEFAULT 'active',\n" +
	"    \"qty\" INTEGER DEFAULT 5,\n" +
	"    \"created_at\" TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP,\n" +
	"    \"updated_at\" TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP,\n" +
	"    \"plain_ts\" TIMESTAMP,\n" +
	"    PRIMARY KEY (\"id\"),\n" +
	"    CONSTRAINT \"fk_child_parent\"\n" +
	"        FOREIGN KEY (\"parent_uuid\")\n" +
	"        REFERENCES \"parent\" (\"id\")\n" +
	"        ON DELETE SET NULL\n" +
	"        ON UPDATE CASCADE\n" +
	");\n\n"

func pgChildColumns() []*pgColumnDetails {
	return []*pgColumnDetails{
		{Name: "id", DataType: "character", IsNullable: "NO", CharMax: ptrInt64(36)},
		{Name: "parent_uuid", DataType: "character", IsNullable: "YES", CharMax: ptrInt64(36)},
		// Postgres reports a literal default already cast to the column's type.
		{Name: "status", DataType: "character varying", IsNullable: "YES", CharMax: ptrInt64(20),
			DefaultValue: ptrString("'active'::character varying")},
		{Name: "qty", DataType: "integer", IsNullable: "YES", DefaultValue: ptrString("5")},
		{Name: "created_at", DataType: "timestamp without time zone", IsNullable: "YES",
			DefaultValue: ptrString("CURRENT_TIMESTAMP"), DatetimePrecision: ptrInt64(3)},
		{Name: "updated_at", DataType: "timestamp without time zone", IsNullable: "YES",
			DefaultValue: ptrString("CURRENT_TIMESTAMP"), DatetimePrecision: ptrInt64(3)},
		// A bare TIMESTAMP is reported with precision 6, which is postgres' own
		// default and folds back to unset.
		{Name: "plain_ts", DataType: "timestamp without time zone", IsNullable: "YES", DatetimePrecision: ptrInt64(6)},
	}
}

func pgParentColumns() []*pgColumnDetails {
	return []*pgColumnDetails{
		{Name: "id", DataType: "character", IsNullable: "NO", CharMax: ptrInt64(36)},
		{Name: "name", DataType: "character varying", IsNullable: "YES", CharMax: ptrInt64(100)},
	}
}

func TestPgIntrospectionKeepsPhysicalDetails(t *testing.T) {
	byName := map[string]*nemgen.Field{}
	for _, c := range pgChildColumns() {
		f := mapPgColumnDetailsToField(c, remoteRows{}, nil)
		byName[f.Identifier] = f
	}

	if got := byName["status"].GetDefaultValue(); got != "active" {
		t.Errorf("status default = %q, want \"active\" — the ::type cast and quotes have to come off", got)
	}
	if byName["status"].GetDefaultValueIsExpression() {
		t.Error("a quoted literal must not come back as an expression")
	}
	if got := byName["qty"].GetDefaultValue(); got != "5" {
		t.Errorf("qty default = %q, want \"5\"", got)
	}
	if !byName["created_at"].GetDefaultValueIsExpression() {
		t.Error("CURRENT_TIMESTAMP must come back as an expression")
	}
	if got := byName["created_at"].GetTypeConfig().GetDatetime().GetPrecision(); got != 3 {
		t.Errorf("created_at precision = %d, want 3", got)
	}
	if got := byName["plain_ts"].GetTypeConfig().GetDatetime().GetPrecision(); got != 0 {
		t.Errorf("plain_ts precision = %d, want 0 — postgres reports its own default of 6, which renders bare", got)
	}
	if !byName["plain_ts"].GetTypeConfig().GetDatetime().GetNoDefaultCurrentTimestamp() {
		t.Error("a timestamp column with no default must say so")
	}
}

func TestPgDefaultNormalization(t *testing.T) {
	for _, tc := range []struct {
		raw          string
		wantValue    string
		wantIsExpr   bool
		wantHasValue bool
	}{
		{"'active'::character varying", "active", false, true},
		{"'it''s'::text", "it's", false, true},
		{"'plain'", "plain", false, true},
		{"5", "5", false, true},
		{"-1.5", "-1.5", false, true},
		{"true", "true", false, true},
		{"CURRENT_TIMESTAMP", "CURRENT_TIMESTAMP", true, true},
		{"now()", "now()", true, true},
		{"nextval('child_id_seq'::regclass)", "nextval('child_id_seq'::regclass)", true, true},
	} {
		got := pgColumnDefault(&tc.raw)
		if got.Value != tc.wantValue || got.IsExpression != tc.wantIsExpr || got.Present != tc.wantHasValue {
			t.Errorf("pgColumnDefault(%q) = %+v, want value=%q expr=%v present=%v",
				tc.raw, got, tc.wantValue, tc.wantIsExpr, tc.wantHasValue)
		}
	}
	if got := pgColumnDefault(nil); got.Present {
		t.Errorf("a NULL column_default must be no default at all, got %+v", got)
	}
}

func TestPgPhysicalDetailsAreAFixedPoint(t *testing.T) {
	if got := renderCreateSQLForPV(t, introspectedPgSchema(t), db.PGDBType); got != pgPhysicalDetailsDDL {
		t.Errorf("re-rendered DDL differs from the DDL that created the database\n got:\n%s\nwant:\n%s",
			got, pgPhysicalDetailsDDL)
	}
}

func introspectedPgSchema(t *testing.T) *nemgen.ProjectVersion {
	t.Helper()

	build := func(name string, columns []*pgColumnDetails, indexes []*pgIndexDetails) *nemgen.Entity {
		fields := []*nemgen.Field{}
		for _, c := range columns {
			fields = append(fields, mapPgColumnDetailsToField(c, remoteRows{}, indexes))
		}
		idxs := []*nemgen.Index{}
		if i := mapPgIndexDetailsToIndex(indexes, fields); i != nil {
			idxs = append(idxs, i)
		}
		return &nemgen.Entity{
			Uuid:       uuid.Must(uuid.NewV4()).String(),
			Identifier: name,
			Fields:     fields,
			Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
			Status:     nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
			TypeConfig: &nemgen.EntityTypeConfig{
				Standalone: &nemgen.EntityTypeStandaloneConfig{Indexes: idxs},
			},
		}
	}

	pkey := func(name string) []*pgIndexDetails {
		return []*pgIndexDetails{{Name: name, Seq: 1, ColumnName: "id", IsKey: true, IsUnique: true, Ascending: true}}
	}

	entities := []*nemgen.Entity{
		build("child", pgChildColumns(), pkey("child_pkey")),
		build("parent", pgParentColumns(), pkey("parent_pkey")),
	}

	rel := mapPgFKDetailsToRelationship(&pgForeignKeyDetails{
		ConstraintName: "fk_child_parent", ColumnName: "parent_uuid",
		ReferencedColumnName: "id", ReferencedTableName: "parent",
		DeleteRule: "SET NULL", UpdateRule: "CASCADE",
	}, "child", entities)

	return &nemgen.ProjectVersion{Entities: entities, Relationships: []*nemgen.Relationship{rel}}
}

// renderCreateSQLForPV runs the full generator over a whole project version —
// the multi-entity form of renderCreateSQL, needed because a foreign key only
// renders when both of its endpoints are present.
func renderCreateSQLForPV(t *testing.T, pv *nemgen.ProjectVersion, dbType db.DBType) string {
	t.Helper()

	entities := []string{}
	for _, e := range pv.Entities {
		entities = append(entities, e.Uuid)
	}
	res, err := tosql.GenerateSQL(context.Background(), tosql.GenerateRequest{
		ExecutionUUID:  uuid.Must(uuid.NewV4()).String(),
		ProjectVersion: pv,
		Configvalues: &tosql.ConfigValues{
			DBType:   dbType,
			Entities: entities,
			Actions:  []tosql.Action{tosql.CreateAction},
		},
	})
	if err != nil {
		t.Fatalf("generating sql: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(res.WorkingDir)
		os.RemoveAll(res.ZipFile)
	})

	for _, r := range res.Results {
		if r.Action == tosql.CreateAction {
			return r.Data
		}
	}
	t.Fatal("no create action in the result")
	return ""
}

func ptrString(v string) *string { return &v }
