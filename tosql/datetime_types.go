package tosql

import (
	"fmt"

	nemgen "github.com/nuzur/nem/idl/gen"
)

// datetimeMaxPrecision is the most fractional-second digits either engine
// stores: mysql caps DATETIME fsp at 6, postgres caps TIMESTAMP precision at 6.
// A model asking for more is clamped rather than rejected — the alternative is a
// CREATE TABLE the engine refuses, which takes the whole migration with it.
const datetimeMaxPrecision = 6

// datetimePrecision resolves the fractional-second digits to render, or 0 for
// "render the bare type".
//
// 0 is deliberately indistinguishable from unset. On mysql that is exact —
// DATETIME and DATETIME(0) are the same type — and on postgres bare TIMESTAMP
// means precision 6, which is what an unconfigured datetime field has always
// been generated as. Rendering TIMESTAMP(0) for an unset precision would quietly
// truncate every existing postgres deployment's timestamps to whole seconds.
func datetimePrecision(config *nemgen.FieldTypeDatetimeConfig) int64 {
	p := config.GetPrecision()
	if p <= 0 {
		return 0
	}
	if p > datetimeMaxPrecision {
		return datetimeMaxPrecision
	}
	return p
}

// datetimeTypeMYSQL renders DATETIME, or DATETIME(n) when the field states a
// fractional-second precision.
func datetimeTypeMYSQL(config *nemgen.FieldTypeDatetimeConfig) string {
	if p := datetimePrecision(config); p > 0 {
		return fmt.Sprintf("DATETIME(%d)", p)
	}
	return "DATETIME"
}

// datetimeTypePG renders TIMESTAMP, or TIMESTAMP(n) when the field states a
// fractional-second precision.
func datetimeTypePG(config *nemgen.FieldTypeDatetimeConfig) string {
	if p := datetimePrecision(config); p > 0 {
		return fmt.Sprintf("TIMESTAMP(%d)", p)
	}
	return "TIMESTAMP"
}

// currentTimestampMYSQL is CURRENT_TIMESTAMP at the column's own precision.
//
// The fsp has to match: mysql rejects `DATETIME(3) DEFAULT CURRENT_TIMESTAMP`
// with "Invalid default value" (error 1067), and the same applies to the ON
// UPDATE clause.
func currentTimestampMYSQL(config *nemgen.FieldTypeDatetimeConfig) string {
	if p := datetimePrecision(config); p > 0 {
		return fmt.Sprintf("CURRENT_TIMESTAMP(%d)", p)
	}
	return "CURRENT_TIMESTAMP"
}
