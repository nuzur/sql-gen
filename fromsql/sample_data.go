package fromsql

import (
	"fmt"

	"github.com/nuzur/sql-gen/db"
)

func (rt *sqlremote) sampleTableValues(name string) (remoteRows, error) {
	// Get Data
	query := ""
	if rt.dbType == db.MYSQLDBType {
		query = fmt.Sprintf("SELECT * FROM `%s` ORDER BY RAND() LIMIT 10", name)
	} else if rt.dbType == db.PGDBType {
		query = fmt.Sprintf(`SELECT * FROM "%s" ORDER BY RANDOM() LIMIT 10`, name)
	}

	rows, err := rt.sqlConnection.Queryx(query)
	if err != nil {
		return nil, fmt.Errorf("error getting sample data: %v | query:  %v", err, query)
	}
	data := []map[string]interface{}{}
	for rows.Next() {
		r := make(map[string]interface{})
		err := rows.MapScan(r)
		if err != nil {
			return nil, err
		}
		data = append(data, r)
	}
	return data, nil
}
