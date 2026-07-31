package fromsql

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"

	"github.com/gofrs/uuid"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/nuzur/sql-gen/tosql"
)

// TestMapMysqlDecimalCarriesScale: a decimal column that comes back with no
// number_of_decimals renders at the default scale on the way out, so the differ
// proposes the same MODIFY COLUMN on every plan, forever.
func TestMapMysqlDecimalCarriesScale(t *testing.T) {
	in := &mysqlColumnDetails{
		Name: "price_per_kg", DataType: "decimal", ColumnType: "decimal(38,2)",
		NumericPrecision: ptrInt64(38), NumericScale: ptrInt64(2),
	}
	got, config := mapMysqlColumnDataTypeToFieldType(in, remoteRows{})
	if got != nemgen.FieldType_FIELD_TYPE_DECIMAL {
		t.Fatalf("got %v, want DECIMAL", got)
	}
	if n := config.GetDecimal().GetNumberOfDecimals(); n != 2 {
		t.Errorf("number_of_decimals = %d, want 2 — the scale is lost on the round trip", n)
	}
}

// mysql information_schema rows for the table sql-gen generates from a model
// with a decimal, a text index, a blob index and a FULLTEXT index. Captured
// verbatim from a stock mysql:8 container after applying the generated
// create.sql, so these are what MySQL really reports rather than what we expect
// it to.
func lotColumns() []*mysqlColumnDetails {
	return []*mysqlColumnDetails{
		{Name: "id", DataType: "char", ColumnType: "char(36)", ColumnKey: "PRI", IsNullable: "NO", CharMax: ptrInt64(36)},
		{Name: "price_per_kg", DataType: "decimal", ColumnType: "decimal(38,2)", IsNullable: "YES", NumericPrecision: ptrInt64(38), NumericScale: ptrInt64(2)},
		{Name: "weight_kg", DataType: "decimal", ColumnType: "decimal(38,9)", IsNullable: "YES", NumericPrecision: ptrInt64(38), NumericScale: ptrInt64(9)},
		// A non-default-width NOT NULL varchar with data in it: the shape that
		// churned in production. Sampling has to leave the width alone.
		{Name: "lot_code", DataType: "varchar", ColumnType: "varchar(64)", IsNullable: "NO", CharMax: ptrInt64(64)},
		{Name: "warehouse_bin", DataType: "varchar", ColumnType: "varchar(24)", IsNullable: "YES", CharMax: ptrInt64(24)},
		// Default-width varchars carrying a url and an address: these promote,
		// because a url/email field renders at exactly 512 on mysql.
		{Name: "sample_report", DataType: "varchar", ColumnType: "varchar(512)", IsNullable: "YES", CharMax: ptrInt64(512)},
		{Name: "contact_email", DataType: "varchar", ColumnType: "varchar(512)", IsNullable: "YES", CharMax: ptrInt64(512)},
		{Name: "body", DataType: "text", ColumnType: "text", ColumnKey: "MUL", IsNullable: "YES", CharMax: ptrInt64(65535)},
		{Name: "doc", DataType: "blob", ColumnType: "blob", ColumnKey: "MUL", IsNullable: "YES", CharMax: ptrInt64(65535)},
		{Name: "description", DataType: "text", ColumnType: "text", ColumnKey: "MUL", IsNullable: "YES", CharMax: ptrInt64(65535)},
	}
}

// lotSample is one row of sample data as the introspection really receives it.
// The values are Go strings, not []byte: a MySQL introspection that goes
// through a local agent gets them that way, because the agent collapses []byte
// to string before the row crosses the stream (nuzur-cli/agent/handlers.go,
// encodeValue). Sample-driven type promotion only looks at string values, so
// this — not the direct sqlx path — is where it fires.
func lotSample() remoteRows {
	return remoteRows{{
		"id":            "11111111-1111-4111-8111-111111111111",
		"lot_code":      "LOT-2026-0001",
		"warehouse_bin": "A-14-3",
		"sample_report": "https://example.com/reports/lot-2026-0001.pdf",
		"contact_email": "ada@example.com",
		"body":          "chocolate, red fruit",
		"price_per_kg":  "12.50",
	}}
}

