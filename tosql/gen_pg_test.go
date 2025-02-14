package tosql

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/gofrs/uuid"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/stretchr/testify/assert"
)

func TestGenPG(t *testing.T) {
	pvdata, err := os.ReadFile("./testdata/project_version.json")
	assert.NoError(t, err)
	projectVerion := &nemgen.ProjectVersion{}
	err = json.Unmarshal(pvdata, projectVerion)
	assert.NoError(t, err)
	req := GenerateRequest{
		ExecutionUUID: uuid.Must(uuid.NewV4()).String(),
		Configvalues: &ConfigValues{
			DBType: db.PGDBType,
			Entities: []string{
				"b8629dd5-f6e5-483f-893a-842357e171fc", "6f9ca9c7-6af3-4301-82d2-739ec84eab83",
			},
			Actions: []Action{
				CreateAction,
				DeleteAction,
				InsertAction,
				DeleteAction,
				SelectSimpleAction,
				SelectForIndexedSimpleAction,
				SelectForIndexedCombinedAction,
			},
		},
		ProjectVersion: projectVerion,
	}
	res, err := GenerateSQL(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	os.RemoveAll(res.WorkingDir)
	os.RemoveAll(res.ZipFile)

	insertsData, err := os.ReadFile("./testdata/inserts_pg.sql")
	assert.NoError(t, err)
	updatesData, err := os.ReadFile("./testdata/updates_pg.sql")
	assert.NoError(t, err)
	deletesData, err := os.ReadFile("./testdata/deletes_pg.sql")
	assert.NoError(t, err)
	createsData, err := os.ReadFile("./testdata/creates_pg.sql")
	assert.NoError(t, err)
	selectsSimpleData, err := os.ReadFile("./testdata/selects_simple_pg.sql")
	assert.NoError(t, err)
	selectsIndexedSimpleData, err := os.ReadFile("./testdata/selects_indexed_simple_pg.sql")
	assert.NoError(t, err)
	selectsIndexedCombinedData, err := os.ReadFile("./testdata/selects_indexed_combined_pg.sql")
	assert.NoError(t, err)

	for _, db := range res.Results {
		switch db.Action {
		case "insert":
			assert.Equal(t, string(insertsData), db.Data)
		case "update":
			assert.Equal(t, string(updatesData), db.Data)
		case "delete":
			assert.Equal(t, string(deletesData), db.Data)
		case "create":
			assert.Equal(t, string(createsData), db.Data)
		case "select-simple":
			assert.Equal(t, string(selectsSimpleData), db.Data)
		case "select-indexed-simple":
			assert.Equal(t, string(selectsIndexedSimpleData), db.Data)
		case "select-indexed-combined":
			assert.Equal(t, string(selectsIndexedCombinedData), db.Data)
		}
	}

}
