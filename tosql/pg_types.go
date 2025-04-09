package tosql

import (
	"fmt"

	nemgen "github.com/nuzur/nem/idl/gen"
)

func FieldTypeToPG(f *nemgen.Field) string {
	switch f.Type {

	case nemgen.FieldType_FIELD_TYPE_UUID: // 1
		return "UUID"
	case nemgen.FieldType_FIELD_TYPE_INTEGER: // 2
		if f.TypeConfig.Integer != nil && f.TypeConfig.Integer.Size != nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_INVALID {
			switch f.TypeConfig.Integer.Size {
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_ONE_BIT:
				return "BOOLEAN"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_EIGHT_BITS:
				return "SMALLINT"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTEEN_BITS:
				return "SMALLINT"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_TWENTY_FOUR_BITS:
				return "INTEGER"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS:
				return "INTEGER"
			case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTY_FOUR_BITS:
				return "BIGINT"
			}
			return "INTEGER"
		}
		return "INTEGER"
	case nemgen.FieldType_FIELD_TYPE_FLOAT: // 3
		return "DOUBLE PRECISION"
	case nemgen.FieldType_FIELD_TYPE_DECIMAL: // 4
		return "DECIMAL"
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN: // 5
		return "BOOLEAN"
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
		return "VARCHAR(2048)" // default
	case nemgen.FieldType_FIELD_TYPE_LOCATION: // 13
		return "VARCHAR(2048)" // default
	case nemgen.FieldType_FIELD_TYPE_COLOR: // 14
		return "VARCHAR(50)" // default
	case nemgen.FieldType_FIELD_TYPE_FILE: // 18
		return handleFileTypePG(f.TypeConfig.File)
	case nemgen.FieldType_FIELD_TYPE_IMAGE: // 19
		return handleFileTypePG(f.TypeConfig.Image)
	case nemgen.FieldType_FIELD_TYPE_AUDIO: // 20
		return handleFileTypePG(f.TypeConfig.Audio)
	case nemgen.FieldType_FIELD_TYPE_VIDEO: // 21
		return handleFileTypePG(f.TypeConfig.Video)
	case nemgen.FieldType_FIELD_TYPE_ENUM: // 22
		if f.TypeConfig.Enum.AllowMultiple {
			return "JSON"
		}
		return "INTEGER"
	case nemgen.FieldType_FIELD_TYPE_JSON, // 23
		nemgen.FieldType_FIELD_TYPE_ARRAY: // 24
		return "JSON"
	case nemgen.FieldType_FIELD_TYPE_DATE: // 25
		return "DATE"
	case nemgen.FieldType_FIELD_TYPE_DATETIME: // 26
		return "TIMESTAMP"
	case nemgen.FieldType_FIELD_TYPE_TIME: // 27
		return "TIME"
	case nemgen.FieldType_FIELD_TYPE_SLUG: // 28
		return "VARCHAR(512)" // default
	}
	return ""
}

func handleFileTypePG(config *nemgen.FieldTypeFileConfig) string {
	if config == nil {
		return "BYTEA"
	}
	if config.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY {
		return "BYTEA"
	} else if config.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_OBJECT_STORE {
		return "VARCHAR(512)" // default url size
	}
	return "BYTEA"
}