func lotIndexes() [][]*mysqlIndexDetails {
	return [][]*mysqlIndexDetails{
		{{Name: "PRIMARY", Seq: 1, ColumnName: "id", Collation: nullString("A"), IndexType: "BTREE", ConstraintType: "PRIMARY KEY"}},
		// A FULLTEXT index has no TABLE_CONSTRAINTS row, so CONSTRAINT_TYPE is
		// the "INDEX" fallback — INDEX_TYPE is the only thing that identifies it.
		{{Name: "ft_lot_description", Seq: 1, NonUnique: true, ColumnName: "description", IndexType: "FULLTEXT", ConstraintType: "INDEX"}},
		{{Name: "idx_body", Seq: 1, NonUnique: true, ColumnName: "body", Collation: nullString("A"), IndexType: "BTREE", SubPart: nullInt64(255), ConstraintType: "INDEX"}},
		{{Name: "idx_doc", Seq: 1, NonUnique: true, ColumnName: "doc", Collation: nullString("A"), IndexType: "BTREE", SubPart: nullInt64(255), ConstraintType: "INDEX"}},
	}
}

// introspectedLot reassembles the entity buildEntityFromMysql would produce from
// those rows, without needing a live connection.
func introspectedLot(t *testing.T) *nemgen.Entity {
	t.Helper()

	sample := lotSample()
	fields := []*nemgen.Field{}
	for _, c := range lotColumns() {
		fields = append(fields, mapMysqlColumnDetailsToField(c, sample))
	}

	indexes := []*nemgen.Index{}
	for _, group := range lotIndexes() {
		if i := mapMysqlIndexDetailsToIndex(group, fields); i != nil {
			indexes = append(indexes, i)
		}
	}

	return &nemgen.Entity{
		Uuid:       uuid.Must(uuid.NewV4()).String(),
		Identifier: "lot",
		Fields:     fields,
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Status:     nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{Indexes: indexes},
		},
	}
}

func TestMysqlIntrospectionKeepsIndexKindAndPrefix(t *testing.T) {
	e := introspectedLot(t)

	byName := map[string]*nemgen.Index{}
	for _, i := range e.TypeConfig.Standalone.Indexes {
		byName[i.Identifier] = i
	}

	if got := byName["ft_lot_description"].Type; got != nemgen.IndexType_INDEX_TYPE_FULLTEXT {
		t.Errorf("ft_lot_description came back as %v — a live FULLTEXT index reconstructed as a plain one is a DROP KEY / ADD FULLTEXT KEY on every plan", got)
	}
	// FULLTEXT covers whole columns and always reports SUB_PART NULL.
	if got := byName["ft_lot_description"].Fields[0].Length; got != 0 {
		t.Errorf("fulltext index field length = %d, want 0", got)
	}
	if got := byName["idx_body"].Fields[0].Length; got != 255 {
		t.Errorf("idx_body prefix = %d, want the 255 SUB_PART reports", got)
	}
	if got := byName["idx_doc"].Fields[0].Length; got != 255 {
		t.Errorf("idx_doc prefix = %d, want 255", got)
	}
	if got := byName["PRIMARY"].Type; got != nemgen.IndexType_INDEX_TYPE_PRIMARY {
		t.Errorf("PRIMARY came back as %v", got)
	}
}

