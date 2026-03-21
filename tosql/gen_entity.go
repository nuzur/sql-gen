package tosql

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"text/template"

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

	// go through values and add quotes and escape
	escapedValues := make(map[string]string)
	paramsPlaceholders := make(map[string]string)
	paramsValues := []string{}
	for _, f := range entityTemplate.Fields {
		switch params.DBType {
		case db.MYSQLDBType:
			paramsPlaceholders[f.Field.Uuid] = "?"
		case db.PGDBType:
			paramsPlaceholders[f.Field.Uuid] = fmt.Sprintf("$%d", len(paramsPlaceholders)+1)
		}
		if value, ok := params.Values[f.Field.Uuid]; ok {
			escapedValues[f.Field.Uuid] = fmt.Sprintf("'%s'", EscapeValue(value))
			paramsValues = append(paramsValues, value)
		} else {
			escapedValues[f.Field.Uuid] = "NULL"
			paramsValues = append(paramsValues, "NULL")
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
				paramValues = append(paramValues, value)
			}
		}
	}
	for _, f := range entityTemplate.Fields {
		if f.Field.Key {
			if value, ok := params.Keys[f.Field.Uuid]; ok {
				paramValues = append(paramValues, value)
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
				paramValues = append(paramValues, value)
			}
		}
	}
	return &GenerateStatementResult{
		SQL:             displaySQL,
		ParametrizedSQL: parametrizedSQL,
		Params:          paramValues,
	}, nil
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
