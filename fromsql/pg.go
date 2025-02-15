package fromsql

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type pgColumnDetails struct {
	Name             string  `db:"COLUMN_NAME"`
	DataType         string  `db:"DATA_TYPE"` // just the type
	DefaultValue     *string `db:"COLUMN_DEFAULT"`
	IsNullable       string  `db:"IS_NULLABLE"`
	CharMax          *int64  `db:"CHARACTER_MAXIMUM_LENGTH"`
	NumericPrecision *int64  `db:"NUMERIC_PRECISION"`
}

type pgIndexDetails struct {
	Name       string `db:"index_name"`
	Seq        int64  `db:"index_order"`
	ColumnName string `db:"index_column"`
	IsKey      bool   `db:"is_key"`
	IsUnique   bool   `db:"is_unique"`
	Ascending  bool   `db:"ascending"`
}

func (rt *sqlremote) buildProjectVersionFromPg() (*nemgen.ProjectVersion, error) {
	tableNames, err := rt.getTableNames()
	if err != nil {
		return nil, err
	}

	entities := []*nemgen.Entity{}
	for _, tableName := range tableNames {
		e, err := rt.buildEntityFromPg(tableName)
		if err != nil {
			return nil, err
		}
		entities = append(entities, e)
	}

	return &nemgen.ProjectVersion{
		Uuid:          uuid.Must(uuid.NewV4()).String(),
		Version:       time.Now().Unix(),
		Entities:      entities,
		Status:        nemgen.ProjectVersionStatus_PROJECT_VERSION_STATUS_ACTIVE,
		Relationships: []*nemgen.Relationship{}, // todo build relationships
	}, nil
}

func (rt *sqlremote) buildEntityFromPg(tableName string) (*nemgen.Entity, error) {

	indexDetails, err := rt.fetchPgIndexDetails(tableName)
	if err != nil {
		return nil, err
	}

	fields, err := rt.buildFieldsFromPg(tableName, indexDetails)
	if err != nil {
		return nil, err
	}

	indexes, err := rt.buildIndexesFromPg(indexDetails, fields)
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

func (rt *sqlremote) buildFieldsFromPg(tableName string, indexDetails []*pgIndexDetails) ([]*nemgen.Field, error) {
	columnsQuery := fmt.Sprintf(
		`SELECT column_name,
				data_type,
				column_default,
				is_nullable,
				character_maximum_length,
				numeric_precision
				FROM information_schema.columns
				WHERE table_schema = '%s' 
				AND table_name = '%s'
				ORDER BY ordinal_position;`,
		rt.userConnection.DbSchema,
		tableName,
	)

	sampleData, err := rt.sampleTableValues(tableName)
	if err != nil {
		return nil, err
	}

	var columnsDetails []*pgColumnDetails = []*pgColumnDetails{}
	err = rt.sqlConnection.Select(&columnsDetails, columnsQuery)
	if err != nil {
		return nil, fmt.Errorf("error getting columns: %v", err)
	}

	fields := []*nemgen.Field{}
	for _, columnDetails := range columnsDetails {
		f := mapPgColumnDetailsToField(columnDetails, sampleData, indexDetails)
		if f != nil {
			fields = append(fields, f)
		}
	}
	return fields, nil
}

func (rt *sqlremote) fetchPgIndexDetails(tableName string) ([]*pgIndexDetails, error) {
	indexesQuery := fmt.Sprintf(`
			SELECT i.indexrelid::regclass AS index_name,                                    
				k.i AS index_order,                                                                                                             
				coalesce(a.attname,                                                     
							(('{' || pg_get_expr(                                          
										i.indexprs,                                        
										i.indrelid                                         
									)                                                     
								|| '}')::text[]                                          
							)[k.i]                                                         
						) AS index_column,                                              
				i.indoption[k.i - 1] = 0 AS ascending,                                  
				i.indisprimary AS is_key, 
				i.indisunique as is_unique
			FROM pg_index i                                                                
			CROSS JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, i)         
			LEFT JOIN pg_attribute AS a                                                 
				ON i.indrelid = a.attrelid AND k.attnum = a.attnum                       
			WHERE i.indrelid = '%s'::regclass;;
			`,
		//rt.userConnection.DbSchema,
		tableName)

	var indexesDetails []*pgIndexDetails = []*pgIndexDetails{}
	err := rt.sqlConnection.Select(&indexesDetails, indexesQuery)
	if err != nil {
		return nil, fmt.Errorf("error getting indexes: %v", err)
	}
	return indexesDetails, nil
}

func (rt *sqlremote) buildIndexesFromPg(indexesDetails []*pgIndexDetails, fields []*nemgen.Field) ([]*nemgen.Index, error) {

	// group indexes by name
	groupedIndexesDetails := make(map[string][]*pgIndexDetails)
	for _, indexDetails := range indexesDetails {
		arr, found := groupedIndexesDetails[indexDetails.Name]
		if !found {
			arr = []*pgIndexDetails{}
		}
		arr = append(arr, indexDetails)
		groupedIndexesDetails[indexDetails.Name] = arr
	}

	indexes := []*nemgen.Index{}
	for _, groupedDetails := range groupedIndexesDetails {
		i := mapPgIndexDetailsToIndex(groupedDetails, fields)
		if i != nil {
			indexes = append(indexes, i)
		}
	}

	return indexes, nil
}

func mapPgColumnDetailsToField(in *pgColumnDetails, sampleData remoteRows, indexDetails []*pgIndexDetails) *nemgen.Field {
	if in == nil {
		return &nemgen.Field{}
	}

	isKey := false
	isUnique := false
	for _, id := range indexDetails {
		if id.ColumnName == in.Name {
			if id.IsKey {
				isKey = true
			}
			if id.IsUnique {
				isUnique = true
			}
		}
	}

	fieldType, fieldTypeConfig := mapPgColumnDataTypeToFieldType(in, sampleData)
	return &nemgen.Field{
		Uuid:       uuid.Must(uuid.NewV4()).String(),
		Version:    time.Now().Unix(),
		Identifier: in.Name,
		Required:   in.IsNullable == "NO",
		Type:       fieldType,
		TypeConfig: fieldTypeConfig,
		Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
		Key:        isKey,
		Unique:     isUnique,
	}
}

func mapPgColumnDataTypeToFieldType(in *pgColumnDetails, sampleData remoteRows) (nemgen.FieldType, *nemgen.FieldTypeConfig) {
	if in == nil {
		return nemgen.FieldType_FIELD_TYPE_INVALID, nil
	}
	dataType := strings.ToLower(in.DataType)
	switch dataType {
	case "uuid":
		return nemgen.FieldType_FIELD_TYPE_UUID, nil
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
	case "boolean":
		return nemgen.FieldType_FIELD_TYPE_BOOLEAN, nil
	case "smallint":
		return nemgen.FieldType_FIELD_TYPE_INTEGER, &nemgen.FieldTypeConfig{
			Integer: &nemgen.FieldTypeIntegerConfig{
				Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTEEN_BITS,
			},
		}
	case "integer":
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
	case "bytea":
		var max int64 = 65535
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
	case "timestamp":
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

func mapPgIndexDetailsToIndex(in []*pgIndexDetails, fields []*nemgen.Field) *nemgen.Index {
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
