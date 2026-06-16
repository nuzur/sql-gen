package tosql

import (
	"fmt"
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"

	"github.com/nuzur/sql-gen/db"
)

// buildEntityWithIndexes returns a standalone entity with n single-field,
// non-datetime indexes (each on its own field) and no primary key.
func buildEntityWithIndexes(n int) *nemgen.Entity {
	fields := make([]*nemgen.Field, 0, n)
	indexes := make([]*nemgen.Index, 0, n)
	for i := 0; i < n; i++ {
		fieldUUID := fmt.Sprintf("field-%d", i)
		fields = append(fields, &nemgen.Field{
			Uuid:       fieldUUID,
			Identifier: fmt.Sprintf("field_%d", i),
			Type:       nemgen.FieldType_FIELD_TYPE_UUID, // maps without needing TypeConfig
		})
		indexes = append(indexes, &nemgen.Index{
			Uuid:   fmt.Sprintf("index-%d", i),
			Type:   nemgen.IndexType_INDEX_TYPE_INDEX,
			Fields: []*nemgen.IndexField{{FieldUuid: fieldUUID}},
		})
	}
	return &nemgen.Entity{
		Identifier: "thing",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields:     fields,
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: indexes,
			},
		},
	}
}

// At or below the threshold the full power set (2^N - 1) is generated.
func TestResolveSelectStatements_PowerSetUnderThreshold(t *testing.T) {
	e := buildEntityWithIndexes(3)
	selects := ResolveSelectStatements(e, db.MYSQLDBType)
	want := (1 << 3) - 1 // 7
	if len(selects) != want {
		t.Fatalf("got %d selects, want %d (power set)", len(selects), want)
	}
}

// Above the threshold it must NOT explode: one select per index, not 2^N.
func TestResolveSelectStatements_CapsAboveThreshold(t *testing.T) {
	const n = maxPowerSetIndexes + 5 // 13
	e := buildEntityWithIndexes(n)
	selects := ResolveSelectStatements(e, db.MYSQLDBType)
	if len(selects) != n {
		t.Fatalf("got %d selects, want %d (one per index); power set would be %d",
			len(selects), n, (1<<n)-1)
	}
}

func TestSingleIndexSubsets(t *testing.T) {
	subsets := singleIndexSubsets([]string{"a", "b", "c"})
	if len(subsets) != 3 {
		t.Fatalf("got %d subsets, want 3", len(subsets))
	}
	for i, s := range subsets {
		if len(s) != 1 {
			t.Errorf("subset %d has len %d, want 1", i, len(s))
		}
	}
}
