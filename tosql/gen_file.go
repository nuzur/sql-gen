package tosql

import (
	"context"
	"embed"
	"errors"
	"fmt"
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
