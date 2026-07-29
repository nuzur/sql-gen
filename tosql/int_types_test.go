package tosql

import (
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/stretchr/testify/assert"
)

// Downstream consumers pick a Go type per SQL type by matching the type NAME
// (go-code-gen renders sqlc `overrides:` keyed on db_type, and sqlc has no way to
// discriminate two nuzur field types that share a column type). So a SQL type is
// effectively a channel: if BOOLEAN and INTEGER share one, the override has to be
// either bool or int64, and every field of the losing type generates a project
// that does not compile.
//
// This was live for narrow INTEGERs: MySQL emitted TINYINT(1) at 1-bit and
// TINYINT at 8-bit against BOOLEAN's TINYINT(1), and Postgres emitted BOOLEAN at
// 1-bit against BOOLEAN's BOOLEAN.

func intField(size nemgen.FieldTypeIntegerConfigSize) *nemgen.Field {
	return &nemgen.Field{
		Identifier: "count",
		Type:       nemgen.FieldType_FIELD_TYPE_INTEGER,
		TypeConfig: &nemgen.FieldTypeConfig{
			Integer: &nemgen.FieldTypeIntegerConfig{Size: size},
		},
	}
}

func boolField() *nemgen.Field {
	return &nemgen.Field{
		Identifier: "enabled",
		Type:       nemgen.FieldType_FIELD_TYPE_BOOLEAN,
		TypeConfig: &nemgen.FieldTypeConfig{},
	}
}

var allIntSizes = []nemgen.FieldTypeIntegerConfigSize{
	nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_INVALID,
	nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_ONE_BIT,
	nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_EIGHT_BITS,
	nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTEEN_BITS,
	nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_TWENTY_FOUR_BITS,
	nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS,
	nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTY_FOUR_BITS,
}

// TestIntegerNeverSharesTypeWithBoolean is the regression guard. It is width-
// agnostic on purpose: it does not care WHICH integer type a size maps to, only
// that no size collides with BOOLEAN.
func TestIntegerNeverSharesTypeWithBoolean(t *testing.T) {
	for _, tc := range []struct {
		engine string
		mapper func(*nemgen.Field) string
	}{
		{"mysql", FieldTypeToMYSQL},
		{"postgres", FieldTypeToPG},
	} {
		boolType := tc.mapper(boolField())
		for _, size := range allIntSizes {
			got := tc.mapper(intField(size))
			assert.NotEqualf(t, boolType, got,
				"%s: INTEGER size %v maps to %s, the same SQL type BOOLEAN uses. A name-keyed "+
					"type override cannot then serve bool and int64 at once, so one of the two "+
					"generates code that does not compile.",
				tc.engine, size, got)
		}
	}
}

// TestIntegerSizesToSQL pins the concrete mapping so a width change is a
// deliberate edit rather than an accident. Widening is safe for consumers (the
// domain type is int64 at every width); narrowing truncates stored values.
func TestIntegerSizesToSQL(t *testing.T) {
	for _, tc := range []struct {
		size  nemgen.FieldTypeIntegerConfigSize
		mysql string
		pg    string
	}{
		{nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_INVALID, "INT", "INTEGER"},
		// 1-bit and 8-bit are SMALLINT rather than TINYINT/BOOLEAN precisely to
		// stay off BOOLEAN's type. One extra byte per row.
		{nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_ONE_BIT, "SMALLINT", "SMALLINT"},
		{nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_EIGHT_BITS, "SMALLINT", "SMALLINT"},
		{nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTEEN_BITS, "SMALLINT", "SMALLINT"},
		{nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_TWENTY_FOUR_BITS, "MEDIUMINT", "INTEGER"},
		{nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS, "INT", "INTEGER"},
		{nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTY_FOUR_BITS, "BIGINT", "BIGINT"},
	} {
		assert.Equalf(t, tc.mysql, FieldTypeToMYSQL(intField(tc.size)), "mysql INTEGER size %v", tc.size)
		assert.Equalf(t, tc.pg, FieldTypeToPG(intField(tc.size)), "postgres INTEGER size %v", tc.size)
	}
}

// TestBooleanTypeUnchanged guards the other side of the collision: BOOLEAN keeps
// the type its Go mapping (bool / null.Bool) is pinned to.
func TestBooleanTypeUnchanged(t *testing.T) {
	assert.Equal(t, "TINYINT(1)", FieldTypeToMYSQL(boolField()))
	assert.Equal(t, "BOOLEAN", FieldTypeToPG(boolField()))
}
