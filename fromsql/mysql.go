package fromsql

import (
	"fmt"
	"slices"
	"strings"
	"time"

	nemgen "github.com/nuzur/nem/idl/gen"

	"github.com/gofrs/uuid"
)

type mysqlColumnDetails struct {
	Name             string  `db:"COLUMN_NAME"`
	DataType         string  `db:"DATA_TYPE"`   // just the type
	ColumnType       string  `db:"COLUMN_TYPE"` // full type with size
	ColumnKey        string  `db:"COLUMN_KEY"`
	DefaultValue     *string `db:"COLUMN_DEFAULT"`
	IsNullable       string  `db:"IS_NULLABLE"`
	CharMax          *int64  `db:"CHARACTER_MAXIMUM_LENGTH"`
	NumericPrecision *int64  `db:"NUMERIC_PRECISION"`
}

type mysqlIndexDetails struct {
	Name       string `db:"INDEX_NAME"`
	Seq        int64  `db:"SEQ_IN_INDEX"`
	NonUnique  bool   `db:"NON_UNIQUE"`
	ColumnName string `db:"COLUMN_NAME"`
}

type mysqlForeignKeyDetails struct {
	ConstraintName       string `db:"CONSTRAINT_NAME"`
	ColumnName           string `db:"COLUMN_NAME"`
	ReferencedColumnName string `db:"REFERENCED_COLUMN_NAME"`
	ReferencedTableName  string `db:"REFERENCED_TABLE_NAME"`
}

func (rt *sqlremote) buildProjectVersionFromMysql() (*nemgen.ProjectVersion, error) {
	tableNames, err := rt.getTableNames()
	if err != nil {
		return nil, err
	}

	entities := []*nemgen.Entity{}
	for _, tableName := range tableNames {
		e, err := rt.buildEntityFromMysql(tableName)
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}

	relationships := []*nemgen.Relationship{}
	for _, e := range entities {
		rels, err := rt.buildRelationshipsFromMysql(e.Identifier, entities)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, rels...)
	}

	return &nemgen.ProjectVersion{
		Uuid:          uuid.Must(uuid.NewV4()).String(),
		Version:       time.Now().Unix(),
		Entities:      entities,
		Status:        nemgen.ProjectVersionStatus_PROJECT_VERSION_STATUS_ACTIVE,
		Relationships: relationships,
	}, nil
}

func (rt *sqlremote) buildRelationshipsFromMysql(tableName string, entities []*nemgen.Entity) ([]*nemgen.Relationship, error) {
	foreignKeysQuery := fmt.Sprintf(`
		SELECT 
			tc.CONSTRAINT_NAME, 
			kcu.COLUMN_NAME, 
			kcu.REFERENCED_COLUMN_NAME, 
			kcu.REFERENCED_TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
		JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu ON (
			tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME AND
			tc.TABLE_NAME = kcu.TABLE_NAME AND
			tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA)
		WHERE 
			tc.CONSTRAINT_TYPE='FOREIGN KEY' AND
			tc.TABLE_NAME = '%s' AND
			tc.TABLE_SCHEMA = '%s' 
		ORDER BY ORDINAL_POSITION`,
		rt.userConnection.DbSchema,
		tableName,
	)

	var fkDetails []*mysqlForeignKeyDetails = []*mysqlForeignKeyDetails{}
	err := rt.sqlConnection.Select(&fkDetails, foreignKeysQuery)
	if err != nil {
		return nil, fmt.Errorf("error getting constraint details: %v", err)
	}

	rels := []*nemgen.Relationship{}
	for _, fkd := range fkDetails {
		rels = append(rels, mapMysqlFKDetailsToRelationship(fkd, tableName, entities))
	}

	return rels, nil
}

func (rt *sqlremote) buildEntityFromMysql(tableName string) (*nemgen.Entity, error) {

	fields, err := rt.buildFieldsFromMysql(tableName)
	if err != nil {
		return nil, err
	}

	indexes, err := rt.buildIndexesFromMysql(tableName, fields)
	if err != nil {
		return nil, err
	}

	return &nemgen.Entity{
		Uuid:       uuid.Must(uuid.NewV4()).String(),
		Version:    time.Now().Unix(),
		Identifier: tableName,
		Fields:     fields,
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: indexes,
			},
		},
		Status: nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
	}, nil

}

