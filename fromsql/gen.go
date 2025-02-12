package fromsql

import (
	"errors"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

func Generate(params Params) (*nemgen.ProjectVersion, error) {
	rt := New(params)

	if params.DBType == db.MYSQLDBType {
		return rt.buildProjectVersionFromMysql()
	}

	return nil, errors.New("unsupported database type")
}
