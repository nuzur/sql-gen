package fromsql

import (
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/tosql"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	NumericScale     *int64  `db:"NUMERIC_SCALE"`
	// DatetimePrecision is the fractional-second precision (fsp) of a
	// DATETIME/TIMESTAMP/TIME column. NULL for every other type. mysql's default
	// fsp is 0, so a bare DATETIME reports 0 — which is exactly the "unset"
	// precision the model renders bare.
	DatetimePrecision *int64 `db:"DATETIME_PRECISION"`
	// Extra carries "DEFAULT_GENERATED" for an expression default and
	// "on update CURRENT_TIMESTAMP" for the auto-refresh clause. Neither is
	// visible anywhere else in information_schema, so without it an expression
	// default reconstructs as a string literal and ON UPDATE is lost entirely.
	Extra string `db:"EXTRA"`
}

type mysqlIndexDetails struct {
	Name           string `db:"INDEX_NAME"`
	Seq            int64  `db:"SEQ_IN_INDEX"`
	NonUnique      bool   `db:"NON_UNIQUE"`
	ColumnName     string `db:"COLUMN_NAME"`
	ConstraintType string `db:"CONSTRAINT_TYPE"`
	// IndexType is the storage/algorithm kind: BTREE, FULLTEXT, SPATIAL or HASH.
	// TABLE_CONSTRAINTS knows nothing about FULLTEXT — it reports one as a plain
	// INDEX — so without this a live FULLTEXT index reconstructs as an ordinary
	// one and the differ proposes DROP KEY / ADD FULLTEXT KEY on every plan.
	IndexType string `db:"INDEX_TYPE"`
	// Collation is 'A' (ascending), 'D' (descending) or NULL (not sorted).
	Collation sql.NullString `db:"COLLATION"`
	// SubPart is the indexed prefix length in characters, NULL when the whole
	// column is indexed.
	SubPart sql.NullInt64 `db:"SUB_PART"`
}

type mysqlForeignKeyDetails struct {
	ConstraintName       string `db:"CONSTRAINT_NAME"`
	ColumnName           string `db:"COLUMN_NAME"`
	ReferencedColumnName string `db:"REFERENCED_COLUMN_NAME"`
	ReferencedTableName  string `db:"REFERENCED_TABLE_NAME"`
	// DeleteRule / UpdateRule are the referential actions, which live in
	// REFERENTIAL_CONSTRAINTS rather than TABLE_CONSTRAINTS. A constraint
	// created without an explicit clause reports "NO ACTION".
	DeleteRule string `db:"DELETE_RULE"`
	UpdateRule string `db:"UPDATE_RULE"`
}

