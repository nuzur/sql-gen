package tosql

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"path"
	"slices"
	"sync"
	"text/template"

	"github.com/nuzur/filetools"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"golang.org/x/sync/errgroup"
)

//go:embed templates/**
var templates embed.FS

type GenerateRequest struct {
	ExecutionUUID  string
	ProjectVersion *nemgen.ProjectVersion
	Configvalues   *ConfigValues
}

type GenerateResponse struct {
	ExecutionUUID string
	WorkingDir    string
	ZipFile       string
	Results       []ActionResult
}

type ActionResult struct {
	Action Action
	Data   string
}

func GenerateSQL(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	configvalues := req.Configvalues
	if len(configvalues.Actions) == 0 {
		return nil, errors.New("invalid request")
	}

	projectVersion := req.ProjectVersion

	SortStandaloneEntities(projectVersion)

	entities := []SchemaEntity{}
	for _, e := range projectVersion.Entities {
		if slices.Contains(configvalues.Entities, e.Uuid) {
			if e.Identifier == "" {
				continue
			}

			entityTemplate, err := MapEntityToSchemaEntity(e, projectVersion, configvalues.DBType)
			if err != nil {
				return nil, err
			}
			entities = append(entities, entityTemplate)
		}
	}

	if configvalues.DBType == db.PGDBType {
		// for pg we want to make sure all index names are unique
		indexOccurances := make(map[string]int)
		for i := range entities {
			for j := range entities[i].Indexes {
				indexName := entities[i].Indexes[j].Name
				if _, ok := indexOccurances[indexName]; ok {
					indexOccurances[indexName]++
					entities[i].Indexes[j].Name = fmt.Sprintf("%s_%d", indexName, indexOccurances[indexName])
				} else {
					indexOccurances[indexName] = 1
				}
			}
		}
	}

	tpl := SchemaTemplate{
		Entities: entities,
	}
	results := []ActionResult{}

	eg, _ := errgroup.WithContext(ctx)
	for _, action := range configvalues.Actions {
		eg.Go(func() error {
			return GenerateFile(ctx, &GenerateFileRequest{
				ExecutionUUID: req.ExecutionUUID,
				Configvalues:  configvalues,
				Data:          tpl,
				ActionResults: &results,
				Action:        action,
			})
		})
	}
	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	err = filetools.GenerateZip(ctx, filetools.ZipRequest{
		OutputPath: "executions",
		Identifier: req.ExecutionUUID,
	})
	if err != nil {
		return nil, err
	}

	return &GenerateResponse{
		ExecutionUUID: req.ExecutionUUID,
		WorkingDir:    path.Join("executions", req.ExecutionUUID),
		ZipFile:       path.Join("executions", fmt.Sprintf("%s.zip", req.ExecutionUUID)),
		Results:       results,
	}, nil
}

type GenerateFileRequest struct {
	mu            sync.Mutex
	ExecutionUUID string
	Configvalues  *ConfigValues
	Data          SchemaTemplate
	ActionResults *[]ActionResult
	Action        Action
}

func GenerateFile(ctx context.Context, req *GenerateFileRequest) error {
	fileName := fmt.Sprintf("%s_%s", string(req.Action), req.Configvalues.DBType)
	tmplBytes, err := templates.ReadFile(fmt.Sprintf("templates/%s.tmpl", fileName))
	if err != nil {
		return err
	}
	data, err := filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      path.Join("executions", req.ExecutionUUID, fmt.Sprintf("%s.sql", string(req.Action))),
		TemplateBytes:   tmplBytes,
		Data:            req.Data,
		DisableGoFormat: true,
		Funcs: template.FuncMap{
			"inc": func(i int) int {
				return i + 1
			},
		},
	})
	if err != nil {
		return err
	}
	req.mu.Lock()
	*req.ActionResults = append(*req.ActionResults, ActionResult{
		Action: req.Action,
		Data:   string(data),
	})
	req.mu.Unlock()

	return nil
}

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
			finalValues[f.Field.Uuid] = fmt.Sprintf("'%s'", escapeValue(value))
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
		log.Println("Error executing email template - ", err)
		return nil, err
	}

	res := body.String()
	return &res, nil
}

func escapeValue(sql string) string {
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
