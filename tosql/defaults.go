package tosql

import (
	"fmt"
	"regexp"
	"strings"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

// currentTimestampExpr matches CURRENT_TIMESTAMP and CURRENT_TIMESTAMP(n),
// which mysql accepts bare in a DEFAULT / ON UPDATE clause. Every other
// expression default has to be wrapped in parentheses there.
var currentTimestampExpr = regexp.MustCompile(`(?i)^current_timestamp(\(\s*\d+\s*\))?$`)

// bareLiteralFieldTypes are the field types whose column takes an unquoted
// default literal. Quoting a number is accepted by both engines but comes back
// from information_schema unquoted, so the round trip would not be a fixed point
// — and on postgres a quoted literal is reported with a `::type` cast that the
// re-rendered DDL would not have.
var bareLiteralFieldTypes = map[nemgen.FieldType]bool{
	nemgen.FieldType_FIELD_TYPE_INTEGER: true,
	nemgen.FieldType_FIELD_TYPE_FLOAT:   true,
	nemgen.FieldType_FIELD_TYPE_DECIMAL: true,
	nemgen.FieldType_FIELD_TYPE_BOOLEAN: true,
	nemgen.FieldType_FIELD_TYPE_ENUM:    true,
}

// defaultClause renders the DEFAULT clause for a column, or "" when the column
// takes none.
//
// Precedence:
//  1. an explicit default_value on the field, as a verbatim expression when
//     default_value_is_expression is set and as a literal otherwise;
//  2. for a datetime field that states neither, DEFAULT CURRENT_TIMESTAMP —
//     the implicit default this generator has always emitted. Every model
//     created before per-field defaults existed relies on it, so it stays the
//     behavior for an unset default; a field opts out with the datetime config's
//     no_default_current_timestamp (which is also what introspection sets for a
//     datetime column the database reports without a default, so re-rendering an
//     imported schema does not invent one).
func defaultClause(f *nemgen.Field, dbType db.DBType) string {
	if f == nil {
		return ""
	}

	if value := f.GetDefaultValue(); value != "" {
		if f.GetDefaultValueIsExpression() {
			return "DEFAULT " + defaultExpression(value, dbType)
		}
		if bareLiteralFieldTypes[f.GetType()] {
			// BOOLEAN is the one bare-literal type whose spelling is not the
			// same on both engines. INTEGER, FLOAT, DECIMAL and ENUM render
			// verbatim, deliberately.
			if f.GetType() == nemgen.FieldType_FIELD_TYPE_BOOLEAN {
				value = booleanDefaultLiteral(value, dbType)
			}
			return "DEFAULT " + value
		}
		return fmt.Sprintf("DEFAULT '%s'", EscapeValue(value))
	}

	if f.GetType() == nemgen.FieldType_FIELD_TYPE_DATETIME {
		config := f.GetTypeConfig().GetDatetime()
		if config.GetNoDefaultCurrentTimestamp() {
			return ""
		}
		if dbType == db.MYSQLDBType {
			return "DEFAULT " + currentTimestampMYSQL(config)
		}
		return "DEFAULT CURRENT_TIMESTAMP"
	}

	return ""
}

// defaultExpression renders an expression default for one engine. mysql only
// accepts a bare expression for the CURRENT_TIMESTAMP forms; anything else — a
// function call such as uuid() — has to be parenthesized, and mysql reports it
// back through information_schema without the parentheses, so they are added
// here rather than expected from the model. Postgres accepts either form and
// reports it back unparenthesized too.
func defaultExpression(value string, dbType db.DBType) string {
	value = strings.TrimSpace(value)
	if dbType != db.MYSQLDBType {
		return value
	}
	if currentTimestampExpr.MatchString(value) {
		return value
	}
	if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
		return value
	}
	return "(" + value + ")"
}

// booleanLiteral resolves the spellings a boolean default_value arrives in onto
// a single value. "true"/"false" is what a hand-written model stores; "1"/"0" is
// what a mysql import persists, because mysql reports a TINYINT(1)'s
// COLUMN_DEFAULT numerically and sql-import saves that into the project version.
// Both have to mean the same column. Matching is case-insensitive so TRUE, True
// and true are one value.
//
// ok=false means the value is not a boolean spelling at all — see
// booleanDefaultLiteral for why that is returned unchanged rather than coerced.
func booleanLiteral(value string) (isTrue bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	}
	return false, false
}

// booleanDefaultLiteral renders a boolean default in the spelling the engine
// itself reports back, so the DDL generated from the model and the DDL
// reconstructed by introspecting a live database are the same string.
//
// mysql has no boolean type: BOOLEAN renders TINYINT(1), and a column created
// with DEFAULT true comes back from information_schema.COLUMN_DEFAULT as 1.
// Introspection passes that through, so the reconstructed "existing" side of a
// mysql diff renders DEFAULT 1 while the model side renders DEFAULT true. The
// differ compares the two column definitions as ASTs — BoolVal(true) against
// Literal{IntVal,"1"} — and those are never equal, so it proposes MODIFY COLUMN
// on every plan and applying it changes nothing. (vitess has a normalizer for
// exactly this, but it is gated on the column type being spelled "boolean",
// which this generator never emits.)
//
// Postgres has a real boolean and reports true/false, so it gets the keywords.
// That also fixes the other direction: a model that stores "1" on a boolean
// field — every project imported from mysql — rendered BOOLEAN DEFAULT 1, which
// postgres rejects outright at CREATE TABLE.
func booleanDefaultLiteral(value string, dbType db.DBType) string {
	isTrue, ok := booleanLiteral(value)
	if !ok {
		// Not a boolean spelling. Introspection types EVERY tinyint(1) as a
		// BOOLEAN field, so a column someone really declared as
		// `tinyint(1) DEFAULT 5` arrives here as "5". Rendering DEFAULT 5
		// reproduces what the database actually has and keeps the round trip a
		// fixed point; coercing it to 0 would silently rewrite a live default,
		// and the introspected side would then disagree with the model side on
		// every plan — the same non-converging diff, from the other direction.
		// A genuinely invalid value is left to fail at CREATE TABLE, where the
		// engine's error names the column.
		return value
	}
	if dbType == db.MYSQLDBType {
		if isTrue {
			return "1"
		}
		return "0"
	}
	if isTrue {
		return "true"
	}
	return "false"
}

// onUpdateClause renders ON UPDATE CURRENT_TIMESTAMP for a datetime field that
// asks for it.
//
// MYSQL ONLY, and silently absent on postgres by necessity: postgres has no
// column-level ON UPDATE — the equivalent needs a row trigger, which this
// generator does not emit, and inventing one here would create an object no
// other part of the pipeline knows how to diff or drop. The divergence is
// surfaced where a modeller can act on it instead: schemaops raises a
// mysql-only design warning when the flag is set (see design_warnings.go), and
// the field config documents it.
func onUpdateClause(f *nemgen.Field, dbType db.DBType) string {
	if dbType != db.MYSQLDBType || f.GetType() != nemgen.FieldType_FIELD_TYPE_DATETIME {
		return ""
	}
	config := f.GetTypeConfig().GetDatetime()
	if !config.GetOnUpdateCurrentTimestamp() {
		return ""
	}
	return "ON UPDATE " + currentTimestampMYSQL(config)
}
