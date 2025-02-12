package fromsql

import (
	"github.com/jmoiron/sqlx"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

type GenerateRequest struct {
	UserConnection *nemgen.UserConnection
	SQLConnection  *sqlx.DB
	DBType         db.DBType
	Version        *int64
}

type sqlremote struct {
	userConnection *nemgen.UserConnection
	sqlConnection  *sqlx.DB
	dbType         db.DBType
	version        *int64
}

type remoteRows []map[string]interface{}
