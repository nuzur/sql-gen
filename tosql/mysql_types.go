package tosql

import (
	"fmt"

	nemgen "github.com/nuzur/nem/idl/gen"
)

func FieldTypeToMYSQL(f *nemgen.Field) string {
	switch f.Type {

	case nemgen.FieldType_FIELD_TYPE_UUID: // 1
		return "CHAR(36)"
	case nemgen.FieldType_FIELD_TYPE_INTEGER: // 2
		if f.TypeConfig.Integer != nil && f.TypeConfig.Integer.Size != nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_INVALID {
			switch f.TypeConfig.Integer.Size {
			// An INTEGER must never land on TINYINT, which is what BOOLEAN maps to
			// below. Consumers that pick a Go type per SQL type (sqlc, via
			// go-code-gen's overrides) key on the type NAME, so a shared TINYINT
			// forces one Go type on both: BOOLEAN needs bool, INTEGER needs int64,
			// and whichever loses generates code that does not compile. SMALLINT is
			// the narrowest unambiguously-integer type, and costs one extra byte.
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_ONE_BIT:
				return "SMALLINT"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_EIGHT_BITS:
				return "SMALLINT"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTEEN_BITS:
				return "SMALLINT"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_TWENTY_FOUR_BITS:
				return "MEDIUMINT"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS:
				return "INT"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTY_FOUR_BITS:
				return "BIGINT"
			}
			return "INT"
		}
		return "INT"
	case nemgen.FieldType_FIELD_TYPE_FLOAT: // 3
		return "DOUBLE"
	case nemgen.FieldType_FIELD_TYPE_DECIMAL: // 4
		return decimalType(f.GetTypeConfig().GetDecimal())
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN: // 5
		return "TINYINT(1)"
	case nemgen.FieldType_FIELD_TYPE_CHAR: // 6
		if f.TypeConfig.Char != nil && f.TypeConfig.Char.MaxSize != 0 {
			return fmt.Sprintf("CHAR(%d)", f.TypeConfig.Char.MaxSize)
		}
		return "CHAR(255)" // default
	case nemgen.FieldType_FIELD_TYPE_VARCHAR: // 7
		if f.TypeConfig.Varchar != nil && f.TypeConfig.Varchar.MaxSize != 0 {
			return fmt.Sprintf("VARCHAR(%d)", f.TypeConfig.Varchar.MaxSize)
		}
		return "VARCHAR(255)" // default
	case nemgen.FieldType_FIELD_TYPE_TEXT: // 8
		if f.TypeConfig.Text != nil && f.TypeConfig.Text.MaxSize != 0 {
			if f.TypeConfig.Text.MaxSize <= 255 {
				return "TINYTEXT"
			}
			if f.TypeConfig.Text.MaxSize <= 65535 {
				return "TEXT"
			}
			if f.TypeConfig.Text.MaxSize <= 16777215 {
				return "MEDIUMTEXT"
			}
			if f.TypeConfig.Text.MaxSize <= 4294967295 {
				return "LONGTEXT"
			}
		}
		return "TEXT"
	case nemgen.FieldType_FIELD_TYPE_RICHTEXT, // 15
		nemgen.FieldType_FIELD_TYPE_CODE,     // 16
		nemgen.FieldType_FIELD_TYPE_MARKDOWN: // 17
		return "TEXT"
	case nemgen.FieldType_FIELD_TYPE_ENCRYPTED: // 9
		if f.TypeConfig.Encrypted != nil && f.TypeConfig.Encrypted.MaxSize != 0 {
			return fmt.Sprintf("VARCHAR(%d)", f.TypeConfig.Encrypted.MaxSize)
		}
		return "VARCHAR(255)" // default
	case nemgen.FieldType_FIELD_TYPE_EMAIL: // 10
		return "VARCHAR(512)" // default
	case nemgen.FieldType_FIELD_TYPE_PHONE: // 11
		return "VARCHAR(50)" // default
	case nemgen.FieldType_FIELD_TYPE_URL: // 12
		return "VARCHAR(512)" // default
	case nemgen.FieldType_FIELD_TYPE_LOCATION: // 13
		return "VARCHAR(512)" // default
	case nemgen.FieldType_FIELD_TYPE_COLOR: // 14
		return "VARCHAR(50)" // default
	case nemgen.FieldType_FIELD_TYPE_FILE: // 18
		return handleFileTypeMYSQL(f.TypeConfig.File)
	case nemgen.FieldType_FIELD_TYPE_IMAGE: // 19
		return handleFileTypeMYSQL(f.TypeConfig.Image)
	case nemgen.FieldType_FIELD_TYPE_AUDIO: // 20
		return handleFileTypeMYSQL(f.TypeConfig.Audio)
	case nemgen.FieldType_FIELD_TYPE_VIDEO: // 21
		return handleFileTypeMYSQL(f.TypeConfig.Video)
	case nemgen.FieldType_FIELD_TYPE_ENUM: // 22
		if f.TypeConfig.Enum.AllowMultiple {
			return "JSON"
		}
		return "INT"
	case nemgen.FieldType_FIELD_TYPE_JSON, // 23
		nemgen.FieldType_FIELD_TYPE_ARRAY: // 24
		return "JSON"
	case nemgen.FieldType_FIELD_TYPE_DATE: // 25
		return "DATE"
	case nemgen.FieldType_FIELD_TYPE_DATETIME: // 26
		return datetimeTypeMYSQL(f.GetTypeConfig().GetDatetime())
	case nemgen.FieldType_FIELD_TYPE_TIME: // 27
		return "TIME"
	case nemgen.FieldType_FIELD_TYPE_SLUG: // 28
		return "VARCHAR(512)" // default
	}
	return ""
}

// handleFileTypeMYSQL maps a file/image/audio/video field to its column type.
//
// Only BINARY storage puts bytes in the column. Everything else stores a
// REFERENCE into the object store — a url or key — so the column is text, and
// with allow_multiple it is a JSON array of them. An unset storage_type
// resolves to object store, not binary: that is what the platform normalizes a
// file field to (a storage_config naming an object store, no explicit
// storage_type), and it is what the generated /upload endpoint produces. The
// old default of BLOB made the column disagree with the string the domain layer
// held for exactly those fields. Importers that mean binary say so explicitly
// (see fromsql), so nothing that is really a blob relies on the default.
func handleFileTypeMYSQL(config *nemgen.FieldTypeFileConfig) string {
	if config.GetStorageType() == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY {
		switch {
		case config.GetMaxSize() == 0:
			return "BLOB"
		case config.GetMaxSize() <= 255:
			return "TINYBLOB"
		case config.GetMaxSize() <= 65535:
			return "BLOB"
		case config.GetMaxSize() <= 16777215:
			return "MEDIUMBLOB"
		default:
			return "LONGBLOB"
		}
	}
	if config.GetAllowMultiple() {
		// a list of object-store references, stored the same way every other
		// list in the schema is
		return "JSON"
	}
	return "VARCHAR(512)" // default url size
}
