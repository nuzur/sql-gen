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
	Values         map[string]string // field uuid / value
}

func GenerateInsertForEntityWithValues(ctx context.Context, params GenerateInsertForEntityWithValuesParams) (*string, error) {
	entityTemplate, err := MapEntityToSchemaEntity(params.Entity, params.ProjectVersion, params.DBType)
	if err != nil {
		return nil, err
	}

	// go through values and add quotes and escape
	finalValues := make(map[string]string)
	for _, f := range entityTemplate.Fields {
		if value, ok := params.Values[f.Field.Uuid]; ok {
			finalValues[f.Field.Uuid] = fmt.Sprintf("'%s'", EscapeValue(value))
		} else {
			finalValues[f.Field.Uuid] = "NULL"
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

	var body bytes.Buffer
	if err := tpl.Execute(&body, struct {
		Entity SchemaEntity
		Values map[string]string
	}{
		Entity: entityTemplate,
		Values: finalValues,
	}); err != nil {
		log.Println("error executing template - ", err)
		return nil, err
	}

	res := body.String()
	return &res, nil
}

type GenerateUpdateForEntityWithValuesParams struct {
	Entity         *nemgen.Entity
	ProjectVersion *nemgen.ProjectVersion
	DBType         db.DBType
	Values         map[string]string // field uuid / value
	Keys           map[string]string // field uuid / value
}

func GenerateUpdateForEntityWithValues(ctx context.Context, params GenerateUpdateForEntityWithValuesParams) (*string, error) {
	entityTemplate, err := MapEntityToSchemaEntity(params.Entity, params.ProjectVersion, params.DBType)
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

	res := body.String()
	return &res, nil
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
		case '\032': //十进制26,八进制32,十六进制1a, /* This gives problems on Win32 */
			escape = 'Z'
		}

		if escape != 0 {
			dest = append(dest, '\\', escape)
		} else {
			dest = append(dest, c)
		}
	}

	return string(dest)
}
