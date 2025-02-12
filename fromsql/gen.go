package fromsql

import (
	"context"
	"errors"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

func Generate(ctx context.Context, params GenerateRequest) (*nemgen.ProjectVersion, error) {
	rt := New(params)

	if params.DBType == db.MYSQLDBType {
		return rt.buildProjectVersionFromMysql()
	}

	return nil, errors.New("unsupported database type")
}
