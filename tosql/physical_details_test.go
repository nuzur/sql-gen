package tosql

import (
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/stretchr/testify/assert"
)

// Four physical-SQL details the model could not express: fractional-second
// precision on a datetime, a column DEFAULT, mysql's ON UPDATE CURRENT_TIMESTAMP
// and a foreign key's referential actions. Every expectation below was executed
// against stock mysql:8 and postgres:16 containers and read back out of
// information_schema before being written down here.

func datetimeField(config *nemgen.FieldTypeDatetimeConfig) *nemgen.Field {
	return &nemgen.Field{
		Identifier: "created_at",
		Type:       nemgen.FieldType_FIELD_TYPE_DATETIME,
		TypeConfig: &nemgen.FieldTypeConfig{Datetime: config},
	}
}

func TestDatetimePrecisionRendersPerEngine(t *testing.T) {
	for _, tc := range []struct {
		name      string
		config    *nemgen.FieldTypeDatetimeConfig
		wantMysql string
		wantPG    string
	}{
		// Unset must keep rendering the bare type. Anything else rewrites the
		// column type of every datetime in every deployment that exists.
		{"unset", nil, "DATETIME", "TIMESTAMP"},
		{"empty config", &nemgen.FieldTypeDatetimeConfig{}, "DATETIME", "TIMESTAMP"},
		{"milliseconds", &nemgen.FieldTypeDatetimeConfig{Precision: 3}, "DATETIME(3)", "TIMESTAMP(3)"},
		{"microseconds", &nemgen.FieldTypeDatetimeConfig{Precision: 6}, "DATETIME(6)", "TIMESTAMP(6)"},
		// Both engines cap at 6; a larger value is clamped rather than emitted,
		// because a rejected CREATE TABLE takes the whole migration with it.
		{"clamped", &nemgen.FieldTypeDatetimeConfig{Precision: 9}, "DATETIME(6)", "TIMESTAMP(6)"},
		{"negative is unset", &nemgen.FieldTypeDatetimeConfig{Precision: -1}, "DATETIME", "TIMESTAMP"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := datetimeField(tc.config)
			assert.Equal(t, tc.wantMysql, FieldTypeToMYSQL(f))
			assert.Equal(t, tc.wantPG, FieldTypeToPG(f))
		})
	}
}

// A datetime with no default still gets DEFAULT CURRENT_TIMESTAMP — the
// behavior every model written before per-field defaults existed relies on —
// and mysql's default has to carry the column's own fsp or the engine rejects
// the column definition outright (error 1067).
func TestDatetimeDefaultStaysImplicitAndMatchesPrecision(t *testing.T) {
	assert.Equal(t, "DEFAULT CURRENT_TIMESTAMP", defaultClause(datetimeField(nil), db.MYSQLDBType))
	assert.Equal(t, "DEFAULT CURRENT_TIMESTAMP", defaultClause(datetimeField(nil), db.PGDBType))

	withFsp := datetimeField(&nemgen.FieldTypeDatetimeConfig{Precision: 3})
	assert.Equal(t, "DEFAULT CURRENT_TIMESTAMP(3)", defaultClause(withFsp, db.MYSQLDBType))
	// postgres' CURRENT_TIMESTAMP is not precision-qualified; the assignment to a
	// TIMESTAMP(3) column truncates on its own.
	assert.Equal(t, "DEFAULT CURRENT_TIMESTAMP", defaultClause(withFsp, db.PGDBType))

	optOut := datetimeField(&nemgen.FieldTypeDatetimeConfig{NoDefaultCurrentTimestamp: true})
	assert.Equal(t, "", defaultClause(optOut, db.MYSQLDBType))
	assert.Equal(t, "", defaultClause(optOut, db.PGDBType))
}

