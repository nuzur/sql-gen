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

	// Field-level `unique: true` is sugar for a single-field UNIQUE index, and
	// this is where it is desugared — before anything reads the index list.
	// GenerateSQL is the single entry point every consumer goes through,
	// including nuzur's sql-diff-manager, which hands it a raw (non-normalized)
	// project version to build the desired state of a diff.
	EnsureUniqueFieldIndexes(projectVersion)

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
		// Postgres index names must be unique per schema (MySQL scopes them per
		// table, so it needs no disambiguation).
		deduplicateIndexNames(entities)
	}

	deduplicateConstraintNames(entities)

	tpl := SchemaTemplate{
		Entities: entities,
	}
	results := []ActionResult{}

	// One mutex shared by every action, not one per request. The actions run
	// concurrently and all append to the same results slice: a per-request mutex
	// locks nothing anyone else observes, so two actions finishing together race
	// on the slice header and one of them silently loses its file — the caller
	// gets a response missing, say, the whole delete script.
	resultsMu := &sync.Mutex{}

	eg, _ := errgroup.WithContext(ctx)
	for _, action := range configvalues.Actions {
		eg.Go(func() error {
			return GenerateFile(ctx, &GenerateFileRequest{
				mu:            resultsMu,
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
	// mu guards the append to ActionResults. It is a pointer so concurrent
	// callers writing to one results slice can share it; nil means the caller
	// is not sharing (a single, sequential GenerateFile call).
	mu            *sync.Mutex
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
// deduplicateIndexNames disambiguates index names that collide across entities.
// nem index names are only unique per entity — every table carries a "created_at",
// "updated_at", and "status" index, plus per-FK indexes like "season_id" — but
// Postgres requires index names to be unique per schema. Left alone, one
// CREATE INDEX would collide with another.
//
// Colliding names are suffixed with the owning table name rather than a
// position-dependent counter. The suffix must stay identical across regenerations
// of the same schema: a counter over the (unordered) entities slice assigns the
// same name (e.g. "created_at_5") to a different physical index from run to run,
// so pg-schema-diff sees every index under a new name each time and emits a
// spurious rename → recreate → drop churn. The table name makes the result a pure
// function of (table, index) — deterministic regardless of slice order or of
// other entities being added/removed — and stays unique because index names are
// already unique within a table. (The index's own uuid is NOT a safe key: legacy
// models can carry two distinct indexes sharing a uuid.)
func deduplicateIndexNames(entities []SchemaEntity) {
	occurances := make(map[string]int)
	for i := range entities {
		for j := range entities[i].Indexes {
			occurances[entities[i].Indexes[j].Name]++
		}
	}
	for i := range entities {
		for j := range entities[i].Indexes {
			idx := &entities[i].Indexes[j]
			if occurances[idx.Name] > 1 && entities[i].Name != "" {
				idx.Name = fmt.Sprintf("%s_%s", idx.Name, entities[i].Name)
			}
		}
	}
}

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
	if req.mu != nil {
		req.mu.Lock()
		defer req.mu.Unlock()
	}
	*req.ActionResults = append(*req.ActionResults, ActionResult{
		Action: req.Action,
		Data:   string(data),
	})

	return nil
}
