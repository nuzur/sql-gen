package fromsql

import (
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
)

// ptrInt64 is a small helper for the CharMax field.
func ptrInt64(v int64) *int64 { return &v }

// TestMapPgColumnDataTypeToFieldType_InformationSchemaSpellings guards the exact
// data_type strings Postgres' information_schema.columns reports. Regression for
// the bug where "character" / "double precision" / "numeric" columns fell through
// to FIELD_TYPE_INVALID (and were dropped from the introspected schema while their
// indexes survived, producing invalid DDL on replay).
func TestMapPgColumnDataTypeToFieldType_InformationSchemaSpellings(t *testing.T) {
	cases := []struct {
		name     string
		dataType string
		charMax  *int64
		want     nemgen.FieldType
	}{
		{"fixed char", "character", ptrInt64(10), nemgen.FieldType_FIELD_TYPE_CHAR},
		{"bpchar alias", "bpchar", ptrInt64(10), nemgen.FieldType_FIELD_TYPE_CHAR},
		{"double precision", "double precision", nil, nemgen.FieldType_FIELD_TYPE_FLOAT},
		{"real", "real", nil, nemgen.FieldType_FIELD_TYPE_FLOAT},
		{"numeric", "numeric", nil, nemgen.FieldType_FIELD_TYPE_DECIMAL},
		{"varchar", "character varying", ptrInt64(255), nemgen.FieldType_FIELD_TYPE_VARCHAR},
		{"integer", "integer", nil, nemgen.FieldType_FIELD_TYPE_INTEGER},
		{"uuid", "uuid", nil, nemgen.FieldType_FIELD_TYPE_UUID},
		{"jsonb", "jsonb", nil, nemgen.FieldType_FIELD_TYPE_JSON},
		{"timestamp tz", "timestamp with time zone", nil, nemgen.FieldType_FIELD_TYPE_DATETIME},
		{"time without tz", "time without time zone", nil, nemgen.FieldType_FIELD_TYPE_TIME},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &pgColumnDetails{Name: "col", DataType: tc.dataType, CharMax: tc.charMax}
			got, _ := mapPgColumnDataTypeToFieldType(in, remoteRows{})
			if got == nemgen.FieldType_FIELD_TYPE_INVALID {
				t.Fatalf("data_type %q mapped to INVALID (column would be dropped)", tc.dataType)
			}
			if got != tc.want {
				t.Errorf("data_type %q: got %v, want %v", tc.dataType, got, tc.want)
			}
		})
	}
}

// TestMapPgColumnDataTypeToFieldType_Char36UUIDPromotion confirms a char(36)
// column whose sampled values look like UUIDs is promoted to a UUID field, while
// a char(36) with no UUID samples stays CHAR — both via the "character" spelling.
func TestMapPgColumnDataTypeToFieldType_Char36UUIDPromotion(t *testing.T) {
	in := &pgColumnDetails{Name: "user_id", DataType: "character", CharMax: ptrInt64(36)}

	sample := remoteRows{{"user_id": "3b6b1f2e-1c2d-4a5b-8c9d-0e1f2a3b4c5d"}}
	if got, _ := mapPgColumnDataTypeToFieldType(in, sample); got != nemgen.FieldType_FIELD_TYPE_UUID {
		t.Errorf("char(36) with UUID sample: got %v, want UUID", got)
	}

	if got, _ := mapPgColumnDataTypeToFieldType(in, remoteRows{}); got != nemgen.FieldType_FIELD_TYPE_CHAR {
		t.Errorf("char(36) with no sample: got %v, want CHAR", got)
	}
}

// TestMapPgVarcharSamplingKeepsWidth is the Postgres half of the finding-11
// varchar guard. Same rule as MySQL — a promotion may not change the width the
// column renders at — but different widths: Postgres renders a url field as
// VARCHAR(2048) and an email as VARCHAR(512), so a varchar(512) of urls that
// promotes on MySQL must NOT promote here.
func TestMapPgVarcharSamplingKeepsWidth(t *testing.T) {
	cases := []struct {
		name      string
		column    string
		width     int64
		sample    string
		wantType  nemgen.FieldType
		wantWidth int64
	}{
		{"name is not a url", "full_name", 160, "Ada Lovelace", nemgen.FieldType_FIELD_TYPE_VARCHAR, 160},
		{"url below the pg url width", "website", 512, "https://example.com/a", nemgen.FieldType_FIELD_TYPE_VARCHAR, 512},
		{"url at the pg url width", "website", 2048, "https://example.com/a", nemgen.FieldType_FIELD_TYPE_URL, 0},
		{"address below the email width", "contact_email", 160, "ada@example.com", nemgen.FieldType_FIELD_TYPE_VARCHAR, 160},
		{"address at the email width", "email", 512, "ada@example.com", nemgen.FieldType_FIELD_TYPE_EMAIL, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &pgColumnDetails{Name: tc.column, DataType: "character varying", CharMax: ptrInt64(tc.width)}
			got, config := mapPgColumnDataTypeToFieldType(in, remoteRows{{tc.column: tc.sample}})
			if got != tc.wantType {
				t.Fatalf("%s sampled as %q: got %v, want %v", tc.column, tc.sample, got, tc.wantType)
			}
			if tc.wantType != nemgen.FieldType_FIELD_TYPE_VARCHAR {
				return
			}
			if w := config.GetVarchar().GetMaxSize(); w != tc.wantWidth {
				t.Errorf("max_size = %d, want %d — the reconstruction rewrites the width", w, tc.wantWidth)
			}
		})
	}
}