func TestDefaultClauseLiteralsAndExpressions(t *testing.T) {
	literal := func(fieldType nemgen.FieldType, value string) *nemgen.Field {
		return &nemgen.Field{Identifier: "col", Type: fieldType, DefaultValue: value}
	}

	// Text-ish columns take a quoted literal; numeric and boolean ones take it
	// bare, because that is how both engines report it back and a quoted number
	// would never round-trip to itself.
	assert.Equal(t, "DEFAULT 'active'",
		defaultClause(literal(nemgen.FieldType_FIELD_TYPE_VARCHAR, "active"), db.MYSQLDBType))
	assert.Equal(t, "DEFAULT 'active'",
		defaultClause(literal(nemgen.FieldType_FIELD_TYPE_VARCHAR, "active"), db.PGDBType))
	assert.Equal(t, "DEFAULT 5",
		defaultClause(literal(nemgen.FieldType_FIELD_TYPE_INTEGER, "5"), db.MYSQLDBType))
	assert.Equal(t, "DEFAULT 0",
		defaultClause(literal(nemgen.FieldType_FIELD_TYPE_DECIMAL, "0"), db.PGDBType))
	assert.Equal(t, "DEFAULT 1",
		defaultClause(literal(nemgen.FieldType_FIELD_TYPE_ENUM, "1"), db.MYSQLDBType))

	// A quote in a literal must not end the literal.
	assert.Equal(t, `DEFAULT 'o\'brien'`,
		defaultClause(literal(nemgen.FieldType_FIELD_TYPE_VARCHAR, "o'brien"), db.MYSQLDBType))

	expr := func(value string) *nemgen.Field {
		return &nemgen.Field{
			Identifier: "col", Type: nemgen.FieldType_FIELD_TYPE_VARCHAR,
			DefaultValue: value, DefaultValueIsExpression: true,
		}
	}
	// mysql only accepts a bare expression for the CURRENT_TIMESTAMP forms;
	// anything else has to be parenthesized, and mysql reports it back without
	// the parentheses — so they are added here, not expected from the model.
	assert.Equal(t, "DEFAULT (uuid())", defaultClause(expr("uuid()"), db.MYSQLDBType))
	assert.Equal(t, "DEFAULT (uuid())", defaultClause(expr("(uuid())"), db.MYSQLDBType))
	assert.Equal(t, "DEFAULT CURRENT_TIMESTAMP", defaultClause(expr("CURRENT_TIMESTAMP"), db.MYSQLDBType))
	assert.Equal(t, "DEFAULT gen_random_uuid()", defaultClause(expr("gen_random_uuid()"), db.PGDBType))

	// An explicit default beats the datetime fallback.
	explicit := datetimeField(nil)
	explicit.DefaultValue = "2020-01-01 00:00:00"
	assert.Equal(t, "DEFAULT '2020-01-01 00:00:00'", defaultClause(explicit, db.MYSQLDBType))
}

// ON UPDATE CURRENT_TIMESTAMP is mysql-only: postgres has no column-level
// equivalent, so the clause is absent there by necessity. The divergence is
// surfaced as a schemaops design warning rather than being invisible.
func TestOnUpdateCurrentTimestampIsMysqlOnly(t *testing.T) {
	f := datetimeField(&nemgen.FieldTypeDatetimeConfig{OnUpdateCurrentTimestamp: true})
	assert.Equal(t, "ON UPDATE CURRENT_TIMESTAMP", onUpdateClause(f, db.MYSQLDBType))
	assert.Equal(t, "", onUpdateClause(f, db.PGDBType))

	withFsp := datetimeField(&nemgen.FieldTypeDatetimeConfig{Precision: 3, OnUpdateCurrentTimestamp: true})
	assert.Equal(t, "ON UPDATE CURRENT_TIMESTAMP(3)", onUpdateClause(withFsp, db.MYSQLDBType))

	// The clause follows DEFAULT — mysql's column grammar fixes that order.
	sf := SchemaField{Null: "NOT NULL", Default: "DEFAULT CURRENT_TIMESTAMP(3)", OnUpdate: "ON UPDATE CURRENT_TIMESTAMP(3)"}
	assert.Equal(t, "NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)", sf.Postfix())

	// A non-datetime field never gets one, whatever its config says.
	notDatetime := &nemgen.Field{Identifier: "col", Type: nemgen.FieldType_FIELD_TYPE_VARCHAR}
	assert.Equal(t, "", onUpdateClause(notDatetime, db.MYSQLDBType))
}

func TestReferentialActionsRender(t *testing.T) {
	constraint := func(onDelete, onUpdate nemgen.RelationshipReferentialAction) SchemaConstraint {
		return SchemaConstraint{Relationship: &nemgen.Relationship{OnDelete: onDelete, OnUpdate: onUpdate}}
	}
	const (
		unset    = nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_INVALID
		noAction = nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_NO_ACTION
		cascade  = nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_CASCADE
		setNull  = nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_SET_NULL
		restrict = nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_RESTRICT
	)

	// Unset renders nothing, so existing schemas see no DDL change at all.
	assert.Equal(t, "", constraint(unset, unset).ReferentialActions())
	// NO ACTION is the SQL default and is what both engines report for a
	// constraint created without a clause — spelling it out would put the
	// generated DDL permanently out of step with introspection.
	assert.Equal(t, "", constraint(noAction, noAction).ReferentialActions())

	assert.Equal(t, "\n        ON DELETE CASCADE", constraint(cascade, unset).ReferentialActions())
	assert.Equal(t, "\n        ON UPDATE RESTRICT", constraint(unset, restrict).ReferentialActions())
	assert.Equal(t, "\n        ON DELETE SET NULL\n        ON UPDATE CASCADE",
		constraint(setNull, cascade).ReferentialActions())
}

func TestReferentialActionRoundTripsThroughSQLSpelling(t *testing.T) {
	for _, rule := range []string{"CASCADE", "cascade", "SET NULL", "set_null", "RESTRICT", "SET DEFAULT"} {
		action := ReferentialActionFromSQL(rule)
		if action == nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_INVALID {
			t.Fatalf("rule %q did not map to an action", rule)
		}
		assert.Equal(t, normalizeRule(rule), ReferentialActionSQL(action))
	}
	// Both engines report this for a constraint with no explicit clause.
	assert.Equal(t, nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_INVALID,
		ReferentialActionFromSQL("NO ACTION"))
}
