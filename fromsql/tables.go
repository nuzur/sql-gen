package fromsql

import (
	"fmt"

	"github.com/nuzur/sql-gen/db"
)

func New(params GenerateRequest) *sqlremote {
	return &sqlremote{
		userConnection: params.UserConnection,
		sqlConnection:  params.SQLConnection,
		dbType:         params.DBType,
		version:        params.Version,
	}
}

func (rt *sqlremote) getTableNames() ([]string, error) {
	// Get table list
	query := ""
	if rt.dbType == db.MYSQLDBType {
		query = "SHOW TABLES"
	} else if rt.dbType == db.PGDBType {
		query = fmt.Sprintf("SELECT tablename FROM pg_catalog.pg_tables where schemaname = '%s';", rt.userConnection.DbSchema)
	}

	data := []string{}
	err := rt.sqlConnection.Select(&data, query)

	if err != nil {
		return nil, fmt.Errorf("error getting table names: %v", err)
	}
	return data, nil
}
