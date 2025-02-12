package fromsql

import (
	"fmt"
)

func (rt *sqlremote) sampleTableValues(name string) (remoteRows, error) {
	// Get Data
	query := fmt.Sprintf(`SELECT * FROM %s AS t1 JOIN 
		(SELECT id FROM %s ORDER BY RAND() LIMIT 10) 
		as t2 ON t1.id=t2.id`, name, name)

	rows, err := rt.sqlConnection.Queryx(query)
	data := []map[string]interface{}{}
	for rows.Next() {
		r := make(map[string]interface{})
		err := rows.MapScan(r)
		if err != nil {
			return nil, err
		}
		data = append(data, r)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting sample data: %v", err)
	}
	return data, nil
}
