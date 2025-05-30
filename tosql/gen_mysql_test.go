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

func TestGenMysql(t *testing.T) {
	pvdata, err := os.ReadFile("./testdata/project_version.json")
	assert.NoError(t, err)
	projectVerion := &nemgen.ProjectVersion{}
	err = json.Unmarshal(pvdata, projectVerion)
	assert.NoError(t, err)
	req := GenerateRequest{
		ExecutionUUID: uuid.Must(uuid.NewV4()).String(),
		Configvalues: &ConfigValues{
			DBType: db.MYSQLDBType,
			Entities: []string{
				"b8629dd5-f6e5-483f-893a-842357e171fc",
				"6f9ca9c7-6af3-4301-82d2-739ec84eab83",
				"de4f4b45-79b5-4f6b-9a2e-2d2d3a660aae",
				"e3b0c442-98fc-4c2a-9c4e-8f8f8f8f8f8f",
			},
			Actions: []Action{
				CreateAction,
				DeleteAction,
				InsertAction,
				UpdateAction,
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

	insertsData, err := os.ReadFile("./testdata/inserts_mysql.sql")
	assert.NoError(t, err)
	updatesData, err := os.ReadFile("./testdata/updates_mysql.sql")
	assert.NoError(t, err)
	deletesData, err := os.ReadFile("./testdata/deletes_mysql.sql")
	assert.NoError(t, err)
	createsData, err := os.ReadFile("./testdata/creates_mysql.sql")
	assert.NoError(t, err)
	selectsSimpleData, err := os.ReadFile("./testdata/selects_simple_mysql.sql")
	assert.NoError(t, err)
	selectsIndexedSimpleData, err := os.ReadFile("./testdata/selects_indexed_simple_mysql.sql")
	assert.NoError(t, err)
	selectsIndexedCombinedData, err := os.ReadFile("./testdata/selects_indexed_combined_mysql.sql")
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
