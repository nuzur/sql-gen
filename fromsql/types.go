package fromsql

import (
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
)

// DB is the minimal database surface fromsql needs to introspect a live SQL
// schema. It's intentionally narrow so that callers can satisfy it via any
// underlying transport — a direct *sqlx.DB for remote connections, or a
// LocalAgentChannel-routed adapter for agent-backed local connections.
//
// Caller responsibility: each method may be invoked with no explicit context,
// so the underlying transport should use a reasonable default deadline.
type DB interface {
	// Select scans a result set into a slice destination. Accepts both
	// scalar slices (`*[]string`) and struct slices with `db:"col"` tags,
	// matching sqlx's behavior.
	Select(dest interface{}, query string, args ...interface{}) error

	// QueryMaps returns each row as a column-name → value map. Used by the
	// sample-data path where the result schema isn't known at compile time.
	QueryMaps(query string, args ...interface{}) ([]map[string]interface{}, error)
}

type GenerateRequest struct {
	UserConnection *nemgen.UserConnection
	DB             DB
	DBType         db.DBType
	Version        *int64
}

type sqlremote struct {
	userConnection *nemgen.UserConnection
	db             DB
	dbType         db.DBType
	version        *int64
}

type remoteRows []map[string]interface{}
