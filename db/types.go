package db

type DBType string

const (
	MYSQLDBType DBType = "mysql"
	PGDBType    DBType = "postgres"
)
