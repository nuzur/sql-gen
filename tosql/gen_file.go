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
	ForGolang      bool
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

			entityTemplate, err := MapEntityToSchemaEntity(e, projectVersion, configvalues.DBType, req.ForGolang)
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

	deduplicateConstraintNames(entities)

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

// deduplicateConstraintNames disambiguates FK constraint names that collide across
// entities. Constraint names come from relationship.Identifier, which is only
// normally unique - legacy schemas can carry relationships that share an
// identifier (e.g. two FKs from "episode" to "drag"). Postgres requires constraint
// names to be unique per table and MySQL requires them unique per database, so any
// collision here would make the generated DDL invalid.
//
// Colliding names are suffixed with the owning relationship's own uuid rather than
// a position-dependent counter: the suffix must stay identical across
// regenerations of the same schema, or pg-schema-diff would see it as an unrelated
// constraint and emit a spurious drop+recreate.
func deduplicateConstraintNames(entities []SchemaEntity) {
	occurances := make(map[string]int)
	for i := range entities {
		for j := range entities[i].Constraints {
			occurances[entities[i].Constraints[j].Name]++
		}
	}
	for i := range entities {
		for j := range entities[i].Constraints {
			constraint := &entities[i].Constraints[j]
			if occurances[constraint.Name] > 1 && constraint.Relationship != nil && len(constraint.Relationship.Uuid) >= 8 {
				constraint.Name = fmt.Sprintf("%s_%s", constraint.Name, constraint.Relationship.Uuid[:8])
			}
		}
	}
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