func (rt *sqlremote) buildFieldsFromMysql(tableName string) ([]*nemgen.Field, error) {
	columnsQuery := fmt.Sprintf(`
		SELECT COLUMN_NAME,
			   	DATA_TYPE,
				COLUMN_TYPE,
				COLUMN_KEY,
			   	COLUMN_DEFAULT,			    
				IS_NULLABLE,
				CHARACTER_MAXIMUM_LENGTH,
				NUMERIC_PRECISION 
		FROM INFORMATION_SCHEMA.columns
		WHERE 
			TABLE_SCHEMA = '%s'
			AND TABLE_NAME = '%s'
		ORDER BY ORDINAL_POSITION`,
		rt.userConnection.DbSchema,
		tableName,
	)

	var columnsDetails []*mysqlColumnDetails = []*mysqlColumnDetails{}
	err := rt.sqlConnection.Select(&columnsDetails, columnsQuery)
	if err != nil {
		return nil, fmt.Errorf("error getting columns: %v", err)
	}

	sampleData, err := rt.sampleTableValues(tableName)
	if err != nil {
		return nil, err
	}

	fields := []*nemgen.Field{}
	for _, columnDetails := range columnsDetails {
		f := mapMysqlColumnDetailsToField(columnDetails, sampleData)
		if f != nil {
			fields = append(fields, f)
		}
	}
	return fields, nil
}

func (rt *sqlremote) buildIndexesFromMysql(tableName string, fields []*nemgen.Field) ([]*nemgen.Index, error) {
	indexesQuery := fmt.Sprintf(`
		SELECT DISTINCT
			INDEX_NAME,
			SEQ_IN_INDEX,
			NON_UNIQUE,
			COLUMN_NAME
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE 
			TABLE_SCHEMA = '%s'
			AND table_name = '%s'`,
		rt.userConnection.DbSchema,
		tableName)

	var indexesDetails []*mysqlIndexDetails = []*mysqlIndexDetails{}
	err := rt.sqlConnection.Select(&indexesDetails, indexesQuery)
	if err != nil {
		return nil, fmt.Errorf("error getting indexes: %v", err)
	}

	// group indexes by name
	groupedIndexesDetails := make(map[string][]*mysqlIndexDetails)
	for _, indexDetails := range indexesDetails {
		arr, found := groupedIndexesDetails[indexDetails.Name]
		if !found {
			arr = []*mysqlIndexDetails{}
		}
		arr = append(arr, indexDetails)
		groupedIndexesDetails[indexDetails.Name] = arr
	}

	indexes := []*nemgen.Index{}
	for _, groupedDetails := range groupedIndexesDetails {
		i := mapMysqlIndexDetailsToIndex(groupedDetails, fields)
		if i != nil {
			indexes = append(indexes, i)
		}
	}

	return indexes, nil
}

func mapMysqlColumnDetailsToField(in *mysqlColumnDetails, sampleData remoteRows) *nemgen.Field {
	if in == nil {
		return &nemgen.Field{}
	}

	fieldType, fieldTypeConfig := mapMysqlColumnDataTypeToFieldType(in, sampleData)
	return &nemgen.Field{
		Uuid:       uuid.Must(uuid.NewV4()).String(),
		Version:    time.Now().Unix(),
		Identifier: in.Name,
		Required:   in.IsNullable == "NO",
		Type:       fieldType,
		TypeConfig: fieldTypeConfig,
		Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
		Key:        in.ColumnKey == "PRI",
		Unique:     in.ColumnKey == "UNI",
	}
}

