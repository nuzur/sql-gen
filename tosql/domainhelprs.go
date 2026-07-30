package tosql

import nemgen "github.com/nuzur/nem/idl/gen"

func EntityPrimaryKeys(entity *nemgen.Entity) []*nemgen.Field {
	res := []*nemgen.Field{}
	for _, f := range entity.Fields {
		if f.Key {
			res = append(res, f)
		}
	}
	return res
}

// blankMeansNull reports whether an empty incoming value for this field has to be
// written as the SQL NULL keyword rather than bound as the empty string.
//
// ” is a legitimate value only for the character columns (see FieldTypeToPG /
// FieldTypeToMySQL). For every other column type it is at best meaningless and at
// worst a hard error: postgres rejects ” for TIMESTAMP/DATE/TIME/UUID/numeric
// types outright, JSON columns reject it as invalid JSON, and mysql in loose mode
// silently coerces it to a zero date. A caller that leaves such a field blank
// means "no value", which is NULL.
//
// Callers must emit the NULL keyword inline (no placeholder) and skip the bound
// parameter, so the driver never receives the string "NULL" as a value.
func blankMeansNull(f *nemgen.Field) bool {
	if f == nil {
		return false
	}
	switch f.Type {
	case nemgen.FieldType_FIELD_TYPE_CHAR,
		nemgen.FieldType_FIELD_TYPE_VARCHAR,
		nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
		nemgen.FieldType_FIELD_TYPE_EMAIL,
		nemgen.FieldType_FIELD_TYPE_PHONE,
		nemgen.FieldType_FIELD_TYPE_URL,
		nemgen.FieldType_FIELD_TYPE_LOCATION,
		nemgen.FieldType_FIELD_TYPE_COLOR,
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN,
		nemgen.FieldType_FIELD_TYPE_SLUG:
		return false
	case nemgen.FieldType_FIELD_TYPE_FILE:
		return fileConfigIsNotVarchar(f.GetTypeConfig().GetFile())
	case nemgen.FieldType_FIELD_TYPE_IMAGE:
		return fileConfigIsNotVarchar(f.GetTypeConfig().GetImage())
	case nemgen.FieldType_FIELD_TYPE_AUDIO:
		return fileConfigIsNotVarchar(f.GetTypeConfig().GetAudio())
	case nemgen.FieldType_FIELD_TYPE_VIDEO:
		return fileConfigIsNotVarchar(f.GetTypeConfig().GetVideo())
	}
	// uuid, integer, float, decimal, boolean, enum, json, array, date, datetime, time
	return true
}

// fileConfigIsNotVarchar mirrors handleFileTypePG/handleFileTypeMySQL: a single
// object-store reference is a varchar url, while multiple references are a JSON
// list and binary storage is a blob — neither of which accepts ”.
func fileConfigIsNotVarchar(config *nemgen.FieldTypeFileConfig) bool {
	return config.GetAllowMultiple() ||
		config.GetStorageType() == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY
}
