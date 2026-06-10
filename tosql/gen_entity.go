package tosql

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"text/template"
	"time"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

type GenerateInsertForEntityWithValuesParams struct {
	Entity         *nemgen.Entity
	ProjectVersion *nemgen.ProjectVersion
	DBType         db.DBType
	ForGolang      bool
	Values         map[string]string // field uuid / value
}

type GenerateStatementResult struct {
	SQL             string   `json:"sql"` // display only
	ParametrizedSQL string   `json:"parametrized_sql"`
	Params          []string `json:"params"`
}

func GenerateInsertForEntityWithValues(ctx context.Context, params GenerateInsertForEntityWithValuesParams) (*GenerateStatementResult, error) {
	entityTemplate, err := MapEntityToSchemaEntity(params.Entity, params.ProjectVersion, params.DBType, params.ForGolang)
	if err != nil {
		return nil, err
	}

	// go through values and add quotes and escape.
	// Fields that are absent from Values, or are a JSON type with an empty
	// value, are treated as SQL NULL. For the parametrized SQL we emit the
	// NULL keyword directly (no placeholder) so the DB driver never receives
	// the string "NULL" as a bound parameter value.
	escapedValues := make(map[string]string)
	paramsPlaceholders := make(map[string]string)
	paramsValues := []string{}
	paramIndex := 0
	for _, f := range entityTemplate.Fields {
		value, ok := params.Values[f.Field.Uuid]
		isNull := !ok || (isJSONField(f.Field) && value == "")
		if isNull {
			escapedValues[f.Field.Uuid] = "NULL"
			paramsPlaceholders[f.Field.Uuid] = "NULL"
		} else {
			value = coerceParamValue(f.Field, value, params.DBType)
			escapedValues[f.Field.Uuid] = fmt.Sprintf("'%s'", EscapeValue(value))
			paramIndex++
			switch params.DBType {
			case db.MYSQLDBType:
				paramsPlaceholders[f.Field.Uuid] = "?"
			case db.PGDBType:
				paramsPlaceholders[f.Field.Uuid] = fmt.Sprintf("$%d", paramIndex)
			}
			paramsValues = append(paramsValues, value)
		}
	}

	fileName := fmt.Sprintf("%s_%s", "insert_data", params.DBType)
	tmplBytes, err := templates.ReadFile(fmt.Sprintf("templates/%s.tmpl", fileName))
	if err != nil {
		return nil, err
	}

	tpl, err := template.New("template").Parse(string(tmplBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating template: %s %w", fileName, err)
	}

	// display sql
	var body bytes.Buffer
	if err := tpl.Execute(&body, struct {
		Entity SchemaEntity
		Values map[string]string
	}{
		Entity: entityTemplate,
		Values: escapedValues,
	}); err != nil {
		log.Println("error executing template - ", err)
		return nil, err
	}
	displaySQL := body.String()

	// parametrized sql
	body.Reset()
	if err := tpl.Execute(&body, struct {
		Entity SchemaEntity
		Values map[string]string
	}{
		Entity: entityTemplate,
		Values: paramsPlaceholders,
	}); err != nil {
		log.Println("error executing template - ", err)
		return nil, err
	}
	parametrizedSQL := body.String()

	return &GenerateStatementResult{
		SQL:             displaySQL,
		ParametrizedSQL: parametrizedSQL,
		Params:          paramsValues,
	}, nil
}

type GenerateUpdateForEntityWithValuesParams struct {
	Entity         *nemgen.Entity
	ProjectVersion *nemgen.ProjectVersion
	DBType         db.DBType
	ForGolang      bool
	Values         map[string]string // field uuid / value
	Keys           map[string]string // field uuid / value
}

func GenerateUpdateForEntityWithValues(ctx context.Context, params GenerateUpdateForEntityWithValuesParams) (*GenerateStatementResult, error) {
	entityTemplate, err := MapEntityToSchemaEntity(params.Entity, params.ProjectVersion, params.DBType, params.ForGolang)
	if err != nil {
		return nil, err
	}

	finalKeys := make(map[string]string)
	for _, f := range entityTemplate.Fields {
		if value, ok := params.Keys[f.Field.Uuid]; ok {
			switch params.DBType {
			case db.MYSQLDBType:
				finalKeys[fmt.Sprintf("`%s`", f.Name)] = fmt.Sprintf("'%s'", EscapeValue(value))
			case db.PGDBType:
				finalKeys[fmt.Sprintf(`"%s"`, f.Name)] = fmt.Sprintf("'%s'", EscapeValue(value))
			}
		}
	}

	fileName := fmt.Sprintf("%s_%s", "update_data", params.DBType)
	tmplBytes, err := templates.ReadFile(fmt.Sprintf("templates/%s.tmpl", fileName))
	if err != nil {
		return nil, err
	}

	tpl, err := template.New("template").Parse(string(tmplBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating template: %s %w", fileName, err)
	}

	// display sql
	var body bytes.Buffer
	if err := tpl.Execute(&body, struct {
		Entity       SchemaEntity
		UpdateFields string
		WhereClause  string
	}{
		Entity:       entityTemplate,
		UpdateFields: entityTemplate.UpdateFieldsWithValues(params.Values),
		WhereClause:  entityTemplate.PrimaryKeysWhereClauseWithValues(finalKeys),
	}); err != nil {
		log.Println("error executing template - ", err)
		return nil, err
	}
	displaySQL := body.String()

	// parametrized sql
	body.Reset()
	if err := tpl.Execute(&body, struct {
		Entity       SchemaEntity
		UpdateFields string
		WhereClause  string
	}{
		Entity:       entityTemplate,
		UpdateFields: entityTemplate.UpdateFieldsParam(true, true, params.Values),
		WhereClause:  entityTemplate.PrimaryKeysWhereClauseParam(true, true),
	}); err != nil {
		log.Println("error executing template - ", err)
		return nil, err
	}
	parametrizedSQL := body.String()

	paramValues := []string{}
	for _, f := range entityTemplate.Fields {
		if !f.Field.Key {
			if value, ok := params.Values[f.Field.Uuid]; ok {
				// JSON fields with empty value are emitted as NULL literals in
				// the parametrized SQL, so they must not be added as bound params.
				if isJSONField(f.Field) && value == "" {
					continue
				}
				paramValues = append(paramValues, coerceParamValue(f.Field, value, params.DBType))
			}
		}
	}
	for _, f := range entityTemplate.Fields {
		if f.Field.Key {
			if value, ok := params.Keys[f.Field.Uuid]; ok {
				paramValues = append(paramValues, coerceParamValue(f.Field, value, params.DBType))
			}
		}
	}

	return &GenerateStatementResult{
		SQL:             displaySQL,
		ParametrizedSQL: parametrizedSQL,
		Params:          paramValues,
	}, nil
}

type GenerateDeleteForEntityWithValuesParams struct {
	Entity         *nemgen.Entity
	ProjectVersion *nemgen.ProjectVersion
	DBType         db.DBType
	ForGolang      bool
	Keys           map[string]string // field uuid / value
}

func GenerateDeleteForEntityWithValues(ctx context.Context, params GenerateDeleteForEntityWithValuesParams) (*GenerateStatementResult, error) {
	entityTemplate, err := MapEntityToSchemaEntity(params.Entity, params.ProjectVersion, params.DBType, params.ForGolang)
	if err != nil {
		return nil, err
	}

	finalKeys := make(map[string]string)
	for _, f := range entityTemplate.Fields {
		if value, ok := params.Keys[f.Field.Uuid]; ok {
			switch params.DBType {
			case db.MYSQLDBType:
				finalKeys[fmt.Sprintf("`%s`", f.Name)] = fmt.Sprintf("'%s'", EscapeValue(value))
			case db.PGDBType:
				finalKeys[fmt.Sprintf(`"%s"`, f.Name)] = fmt.Sprintf("'%s'", EscapeValue(value))
			}
		}
	}

	fileName := fmt.Sprintf("%s_%s", "delete_data", params.DBType)
	tmplBytes, err := templates.ReadFile(fmt.Sprintf("templates/%s.tmpl", fileName))
	if err != nil {
		return nil, err
	}

	tpl, err := template.New("template").Parse(string(tmplBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating template: %s %w", fileName, err)
	}

	// display sql
	var body bytes.Buffer
	if err := tpl.Execute(&body, struct {
		Entity      SchemaEntity
		WhereClause string
	}{
		Entity:      entityTemplate,
		WhereClause: entityTemplate.PrimaryKeysWhereClauseWithValues(finalKeys),
	}); err != nil {
		log.Println("error executing template - ", err)
		return nil, err
	}
	displaySQL := body.String()

	// parametrized sql
	body.Reset()
	if err := tpl.Execute(&body, struct {
		Entity      SchemaEntity
		WhereClause string
	}{
		Entity:      entityTemplate,
		WhereClause: entityTemplate.PrimaryKeysWhereClauseParam(true, false),
	}); err != nil {
		log.Println("error executing template - ", err)
		return nil, err
	}
	parametrizedSQL := body.String()

	paramValues := []string{}
	for _, f := range entityTemplate.Fields {
		if f.Field.Key {
			if value, ok := params.Keys[f.Field.Uuid]; ok {
				paramValues = append(paramValues, coerceParamValue(f.Field, value, params.DBType))
			}
		}
	}
	return &GenerateStatementResult{
		SQL:             displaySQL,
		ParametrizedSQL: parametrizedSQL,
		Params:          paramValues,
	}, nil
}

// coerceParamValue normalizes a stringified field value into the form the SQL
// driver expects for that column type. It exists because the upstream UI
// serializes values as strings (e.g. "true"/"false" for booleans, ISO 8601
// for datetimes) and mysql in strict mode rejects formats it considers
// non-canonical. Postgres is largely tolerant and gets left alone here.
// Only the cases we've actually seen blow up in practice are covered;
// everything else passes through unchanged.
func coerceParamValue(field *nemgen.Field, value string, dbType db.DBType) string {
	if field == nil {
		return value
	}
	switch field.GetType() {
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		// Both mysql (TINYINT(1)) and pg (BOOLEAN) accept "0"/"1"; mysql in
		// strict mode rejects "true"/"false" string literals. Normalize.
		switch value {
		case "true", "TRUE", "True":
			return "1"
		case "false", "FALSE", "False":
			return "0"
		}
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		// 1-bit INTEGER maps to TINYINT(1) on mysql and BOOLEAN on pg — same
		// shape as a boolean field as far as accepted literals go.
		if field.GetTypeConfig() != nil && field.GetTypeConfig().GetInteger() != nil &&
			field.GetTypeConfig().GetInteger().GetSize() == nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_ONE_BIT {
			switch value {
			case "true", "TRUE", "True":
				return "1"
			case "false", "FALSE", "False":
				return "0"
			}
		}
	case nemgen.FieldType_FIELD_TYPE_DATE:
		if dbType == db.MYSQLDBType {
			if t, ok := parseISOTime(value); ok {
				return t.UTC().Format("2006-01-02")
			}
		}
	case nemgen.FieldType_FIELD_TYPE_DATETIME:
		if dbType == db.MYSQLDBType {
			if t, ok := parseISOTime(value); ok {
				// mysql DATETIME wants `YYYY-MM-DD HH:MM:SS[.ffffff]`. The
				// upstream UI sends RFC3339 with `T` and a `Z` suffix, which
				// strict-mode mysql refuses (error 1292). Reformat in UTC.
				return t.UTC().Format("2006-01-02 15:04:05.999999")
			}
		}
	case nemgen.FieldType_FIELD_TYPE_TIME:
		if dbType == db.MYSQLDBType {
			if t, ok := parseISOTime(value); ok {
				return t.UTC().Format("15:04:05.999999")
			}
		}
	}
	return value
}

// parseISOTime tolerates the handful of ISO-ish formats the UI may emit:
// RFC3339 with fractional seconds (`...Z`), RFC3339 without fractional
// seconds, and the mysql-native `YYYY-MM-DD HH:MM:SS` form (left alone).
// Returns false when the value isn't recognized so the caller passes it
// through untouched.
func parseISOTime(value string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02T15:04:05.999999999", value); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02T15:04:05", value); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func EscapeValue(sql string) string {
	dest := make([]byte, 0, 2*len(sql))
	var escape byte
	for i := 0; i < len(sql); i++ {
		c := sql[i]

		escape = 0

		switch c {
		case 0: /* Must be escaped for 'mysql' */
			escape = '0'
		case '\n': /* Must be escaped for logs */
			escape = 'n'
		case '\r':
			escape = 'r'
		case '\\':
			escape = '\\'
		case '\'':
			escape = '\''
		case '"': /* Better safe than sorry */
			escape = '"'
		case '\032': /* This gives problems on Win32 */
			escape = 'Z'
		case ';':
			escape = ';'
		}

		if escape != 0 {
			dest = append(dest, '\\', escape)
		} else {
			dest = append(dest, c)
		}
	}

	return string(dest)
}
