package fromsql

import (
	"strconv"
	"strings"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/nuzur/sql-gen/tosql"
)

// promotionPreservesWidth reports whether re-rendering a column as the promoted
// semantic field type produces the width the live column already has.
//
// Sample-based promotion (a varchar full of addresses coming back as an email
// field, one full of links as a url field) is a modelling nicety, but the
// promoted types carry no max_size: they render at a FIXED width — VARCHAR(512)
// for an email on both engines, VARCHAR(512) on MySQL and VARCHAR(2048) on
// Postgres for a url. So promoting a varchar(200) rewrites it to varchar(512),
// and on MySQL — where the "existing" side of a diff is this reconstruction
// re-rendered as DDL — that is a MODIFY COLUMN proposed on every plan, applied
// on every deploy, and back again on the next plan. The introspected schema has
// to render exactly what created it, so a promotion that would change the width
// is refused and the column stays a varchar of its real size.
func promotionPreservesWidth(t nemgen.FieldType, dbType db.DBType, width int64) bool {
	f := &nemgen.Field{Type: t, TypeConfig: &nemgen.FieldTypeConfig{}}

	var rendered string
	switch dbType {
	case db.MYSQLDBType:
		rendered = tosql.FieldTypeToMYSQL(f)
	case db.PGDBType:
		rendered = tosql.FieldTypeToPG(f)
	default:
		return false
	}

	declared, ok := declaredWidth(rendered)
	return ok && declared == width
}

// declaredWidth reads n out of a rendered "VARCHAR(n)" / "CHAR(n)" type. A type
// with no width at all (UUID, TEXT) reports false — it cannot preserve one.
func declaredWidth(sqlType string) (int64, bool) {
	open := strings.IndexByte(sqlType, '(')
	if open < 0 || !strings.HasSuffix(sqlType, ")") {
		return 0, false
	}
	n, err := strconv.ParseInt(sqlType[open+1:len(sqlType)-1], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
