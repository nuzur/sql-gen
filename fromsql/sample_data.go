package fromsql

import (
	"fmt"

	"github.com/nuzur/sql-gen/db"
)

func (rt *sqlremote) sampleTableValues(name string) (remoteRows, error) {
	// Get Data
	query := ""
	if rt.dbType == db.MYSQLDBType {
		query = fmt.Sprintf("SELECT * FROM `%s` LIMIT 10", name)
	} else if rt.dbType == db.PGDBType {
		query = fmt.Sprintf(`SELECT * FROM "%s" LIMIT 10`, name)
	}

	data, err := rt.db.QueryMaps(query)
	if err != nil {
		return nil, fmt.Errorf("error getting sample data: %v | query:  %v", err, query)
	}
	return data, nil
}