// TestMysqlIntrospectionIsAFixedPoint is the convergence guard: rendering the
// introspected schema back to DDL must reproduce the DDL that created the
// database. Anything the round trip cannot preserve shows up here as a diff —
// which in production is a statement the MySQL plan proposes on every deploy and
// that never makes the next plan any shorter.
//
// The expected DDL is the exact create.sql that was applied to the mysql:8
// container these information_schema rows were read from.
func TestMysqlIntrospectionIsAFixedPoint(t *testing.T) {
	const want = "CREATE TABLE IF NOT EXISTS `lot` (\n" +
		"    `id` CHAR(36) NOT NULL,\n" +
		"    `price_per_kg` DECIMAL(38,2),\n" +
		"    `weight_kg` DECIMAL(38,9),\n" +
		"    `lot_code` VARCHAR(64) NOT NULL,\n" +
		"    `warehouse_bin` VARCHAR(24),\n" +
		"    `sample_report` VARCHAR(512),\n" +
		"    `contact_email` VARCHAR(512),\n" +
		"    `body` TEXT,\n" +
		"    `doc` BLOB,\n" +
		"    `description` TEXT,\n" +
		"    PRIMARY KEY (`id`),\n" +
		"    INDEX `idx_body` (`body`(255)),\n" +
		"    INDEX `idx_doc` (`doc`(255)),\n" +
		"    FULLTEXT INDEX `ft_lot_description` (`description`)\n" +
		") ENGINE = InnoDB;\n\n"

	if got := renderCreateSQL(t, introspectedLot(t), db.MYSQLDBType); got != want {
		t.Errorf("re-rendered DDL differs from the DDL that created the database\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestMysqlVarcharSamplingKeepsWidth is the finding-11 varchar guard.
//
// Sample-based promotion re-types a varchar as an email or a url field, and
// those render at a fixed width (512 on mysql) because they carry no max_size.
// Promoting a varchar(200) therefore rewrote it to varchar(512), and since the
// "existing" side of a MySQL diff IS this reconstruction re-rendered as DDL,
// that produced `MODIFY COLUMN name varchar(200) NOT NULL` on every plan —
// applied, and back again on the next plan.
func TestMysqlVarcharSamplingKeepsWidth(t *testing.T) {
	cases := []struct {
		name      string
		column    string
		width     int64
		sample    string
		wantType  nemgen.FieldType
		wantWidth int64 // only meaningful for VARCHAR
	}{
		// url.Parse accepts any of these as a relative reference, which is what
		// made every populated text column look like a url.
		{"a person's name", "full_name", 160, "Ada Lovelace", nemgen.FieldType_FIELD_TYPE_VARCHAR, 160},
		{"a bcrypt hash", "password_hash", 255, "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", nemgen.FieldType_FIELD_TYPE_VARCHAR, 255},
		{"a lot code", "lot_code", 64, "LOT-2026-0001", nemgen.FieldType_FIELD_TYPE_VARCHAR, 64},
		{"a slug", "slug", 220, "finca-la-ilusion", nemgen.FieldType_FIELD_TYPE_VARCHAR, 220},
		// A real url in a column too narrow for the url field's fixed width:
		// promoting would widen it, so it stays a varchar of its real size.
		{"a url in a narrow column", "website", 120, "https://example.com/a", nemgen.FieldType_FIELD_TYPE_VARCHAR, 120},
		{"an address in a narrow column", "contact_email", 160, "ada@example.com", nemgen.FieldType_FIELD_TYPE_VARCHAR, 160},
		// At the width the promoted type renders, promotion is a no-op for the
		// DDL and the modelling nicety is kept.
		{"a url at the url width", "website", 512, "https://example.com/a", nemgen.FieldType_FIELD_TYPE_URL, 0},
		{"an address at the email width", "email", 512, "ada@example.com", nemgen.FieldType_FIELD_TYPE_EMAIL, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &mysqlColumnDetails{
				Name: tc.column, DataType: "varchar",
				ColumnType: "varchar(" + strconv.FormatInt(tc.width, 10) + ")",
				IsNullable: "NO", CharMax: ptrInt64(tc.width),
			}
			got, config := mapMysqlColumnDataTypeToFieldType(in, remoteRows{{tc.column: tc.sample}})
			if got != tc.wantType {
				t.Fatalf("%s sampled as %q: got %v, want %v", tc.column, tc.sample, got, tc.wantType)
			}
			if tc.wantType != nemgen.FieldType_FIELD_TYPE_VARCHAR {
				return
			}
			if w := config.GetVarchar().GetMaxSize(); w != tc.wantWidth {
				t.Errorf("max_size = %d, want %d — the width is rewritten, which is a MODIFY COLUMN on every plan", w, tc.wantWidth)
			}
		})
	}
}

// TestIsURLNeedsSchemeAndHost pins the predicate itself: url.Parse alone accepts
// almost every string, so isURL used to be true for any populated text column.
func TestIsURLNeedsSchemeAndHost(t *testing.T) {
	urls := []string{"https://example.com", "http://example.com/a?b=c", "ftp://host/f"}
	notURLs := []string{"Ada Lovelace", "LOT-2026-0001", "$2a$10$abcdef", "finca-la-ilusion", "12.50", ""}

	for _, v := range urls {
		if !(remoteRows{{"col": v}}).isURL("col") {
			t.Errorf("isURL(%q) = false, want true", v)
		}
	}
	for _, v := range notURLs {
		if (remoteRows{{"col": v}}).isURL("col") {
			t.Errorf("isURL(%q) = true — that promotes an ordinary text column to a url field and rewrites its width", v)
		}
	}
}

// renderCreateSQL runs the full generator over one entity and returns the create
// statement, which is the artifact the MySQL diff actually compares.
func renderCreateSQL(t *testing.T, e *nemgen.Entity, dbType db.DBType) string {
	t.Helper()

	pv := &nemgen.ProjectVersion{Entities: []*nemgen.Entity{e}}
	res, err := tosql.GenerateSQL(context.Background(), tosql.GenerateRequest{
		ExecutionUUID:  uuid.Must(uuid.NewV4()).String(),
		ProjectVersion: pv,
		Configvalues: &tosql.ConfigValues{
			DBType:   dbType,
			Entities: []string{e.Uuid},
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

func nullString(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }
func nullInt64(v int64) sql.NullInt64    { return sql.NullInt64{Int64: v, Valid: true} }
