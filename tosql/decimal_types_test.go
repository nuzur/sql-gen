package tosql

import (
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/stretchr/testify/assert"
)

// A bare DECIMAL means two different things: arbitrary precision on postgres,
// DECIMAL(10,0) on mysql. So the same model stored 12.75 exactly on one engine
// and 13 on the other, on the field type the platform recommends for money —
// silently, at write time, unrecoverably. Both engines must therefore render an
// explicit precision and scale, and the same ones.

func decimalField(config *nemgen.FieldTypeDecimalConfig) *nemgen.Field {
	return &nemgen.Field{
		Identifier: "price_per_kg",
		Type:       nemgen.FieldType_FIELD_TYPE_DECIMAL,
		TypeConfig: &nemgen.FieldTypeConfig{Decimal: config},
	}
}

func TestDecimalRendersPrecisionAndScale(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config *nemgen.FieldTypeDecimalConfig
		want   string
	}{
		{"number_of_decimals wins", &nemgen.FieldTypeDecimalConfig{NumberOfDecimals: 2}, "DECIMAL(38,2)"},
		{"high scale", &nemgen.FieldTypeDecimalConfig{NumberOfDecimals: 12}, "DECIMAL(38,12)"},
		{"currency defaults to minor units", &nemgen.FieldTypeDecimalConfig{IsCurrency: true, CurrencyCode: "USD"}, "DECIMAL(38,2)"},
		{"zero-decimal currency", &nemgen.FieldTypeDecimalConfig{IsCurrency: true, CurrencyCode: "JPY"}, "DECIMAL(38,0)"},
		{"three-decimal currency", &nemgen.FieldTypeDecimalConfig{IsCurrency: true, CurrencyCode: "kwd"}, "DECIMAL(38,3)"},
		{"explicit scale beats currency", &nemgen.FieldTypeDecimalConfig{NumberOfDecimals: 4, IsCurrency: true, CurrencyCode: "JPY"}, "DECIMAL(38,4)"},
		{"unknown currency falls back to 2", &nemgen.FieldTypeDecimalConfig{IsCurrency: true, CurrencyCode: "ZZZ"}, "DECIMAL(38,2)"},
		{"no config at all", nil, "DECIMAL(38,9)"},
		{"empty config", &nemgen.FieldTypeDecimalConfig{}, "DECIMAL(38,9)"},
		// mysql caps DECIMAL scale at 30; postgres would take more, but the
		// same model has to mean the same thing on both engines.
		{"scale clamped to mysql's limit", &nemgen.FieldTypeDecimalConfig{NumberOfDecimals: 60}, "DECIMAL(38,30)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := decimalField(tc.config)
			assert.Equal(t, tc.want, FieldTypeToMYSQL(f))
			assert.Equal(t, tc.want, FieldTypeToPG(f),
				"both engines must render the identical type — a difference here is the two engines disagreeing about one model")
		})
	}
}

func TestDecimalNeverRendersBare(t *testing.T) {
	for _, mapper := range []func(*nemgen.Field) string{FieldTypeToMYSQL, FieldTypeToPG} {
		assert.NotEqual(t, "DECIMAL", mapper(decimalField(nil)),
			"bare DECIMAL is DECIMAL(10,0) on mysql — every monetary value rounded to a whole number")
	}
}

// TestDecimalInGeneratedDDL pins the column as it lands in the create statement,
// which is the artifact that actually reaches a database.
func TestDecimalInGeneratedDDL(t *testing.T) {
	e := decimalEntity(&nemgen.FieldTypeDecimalConfig{NumberOfDecimals: 2})

	assert.Contains(t, renderCreate(t, e, db.MYSQLDBType), "`price_per_kg` DECIMAL(38,2)")
	assert.Contains(t, renderCreate(t, e, db.PGDBType), `"price_per_kg" DECIMAL(38,2)`)
}

func decimalEntity(config *nemgen.FieldTypeDecimalConfig) *nemgen.Entity {
	return &nemgen.Entity{
		Uuid:       "aaaaaaaa-0000-0000-0000-000000000001",
		Identifier: "lot",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Status:     nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
		Fields: []*nemgen.Field{
			{
				Uuid:       "aaaaaaaa-0000-0000-0000-000000000002",
				Identifier: "id",
				Type:       nemgen.FieldType_FIELD_TYPE_UUID,
				Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
				Key:        true,
				Required:   true,
			},
			{
				Uuid:       "aaaaaaaa-0000-0000-0000-000000000003",
				Identifier: "price_per_kg",
				Type:       nemgen.FieldType_FIELD_TYPE_DECIMAL,
				Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
				TypeConfig: &nemgen.FieldTypeConfig{Decimal: config},
			},
		},
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{},
		},
	}
}
