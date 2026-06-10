package fromsql

import "github.com/jmoiron/sqlx"

// FromSqlx wraps an existing *sqlx.DB so it satisfies the DB interface. Used
// by callers that already hold a concrete sqlx connection (remote DB paths,
// in-tree test rigs, etc.). LocalAgentConnection callers should NOT go through
// this — they have their own adapter that routes via the agent stream.
func FromSqlx(db *sqlx.DB) DB { return &sqlxAdapter{db: db} }

type sqlxAdapter struct{ db *sqlx.DB }

func (a *sqlxAdapter) Select(dest interface{}, query string, args ...interface{}) error {
	return a.db.Select(dest, query, args...)
}

func (a *sqlxAdapter) QueryMaps(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := a.db.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		m := map[string]interface{}{}
		if err := rows.MapScan(m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
