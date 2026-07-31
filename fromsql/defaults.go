package fromsql

import (
	"regexp"
	"strings"
)

// columnDefault is what introspection recovers from a column's DEFAULT: the
// value, whether it is a SQL expression rather than a literal, and whether the
// column has a default at all (which is not the same as an empty literal).
type columnDefault struct {
	Value        string
	IsExpression bool
	Present      bool
}

// numericLiteral matches a bare numeric default, which is reported unquoted by
// both engines and has to be re-rendered unquoted.
var numericLiteral = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// currentTimestampLiteral matches the CURRENT_TIMESTAMP forms mysql reports in
// COLUMN_DEFAULT. mysql 8 also flags them in EXTRA, but older rows and some
// forks do not, so the spelling is matched directly as well.
var currentTimestampLiteral = regexp.MustCompile(`(?i)^current_timestamp(\(\s*\d+\s*\))?$`)

// mysqlColumnDefault reads a mysql column's default.
//
// COLUMN_DEFAULT is NULL when the column has none. An expression default is
// reported without its parentheses (`uuid()`, not `(uuid())`) and marked in
// EXTRA as DEFAULT_GENERATED; a literal is reported raw, without quotes.
func mysqlColumnDefault(raw *string, extra string) columnDefault {
	if raw == nil {
		return columnDefault{}
	}
	value := *raw
	isExpression := strings.Contains(strings.ToUpper(extra), "DEFAULT_GENERATED") ||
		currentTimestampLiteral.MatchString(strings.TrimSpace(value))
	return columnDefault{Value: value, IsExpression: isExpression, Present: true}
}

// mysqlOnUpdateCurrentTimestamp reports whether the column carries ON UPDATE
// CURRENT_TIMESTAMP, which EXTRA is the only place to find it.
func mysqlOnUpdateCurrentTimestamp(extra string) bool {
	return strings.Contains(strings.ToUpper(extra), "ON UPDATE CURRENT_TIMESTAMP")
}

// pgColumnDefault reads a postgres column's default.
//
// Postgres reports the default as it stores it — already resolved and, for a
// literal, carrying an explicit cast: `'active'::character varying`. The cast
// and quotes are stripped so the value round-trips into the same DDL it came
// from; anything that is not a quoted literal, a number or a boolean keyword is
// an expression (CURRENT_TIMESTAMP, now(), nextval(...)) and is kept verbatim.
func pgColumnDefault(raw *string) columnDefault {
	if raw == nil {
		return columnDefault{}
	}
	value := strings.TrimSpace(*raw)
	if value == "" {
		return columnDefault{}
	}

	if literal, ok := stripPgQuotedLiteral(value); ok {
		return columnDefault{Value: literal, Present: true}
	}
	if numericLiteral.MatchString(value) || strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
		return columnDefault{Value: value, Present: true}
	}
	return columnDefault{Value: value, IsExpression: true, Present: true}
}

// stripPgQuotedLiteral unwraps `'value'` and `'value'::type`, undoubling the
// doubled quotes postgres escapes with. It returns ok=false for anything that is
// not exactly one quoted literal (optionally cast), so a concatenation or a
// function call whose argument happens to be quoted stays an expression.
func stripPgQuotedLiteral(value string) (string, bool) {
	if !strings.HasPrefix(value, "'") {
		return "", false
	}
	var sb strings.Builder
	for i := 1; i < len(value); i++ {
		switch {
		case value[i] != '\'':
			sb.WriteByte(value[i])
		case i+1 < len(value) && value[i+1] == '\'':
			sb.WriteByte('\'')
			i++
		default:
			// closing quote: everything after it must be a cast, or this is not a
			// plain literal.
			rest := strings.TrimSpace(value[i+1:])
			if rest != "" && !strings.HasPrefix(rest, "::") {
				return "", false
			}
			return sb.String(), true
		}
	}
	return "", false
}

// pgDefaultDatetimePrecision is the precision a bare postgres TIMESTAMP has.
// information_schema always reports a number, so without folding the engine
// default back to "unset" every imported timestamp column would come back
// asking for TIMESTAMP(6) — the same type, spelled differently from the DDL it
// was created with.
const pgDefaultDatetimePrecision = 6

// pgDatetimePrecision maps a reported precision onto the model's, treating the
// engine default as unset.
func pgDatetimePrecision(precision *int64) int64 {
	if precision == nil || *precision == pgDefaultDatetimePrecision {
		return 0
	}
	return *precision
}
