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
	projectVersion := &nemgen.ProjectVersion{}
	err = json.Unmarshal(pvdata, projectVersion)
	assert.NoError(t, err)
	req := GenerateRequest{
		ExecutionUUID: uuid.Must(uuid.NewV4()).String(),
		Configvalues: &ConfigValues{
			DBType: db.PGDBType,
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
		ProjectVersion: projectVersion,
		//ForGolang:      true,
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

	// The cases are the Action constants themselves. Spelling them out as
	// literals let the select ones drift to "select-indexed-simple" while the
	// action is "select_indexed_simple", so those three golden files were never
	// compared against anything and silently went stale.
	asserted := map[Action]bool{}
	for _, db := range res.Results {
		asserted[db.Action] = true
		switch db.Action {
		case InsertAction:
			assert.Equal(t, string(insertsData), db.Data)
		case UpdateAction:
			assert.Equal(t, string(updatesData), db.Data)
		case DeleteAction:
			assert.Equal(t, string(deletesData), db.Data)
		case CreateAction:
			assert.Equal(t, string(createsData), db.Data)
		case SelectSimpleAction:
			assert.Equal(t, string(selectsSimpleData), db.Data)
		case SelectForIndexedSimpleAction:
			assert.Equal(t, string(selectsIndexedSimpleData), db.Data)
		case SelectForIndexedCombinedAction:
			assert.Equal(t, string(selectsIndexedCombinedData), db.Data)
		}
	}

	// Every requested action must come back, or an assertion above simply never
	// ran for it.
	for _, action := range req.Configvalues.Actions {
		assert.True(t, asserted[action], "no result for action %s", action)
	}

}
