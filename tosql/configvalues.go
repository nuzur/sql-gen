package tosql

import "github.com/nuzur/sql-gen/db"

type Action string

const (
	SelectSimpleAction             Action = "select_simple"
	SelectForIndexedSimpleAction   Action = "select_indexed_simple"
	SelectForIndexedCombinedAction Action = "select_indexed_combined"
	InsertAction                   Action = "insert"
	UpdateAction                   Action = "update"
	DeleteAction                   Action = "delete"
	CreateAction                   Action = "create"
)

type ConfigValues struct {
	DBType   db.DBType `json:"db_type"`
	Entities []string  `json:"entities"`
	Actions  []Action  `json:"actions"`
}