func (rt *sqlremote) buildProjectVersionFromMysql() (*nemgen.ProjectVersion, error) {
	if rt.userConnection.DbSchema == "" {
		res, err := rt.db.QueryMaps("SELECT DATABASE()")
		if err == nil && len(res) > 0 {
			for _, v := range res[0] {
				if str, ok := v.(string); ok {
					rt.userConnection.DbSchema = str
					break
				}
			}
		}
	}

	tableNames, err := rt.getTableNames()
	if err != nil {
		return nil, err
	}

	eg := errgroup.Group{}
	mu := &sync.Mutex{}
	entities := []*nemgen.Entity{}
	for _, tableName := range tableNames {
		eg.Go(func() error {
			e, err := rt.buildEntityFromMysql(tableName)
			if err != nil {
				return err
			}
			mu.Lock()
			entities = append(entities, e)
			mu.Unlock()
			return nil
		})
	}
	err = eg.Wait()
	if err != nil {
		return nil, err
	}

	// The tables are introspected concurrently, so they land in the slice in
	// whatever order they finished. tosql's topological sort uses input order as
	// its tie-break, which would make the emitted DDL differ run to run — and the
	// MySQL diff, which compares re-rendered DDL against the model's, would read
	// that as a change on every plan.
	sort.Slice(entities, func(a, b int) bool {
		return entities[a].Identifier < entities[b].Identifier
	})

	eg = errgroup.Group{}
	relationships := []*nemgen.Relationship{}
	for _, e := range entities {
		eg.Go(func() error {
			rels, err := rt.buildRelationshipsFromMysql(e.Identifier, entities)
			if err != nil {
				return err
			}
			mu.Lock()
			relationships = append(relationships, rels...)
			mu.Unlock()
			return nil
		})
	}
	err = eg.Wait()
	if err != nil {
		return nil, err
	}
	sort.Slice(relationships, func(a, b int) bool {
		return relationships[a].Identifier < relationships[b].Identifier
	})

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
			kcu.REFERENCED_TABLE_NAME,
			rc.DELETE_RULE,
			rc.UPDATE_RULE
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
		JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu ON (
			tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME AND
			tc.TABLE_NAME = kcu.TABLE_NAME AND
			tc.TABLE_SCHEMA = kcu.TABLE_SCHEMA)
		JOIN INFORMATION_SCHEMA.REFERENTIAL_CONSTRAINTS rc ON (
			rc.CONSTRAINT_NAME = tc.CONSTRAINT_NAME AND
			rc.TABLE_NAME = tc.TABLE_NAME AND
			rc.CONSTRAINT_SCHEMA = tc.TABLE_SCHEMA)
		WHERE 
			tc.CONSTRAINT_TYPE='FOREIGN KEY' AND
			tc.TABLE_SCHEMA = '%s' AND
			tc.TABLE_NAME = '%s' 
		ORDER BY ORDINAL_POSITION`,
		rt.userConnection.DbSchema,
		tableName,
	)

	var fkDetails []*mysqlForeignKeyDetails = []*mysqlForeignKeyDetails{}
	err := rt.db.Select(&fkDetails, foreignKeysQuery)
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
				NUMERIC_PRECISION,
				NUMERIC_SCALE,
				DATETIME_PRECISION,
				EXTRA
		FROM INFORMATION_SCHEMA.columns
		WHERE 
			TABLE_SCHEMA = '%s'
			AND TABLE_NAME = '%s'
		ORDER BY ORDINAL_POSITION`,
		rt.userConnection.DbSchema,
		tableName,
	)

	var columnsDetails []*mysqlColumnDetails = []*mysqlColumnDetails{}
	err := rt.db.Select(&columnsDetails, columnsQuery)
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
			s.INDEX_NAME,
			s.SEQ_IN_INDEX,
			s.NON_UNIQUE,
			s.COLUMN_NAME,
			s.COLLATION,
			s.INDEX_TYPE,
			s.SUB_PART,
			IFNULL(t.CONSTRAINT_TYPE, "INDEX") as CONSTRAINT_TYPE
		FROM
			INFORMATION_SCHEMA.STATISTICS s
				LEFT OUTER JOIN
			INFORMATION_SCHEMA.TABLE_CONSTRAINTS t 
				ON t.TABLE_SCHEMA = s.TABLE_SCHEMA
					AND t.TABLE_NAME = s.TABLE_NAME
					AND s.INDEX_NAME = t.CONSTRAINT_NAME
				LEFT OUTER JOIN 
			INFORMATION_SCHEMA.KEY_COLUMN_USAGE kcu
				ON kcu.constraint_name = s.index_name
		WHERE
			0 = 0 AND s.TABLE_SCHEMA = '%s'
				AND s.table_name = '%s'
		ORDER BY s.INDEX_NAME, s.SEQ_IN_INDEX`,
		rt.userConnection.DbSchema,
		tableName)

	var indexesDetails []*mysqlIndexDetails = []*mysqlIndexDetails{}
	err := rt.db.Select(&indexesDetails, indexesQuery)
	if err != nil {
		return nil, fmt.Errorf("error getting indexes: %v", err)
	}

	// mysql creates a supporting index for every foreign key that has none, and
	// names it after the constraint. It is not in the DDL that created the table
	// and the relationship already implies it, so reconstructing it as an index
	// of its own would put the model permanently one INDEX ahead of the DDL it
	// generates — a DROP KEY / ADD KEY the diff proposes on every plan. An index
	// the schema really declares survives, because mysql reuses an existing index
	// instead of creating one and the declared name is what STATISTICS reports.
	implicit, err := rt.foreignKeyConstraintNames(tableName)
	if err != nil {
		return nil, err
	}

	// group indexes by name
	groupedIndexesDetails := make(map[string][]*mysqlIndexDetails)
	for _, indexDetails := range indexesDetails {
		if implicit[indexDetails.Name] {
			continue
		}
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

	// Go randomizes map iteration, so without this the reconstructed schema
	// lists the same indexes in a different order on every introspection — and
	// the MySQL diff, which compares re-rendered DDL, reads that as a change.
	sort.Slice(indexes, func(a, b int) bool {
		return indexes[a].Identifier < indexes[b].Identifier
	})

	return indexes, nil
}

// foreignKeyConstraintNames is the set of foreign key constraint names on a
// table, which is also the set of names mysql gives the indexes it creates to
// support them.
func (rt *sqlremote) foreignKeyConstraintNames(tableName string) (map[string]bool, error) {
	query := fmt.Sprintf(`
		SELECT CONSTRAINT_NAME
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS
		WHERE CONSTRAINT_TYPE = 'FOREIGN KEY'
			AND TABLE_SCHEMA = '%s'
			AND TABLE_NAME = '%s'`,
		rt.userConnection.DbSchema,
		tableName)

	names := []string{}
	if err := rt.db.Select(&names, query); err != nil {
		return nil, fmt.Errorf("error getting foreign key constraint names: %v", err)
	}
	res := make(map[string]bool, len(names))
	for _, n := range names {
		res[n] = true
	}
	return res, nil
}

func mapMysqlColumnDetailsToField(in *mysqlColumnDetails, sampleData remoteRows) *nemgen.Field {
	if in == nil {
		return &nemgen.Field{}
	}

	fieldType, fieldTypeConfig := mapMysqlColumnDataTypeToFieldType(in, sampleData)
	// The column's DEFAULT has to come back out, or re-rendering the introspected
	// schema drops it and the diff proposes the same MODIFY COLUMN on every plan.
	// A datetime keeps its default in the field too rather than relying on the
	// implicit DEFAULT CURRENT_TIMESTAMP, so a column defaulted to something else
	// survives the round trip.
	def := mysqlColumnDefault(in.DefaultValue, in.Extra)
	return &nemgen.Field{
		Uuid:                     uuid.Must(uuid.NewV4()).String(),
		Version:                  time.Now().Unix(),
		Identifier:               in.Name,
		Required:                 in.IsNullable == "NO",
		Type:                     fieldType,
		TypeConfig:               fieldTypeConfig,
		Status:                   nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
		Key:                      in.ColumnKey == "PRI",
		Unique:                   in.ColumnKey == "UNI",
		DefaultValue:             def.Value,
		DefaultValueIsExpression: def.IsExpression,
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
		// The scale has to come back out or the round trip loses it, and a
		// decimal field with no number_of_decimals renders at the default
		// scale — which is a MODIFY COLUMN proposed on every plan, forever.
		var scale int64 = 0
		if in.NumericScale != nil {
			scale = *in.NumericScale
		}
		return nemgen.FieldType_FIELD_TYPE_DECIMAL, &nemgen.FieldTypeConfig{
			Decimal: &nemgen.FieldTypeDecimalConfig{
				NumberOfDecimals: scale,
			},
		}

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
				// The column holds the bytes themselves, so say so: an unset
				// storage_type now means object store (a url/key column).
				StorageType: nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY,
				MaxSize:     max,
			},
		}
	case "blob", "binary":
		var max int64 = 65535
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_FILE, &nemgen.FieldTypeConfig{
			File: &nemgen.FieldTypeFileConfig{
				// The column holds the bytes themselves, so say so: an unset
				// storage_type now means object store (a url/key column).
				StorageType: nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY,
				MaxSize:     max,
			},
		}
	case "mediumblob":
		var max int64 = 16777215
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_FILE, &nemgen.FieldTypeConfig{
			File: &nemgen.FieldTypeFileConfig{
				// The column holds the bytes themselves, so say so: an unset
				// storage_type now means object store (a url/key column).
				StorageType: nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY,
				MaxSize:     max,
			},
		}
	case "longblob":
		var max int64 = 4294967295
		if in.CharMax != nil {
			max = *in.CharMax
		}
		return nemgen.FieldType_FIELD_TYPE_FILE, &nemgen.FieldTypeConfig{
			File: &nemgen.FieldTypeFileConfig{
				// The column holds the bytes themselves, so say so: an unset
				// storage_type now means object store (a url/key column).
				StorageType: nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY,
				MaxSize:     max,
			},
		}
	case "json":
		if sampleData.isJSONArray(in.Name) {
			return nemgen.FieldType_FIELD_TYPE_ARRAY, nil
		}
		return nemgen.FieldType_FIELD_TYPE_JSON, nil
	case "date":
		return nemgen.FieldType_FIELD_TYPE_DATE, nil
	// mysql's TIMESTAMP is deliberately not handled here. It is a different type
	// from DATETIME (utc storage, a narrower range) and tosql has no way to emit
	// one, so mapping it onto a datetime field would make the plan propose a
	// MODIFY COLUMN converting every live TIMESTAMP column — a change nobody
	// asked for. It stays unmapped, as it was.
	case "datetime":
		var precision int64 = 0
		if in.DatetimePrecision != nil {
			precision = *in.DatetimePrecision
		}
		return nemgen.FieldType_FIELD_TYPE_DATETIME, &nemgen.FieldTypeConfig{
			Datetime: &nemgen.FieldTypeDatetimeConfig{
				Precision:                precision,
				OnUpdateCurrentTimestamp: mysqlOnUpdateCurrentTimestamp(in.Extra),
				// A datetime column the database reports without a default has to
				// say so explicitly: an unset default_value still renders DEFAULT
				// CURRENT_TIMESTAMP, which would invent a default the live table
				// does not have and keep the plan proposing it forever.
				NoDefaultCurrentTimestamp: in.DefaultValue == nil,
			},
		}
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

// mysqlIndexNeedsPrefix reports whether a column of this field type lands on a
// TEXT/BLOB column, which MySQL refuses to index without a prefix length
// (error 1170). It mirrors tosql's column-type mapping: text (8) and its
// richtext/code/markdown (15-17) siblings are TEXT, and a file/image/audio/video
// field (18-21) is a BLOB only when it stores the bytes in the column — object
// store storage puts a url in a VARCHAR, which indexes fine on its own.
func mysqlIndexNeedsPrefix(f *nemgen.Field) bool {
	switch f.GetType() {
	case nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		return true
	case nemgen.FieldType_FIELD_TYPE_FILE:
		return isBinaryStorage(f.GetTypeConfig().GetFile())
	case nemgen.FieldType_FIELD_TYPE_IMAGE:
		return isBinaryStorage(f.GetTypeConfig().GetImage())
	case nemgen.FieldType_FIELD_TYPE_AUDIO:
		return isBinaryStorage(f.GetTypeConfig().GetAudio())
	case nemgen.FieldType_FIELD_TYPE_VIDEO:
		return isBinaryStorage(f.GetTypeConfig().GetVideo())
	}
	return false
}

func isBinaryStorage(config *nemgen.FieldTypeFileConfig) bool {
	return config.GetStorageType() == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY
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

	// A FULLTEXT index is only identifiable by INDEX_TYPE: TABLE_CONSTRAINTS has
	// no row for it, so CONSTRAINT_TYPE falls back to "INDEX" and the index
	// would come back as an ordinary one.
	indexType := nemgen.IndexType_INDEX_TYPE_INDEX
	switch {
	case strings.EqualFold(first.IndexType, "FULLTEXT"):
		indexType = nemgen.IndexType_INDEX_TYPE_FULLTEXT
	case first.ConstraintType == "PRIMARY KEY":
		indexType = nemgen.IndexType_INDEX_TYPE_PRIMARY
	case first.ConstraintType == "UNIQUE":
		indexType = nemgen.IndexType_INDEX_TYPE_UNIQUE
	}

	indexFields := make(map[string]*nemgen.Field)
	for _, f := range fields {
		if slices.Contains(columns, f.Identifier) {
			indexFields[f.Identifier] = f
		}
	}

	finalIndexFields := []*nemgen.IndexField{}
	for _, id := range in {
		field, found := indexFields[id.ColumnName]
		if !found {
			continue
		}
		// SUB_PART is the prefix the live index actually uses, so preferring it
		// makes the round trip a fixed point. The type-based default only covers
		// an index reported without one over a column that cannot be indexed
		// bare. A FULLTEXT index always reports SUB_PART NULL — MySQL discards
		// any prefix given for one — so it must not acquire a default either.
		length := int64(0)
		if indexType != nemgen.IndexType_INDEX_TYPE_FULLTEXT {
			switch {
			case id.SubPart.Valid:
				length = id.SubPart.Int64
			case mysqlIndexNeedsPrefix(field):
				length = 255
			}
		}
		order := nemgen.IndexFieldOrder_INDEX_FIELD_ORDER_ASC
		if id.Collation.Valid && id.Collation.String == "D" {
			order = nemgen.IndexFieldOrder_INDEX_FIELD_ORDER_DESC
		}
		finalIndexFields = append(finalIndexFields, &nemgen.IndexField{
			FieldUuid: field.Uuid,
			Priority:  id.Seq,
			Order:     order,
			Length:    length,
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
		if f.Identifier == in.ReferencedColumnName {
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
		Status:        nemgen.RelationshipStatus_RELATIONSHIP_STATUS_ACTIVE,
		UseForeignKey: true,
		Cardinality:   nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_ONE,
		CreatedAt:     timestamppb.Now(),
		UpdatedAt:     timestamppb.Now(),
		OnDelete:      tosql.ReferentialActionFromSQL(in.DeleteRule),
		OnUpdate:      tosql.ReferentialActionFromSQL(in.UpdateRule),
	}

}
