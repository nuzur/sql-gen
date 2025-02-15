package fromsql

import (
	"context"
	"errors"

	"github.com/gofrs/uuid"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/nuzur/sql-gen/tosql"
)

func GenerateProjectVersion(ctx context.Context, params GenerateRequest) (*nemgen.ProjectVersion, error) {
	rt := New(params)

	if params.DBType == db.MYSQLDBType {
		return rt.buildProjectVersionFromMysql()
	} else if params.DBType == db.PGDBType {
		return rt.buildProjectVersionFromPg()
	}

	return nil, errors.New("unsupported database type")
}

func GenerateSQL(ctx context.Context, params GenerateRequest) (*tosql.GenerateResponse, error) {
	rt := New(params)

	if params.DBType == db.MYSQLDBType {
		pv, err := rt.buildProjectVersionFromMysql()
		if err != nil {
			return nil, err
		}

		entities := []string{}
		for _, e := range pv.Entities {
			if e.Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE {
				entities = append(entities, e.Uuid)
			}
		}
		return tosql.GenerateSQL(ctx, tosql.GenerateRequest{
			ExecutionUUID:  uuid.Must(uuid.NewV4()).String(),
			ProjectVersion: pv,
			Configvalues: &tosql.ConfigValues{
				DBType:   params.DBType,
				Entities: entities,
				Actions: []tosql.Action{
					tosql.CreateAction,
				},
			},
		})
	}

	return nil, errors.New("unsupported database type")
}