func mapMysqlColumnDataTypeToFieldType(in *mysqlColumnDetails, sampleData remoteRows) (nemgen.FieldType, *nemgen.FieldTypeConfig) {
	if in == nil {
		return nemgen.FieldType_FIELD_TYPE_INVALID, nil
	}
	dataType := strings.ToLower(in.DataType)
	switch dataType {
	case "char":
		var max int64 = 0
		if in.CharMax != nil {
			max = *in.CharMax
		}
		if max == 36 {
			if sampleData.isUUID(in.Name) {
				return nemgen.FieldType_FIELD_TYPE_UUID, &nemgen.FieldTypeConfig{}
			}
		}
		return nemgen.FieldType_FIELD_TYPE_CHAR, &nemgen.FieldTypeConfig{
			Char: &nemgen.FieldTypeCharConfig{
				MaxSize: max,
			},
		}
	case "tinyint":
		if strings.ToLower(in.ColumnType) == "tinyint(1)" {
			return nemgen.FieldType_FIELD_TYPE_BOOLEAN, nil
		}
		return nemgen.FieldType_FIELD_TYPE_INTEGER, &nemgen.FieldTypeConfig{
			Integer: &nemgen.FieldTypeIntegerConfig{
				Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_EIGHT_BITS,
			},
		}
	case "smallint":
		return nemgen.FieldType_FIELD_TYPE_INTEGER, &nemgen.FieldTypeConfig{
			Integer: &nemgen.FieldTypeIntegerConfig{
				Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTEEN_BITS,
			},
		}
	case "mediumint":
		return nemgen.FieldType_FIELD_TYPE_INTEGER, &nemgen.FieldTypeConfig{
			Integer: &nemgen.FieldTypeIntegerConfig{
				Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_TWENTY_FOUR_BITS,
			},
		}
	case "int":
		return nemgen.FieldType_FIELD_TYPE_INTEGER, &nemgen.FieldTypeConfig{
			Integer: &nemgen.FieldTypeIntegerConfig{
				Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS,
			},
		}
	case "bigint":
		return nemgen.FieldType_FIELD_TYPE_INTEGER, &nemgen.FieldTypeConfig{
			Integer: &nemgen.FieldTypeIntegerConfig{
				Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTY_FOUR_BITS,
			},
		}
	case "double":
		return nemgen.FieldType_FIELD_TYPE_FLOAT, nil
	case "decimal":
		return nemgen.FieldType_FIELD_TYPE_DECIMAL, nil

	case "varchar":
		var max int64 = 255
		if in.CharMax != nil {
			max = *in.CharMax
		}

		if sampleData.isEmail(in.Name) {
			return nemgen.FieldType_FIELD_TYPE_EMAIL, nil
		}
		if sampleData.isURL(in.Name) {
			return nemgen.FieldType_FIELD_TYPE_URL, nil
		}
		return nemgen.FieldType_FIELD_TYPE_VARCHAR, &nemgen.FieldTypeConfig{
			Varchar: &nemgen.FieldTypeVarcharConfig{
				MaxSize: max,
			},
		}
	case "tinytext":
		var max int64 = 255
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_TEXT, &nemgen.FieldTypeConfig{
			Text: &nemgen.FieldTypeTextConfig{
				MaxSize: max,
			},
		}
	case "text":
		var max int64 = 65535
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_TEXT, &nemgen.FieldTypeConfig{
			Text: &nemgen.FieldTypeTextConfig{
				MaxSize: max,
			},
		}
	case "mediumtext":
		var max int64 = 16777215
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_TEXT, &nemgen.FieldTypeConfig{
			Text: &nemgen.FieldTypeTextConfig{
				MaxSize: max,
			},
		}
	case "longtext":
		var max int64 = 4294967295
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_TEXT, &nemgen.FieldTypeConfig{
			Text: &nemgen.FieldTypeTextConfig{
				MaxSize: max,
			},
		}

	case "tinyblob":
		var max int64 = 255
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_FILE, &nemgen.FieldTypeConfig{
			File: &nemgen.FieldTypeFileConfig{
				MaxSize: max,
			},
		}
	case "blob":
		var max int64 = 65535
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_FILE, &nemgen.FieldTypeConfig{
			File: &nemgen.FieldTypeFileConfig{
				MaxSize: max,
			},
		}
	case "mediumblob":
		var max int64 = 16777215
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_FILE, &nemgen.FieldTypeConfig{
			File: &nemgen.FieldTypeFileConfig{
				MaxSize: max,
			},
		}
	case "longblob":
		var max int64 = 4294967295
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_FILE, &nemgen.FieldTypeConfig{
			File: &nemgen.FieldTypeFileConfig{
				MaxSize: max,
			},
		}
	case "json":
		if sampleData.isJSONArray(in.Name) {
			return nemgen.FieldType_FIELD_TYPE_ARRAY, nil
		}
		return nemgen.FieldType_FIELD_TYPE_JSON, nil
	case "date":
		return nemgen.FieldType_FIELD_TYPE_DATE, nil
	case "datetime":
		return nemgen.FieldType_FIELD_TYPE_DATETIME, nil
	case "time":
		return nemgen.FieldType_FIELD_TYPE_TIME, nil
	}
	return nemgen.FieldType_FIELD_TYPE_INVALID, nil

	// possibly analize the content for these in the future
	// nemgen.FieldType_FIELD_TYPE_RICHTEXT, // 15
	// nemgen.FieldType_FIELD_TYPE_CODE,     // 16
	// nemgen.FieldType_FIELD_TYPE_MARKDOWN: // 17
	// nemgen.FieldType_FIELD_TYPE_ENCRYPTED: // 9
	// nemgen.FieldType_FIELD_TYPE_PHONE: // 11
	// nemgen.FieldType_FIELD_TYPE_LOCATION: // 13
	// nemgen.FieldType_FIELD_TYPE_COLOR: // 14
	// nemgen.FieldType_FIELD_TYPE_SLUG: // 28
}

