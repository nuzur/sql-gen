package tosql

import (
	"fmt"
	"strings"

	nemgen "github.com/nuzur/nem/idl/gen"
)

// decimalPrecision is the total digit count every DECIMAL column is rendered
// with. A decimal field carries a scale (number_of_decimals) and no precision,
// so the precision has to be a constant: deriving it from anything else would
// render the same field two ways across runs and keep the MySQL schema diff —
// which reconstructs the existing side by introspection — proposing the same
// MODIFY COLUMN forever. 38 digits is exact on both engines (mysql caps DECIMAL
// at 65 digits, postgres NUMERIC at 1000) and leaves 29 integer digits at the
// default scale.
const decimalPrecision = 38

// decimalDefaultScale applies when a decimal field states no
// number_of_decimals. It has to be generous, because the alternative it
// replaces was not: a bare DECIMAL is arbitrary precision on postgres but
// DECIMAL(10,0) on mysql, so every unconfigured decimal was rounded to a whole
// number there. 9 places covers per-unit rates and tax fractions.
const decimalDefaultScale = 9

// decimalMaxScale is mysql's hard limit on DECIMAL scale. Postgres would take
// more, but a model has to mean the same thing on both engines.
const decimalMaxScale = 30

// currencyMinorUnits lists the ISO 4217 currencies whose minor unit is not the
// usual 2 digits. It only picks the default scale for an is_currency field that
// states no number_of_decimals — an explicit number_of_decimals always wins.
var currencyMinorUnits = map[string]int64{
	"BHD": 3, "BIF": 0, "CLP": 0, "DJF": 0, "GNF": 0, "IQD": 3, "ISK": 0,
	"JOD": 3, "JPY": 0, "KMF": 0, "KRW": 0, "KWD": 3, "LYD": 3, "OMR": 3,
	"PYG": 0, "RWF": 0, "TND": 3, "UGX": 0, "UYW": 4, "VND": 0, "VUV": 0,
	"XAF": 0, "XOF": 0, "XPF": 0,
}

// decimalType renders the column type for a decimal field. Both engines get the
// identical rendering on purpose: bare DECIMAL means arbitrary precision on
// postgres and DECIMAL(10,0) on mysql, so leaving the precision implicit makes
// the two engines silently disagree about the same model — on the field type the
// platform recommends for money.
func decimalType(config *nemgen.FieldTypeDecimalConfig) string {
	return fmt.Sprintf("DECIMAL(%d,%d)", decimalPrecision, decimalScale(config))
}

// decimalScale resolves the number of decimal places to store. A
// number_of_decimals of 0 is indistinguishable from unset in proto3, so it takes
// the default rather than rendering scale 0 — the failure mode of scale 0
// (silent rounding of money) is exactly what this is here to prevent.
func decimalScale(config *nemgen.FieldTypeDecimalConfig) int64 {
	scale := config.GetNumberOfDecimals()
	if scale <= 0 {
		scale = decimalDefaultScale
		if config.GetIsCurrency() {
			scale = 2
			if minor, found := currencyMinorUnits[strings.ToUpper(config.GetCurrencyCode())]; found {
				scale = minor
			}
		}
	}
	if scale > decimalMaxScale {
		scale = decimalMaxScale
	}
	return scale
}
