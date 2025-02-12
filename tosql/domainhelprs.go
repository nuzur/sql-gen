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