func mapMysqlIndexDetailsToIndex(in []*mysqlIndexDetails, fields []*nemgen.Field) *nemgen.Index {
	if len(in) == 0 {
		return nil
	}

	first := in[0]

	columns := []string{}
	for _, id := range in {
		columns = append(columns, id.ColumnName)
	}

	indexFields := make(map[string]*nemgen.Field)
	indexType := nemgen.IndexType_INDEX_TYPE_INDEX
	for _, f := range fields {
		if slices.Contains(columns, f.Identifier) {
			indexFields[f.Identifier] = f
			if f.Key {
				indexType = nemgen.IndexType_INDEX_TYPE_PRIMARY
			}

			if f.Unique {
				indexType = nemgen.IndexType_INDEX_TYPE_UNIQUE
			}
		}
	}

	finalIndexFields := []*nemgen.IndexField{}
	for _, id := range in {
		finalIndexFields = append(finalIndexFields, &nemgen.IndexField{
			FieldUuid: indexFields[id.ColumnName].Uuid,
			Priority:  id.Seq,
			Order:     nemgen.IndexFieldOrder_INDEX_FIELD_ORDER_DESC,
		})
	}

	return &nemgen.Index{
		Uuid:       uuid.Must(uuid.NewV4()).String(),
		Identifier: first.Name,
		Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
		Type:       indexType,
		Fields:     finalIndexFields,
	}
}

func mapMysqlFKDetailsToRelationship(in *mysqlForeignKeyDetails, tableName string, entities []*nemgen.Entity) *nemgen.Relationship {
	if in == nil {
		return nil
	}

	var fromEntity *nemgen.Entity
	var toEntity *nemgen.Entity
	for _, e := range entities {
		if e.Identifier == tableName {
			fromEntity = e
		}
		if e.Identifier == in.ReferencedTableName {
			toEntity = e
		}
	}

	var fromField *nemgen.Field
	for _, f := range fromEntity.Fields {
		if f.Identifier == in.ColumnName {
			fromField = f
			break
		}
	}

	var toField *nemgen.Field
	for _, f := range toEntity.Fields {
		if f.Identifier == in.ColumnName {
			toField = f
			break
		}
	}

	return &nemgen.Relationship{
		Uuid:       uuid.Must(uuid.NewV4()).String(),
		Version:    time.Now().Unix(),
		Identifier: in.ConstraintName,
		From: &nemgen.RelationshipNode{
			Uuid: uuid.Must(uuid.NewV4()).String(),
			Type: nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY,
			TypeConfig: &nemgen.RelationshipNodeTypeConfig{
				Entity: &nemgen.RelationshipNodeTypeEntityConfig{
					EntityUuid: fromEntity.Uuid,
					FieldUuids: []string{fromField.Uuid},
				},
			},
		},
		To: &nemgen.RelationshipNode{
			Uuid: uuid.Must(uuid.NewV4()).String(),
			Type: nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY,
			TypeConfig: &nemgen.RelationshipNodeTypeConfig{
				Entity: &nemgen.RelationshipNodeTypeEntityConfig{
					EntityUuid: toEntity.Uuid,
					FieldUuids: []string{toField.Uuid},
				},
			},
		},
		Status: nemgen.RelationshipStatus_RELATIONSHIP_STATUS_ACTIVE,
	}

}
