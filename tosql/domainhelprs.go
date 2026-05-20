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

// isJSONField reports whether a field is stored as a JSON column (JSON, ARRAY,
// or a multi-value ENUM), so that an empty value should be treated as NULL
// rather than the empty string ”, which is not valid JSON.
func isJSONField(f *nemgen.Field) bool {
	if f == nil {
		return false
	}
	switch f.Type {
	case nemgen.FieldType_FIELD_TYPE_JSON,
		nemgen.FieldType_FIELD_TYPE_ARRAY:
		return true
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		return f.TypeConfig != nil && f.TypeConfig.Enum != nil && f.TypeConfig.Enum.AllowMultiple
	}
	return false
}
