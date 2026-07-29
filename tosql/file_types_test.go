package tosql

import (
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/stretchr/testify/assert"
)

// A file/image/audio/video field has two storage shapes, and the column type
// has to follow the one the schema declares:
//
//   - BINARY      -> the bytes live in the column       -> BLOB / BYTEA
//   - OBJECT_STORE-> the column holds a url or key      -> VARCHAR
//     ...and with allow_multiple, a LIST of them        -> JSON
//
// An unset storage_type resolves to OBJECT_STORE, because that is the shape the
// platform normalizes a file field to (a storage_config naming an object store,
// with no explicit storage_type) and the shape the generated /upload endpoint
// produces. It used to fall back to BINARY, so those fields got a BLOB column
// while every other layer treated them as a string — the column type and the Go
// type disagreed for the single most common way a file field is defined.
//
// These four types are also the ones whose config lives under their OWN key
// (type_config.image for an image, not type_config.file), so each is checked
// through the field-level entry point rather than the shared helper.

func fileField(t nemgen.FieldType, cfg *nemgen.FieldTypeFileConfig) *nemgen.Field {
	tc := &nemgen.FieldTypeConfig{}
	switch t {
	case nemgen.FieldType_FIELD_TYPE_FILE:
		tc.File = cfg
	case nemgen.FieldType_FIELD_TYPE_IMAGE:
		tc.Image = cfg
	case nemgen.FieldType_FIELD_TYPE_AUDIO:
		tc.Audio = cfg
	case nemgen.FieldType_FIELD_TYPE_VIDEO:
		tc.Video = cfg
	}
	return &nemgen.Field{Identifier: "attachment", Type: t, TypeConfig: tc}
}

var fileTypes = []nemgen.FieldType{
	nemgen.FieldType_FIELD_TYPE_FILE,
	nemgen.FieldType_FIELD_TYPE_IMAGE,
	nemgen.FieldType_FIELD_TYPE_AUDIO,
	nemgen.FieldType_FIELD_TYPE_VIDEO,
}

const (
	binaryStorage = nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY
	objectStorage = nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_OBJECT_STORE
)

func TestFileStorageShapeDrivesColumnType(t *testing.T) {
	for _, tc := range []struct {
		name            string
		cfg             *nemgen.FieldTypeFileConfig
		mysql, postgres string
	}{
		{
			name:     "binary stores bytes in the column",
			cfg:      &nemgen.FieldTypeFileConfig{StorageType: binaryStorage},
			mysql:    "BLOB",
			postgres: "BYTEA",
		},
		{
			name:     "object store holds a single url",
			cfg:      &nemgen.FieldTypeFileConfig{StorageType: objectStorage},
			mysql:    "VARCHAR(512)",
			postgres: "VARCHAR(512)",
		},
		{
			// The reported shape: the platform sets storage_config but leaves
			// storage_type unset.
			name: "unset storage type is an object store, not a blob",
			cfg: &nemgen.FieldTypeFileConfig{
				StorageConfig: &nemgen.FileStorageConfig{
					ObjectStore: &nemgen.FileObjectStorageConfig{
						ObjectStoreUuid: "00000000-0000-0000-0000-000000000000",
					},
				},
			},
			mysql:    "VARCHAR(512)",
			postgres: "VARCHAR(512)",
		},
		{
			name:     "no config at all is an object store",
			cfg:      nil,
			mysql:    "VARCHAR(512)",
			postgres: "VARCHAR(512)",
		},
		{
			// A list of urls does not fit a single VARCHAR; it is a JSON array,
			// like every other list in the schema.
			name:     "allow_multiple holds a list of urls",
			cfg:      &nemgen.FieldTypeFileConfig{StorageType: objectStorage, AllowMultiple: true},
			mysql:    "JSON",
			postgres: "JSON",
		},
		{
			name:     "allow_multiple with an unset storage type is still a list",
			cfg:      &nemgen.FieldTypeFileConfig{AllowMultiple: true},
			mysql:    "JSON",
			postgres: "JSON",
		},
		{
			// allow_multiple is meaningless for inline bytes: one column, one blob.
			name:     "binary ignores allow_multiple",
			cfg:      &nemgen.FieldTypeFileConfig{StorageType: binaryStorage, AllowMultiple: true},
			mysql:    "BLOB",
			postgres: "BYTEA",
		},
	} {
		for _, ft := range fileTypes {
			t.Run(tc.name+"/"+ft.String(), func(t *testing.T) {
				f := fileField(ft, tc.cfg)
				assert.Equal(t, tc.mysql, FieldTypeToMYSQL(f))
				assert.Equal(t, tc.postgres, FieldTypeToPG(f))
			})
		}
	}
}

// Binary sizes still tier into MySQL's four blob widths.
func TestBinaryFileTiersBySize(t *testing.T) {
	for _, tc := range []struct {
		size int64
		want string
	}{
		{0, "BLOB"},
		{255, "TINYBLOB"},
		{65535, "BLOB"},
		{16777215, "MEDIUMBLOB"},
		{4294967295, "LONGBLOB"},
		// Anything past the largest tier still has to be a blob, not the
		// fallthrough the old chain of ifs produced.
		{1 << 33, "LONGBLOB"},
	} {
		f := fileField(nemgen.FieldType_FIELD_TYPE_FILE, &nemgen.FieldTypeFileConfig{
			StorageType: binaryStorage,
			MaxSize:     tc.size,
		})
		assert.Equalf(t, tc.want, FieldTypeToMYSQL(f), "max_size %d", tc.size)
		// Postgres has a single unbounded BYTEA.
		assert.Equal(t, "BYTEA", FieldTypeToPG(f))
	}
}
