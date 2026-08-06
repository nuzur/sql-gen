package tosql

import (
	"fmt"

	"github.com/gofrs/uuid"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// uniqueFieldIndexNamespace is the UUID v5 namespace the synthesized indexes are
// minted under. It is a fixed, arbitrary constant on purpose: the uuid of a
// synthesized index must be a pure function of (entity uuid, field uuid) so that
// every regeneration of the same schema — in this process, in nuzur's
// sql-diff-manager, or months later on another machine — produces the same uuid.
// A random uuid would make each run look like a different index to anything that
// tracks them by identity. Changing this constant re-mints every synthesized
// index, so it must never change.
const uniqueFieldIndexNamespace = "3f2b9f2e-9c9a-4a8b-9c1d-2f4d6b8a0c7e"

// maxSynthesizedIndexIdentifierLength bounds the generated index identifier.
// Postgres truncates identifiers at 63 bytes and MySQL rejects them past 64, and
// a silently truncated name is worse than a short one: two long names that
// collide after truncation become one index in the database and a permanent diff
// against the model. 60 leaves room for the suffixes deduplicateIndexNames adds.
const maxSynthesizedIndexIdentifierLength = 60

// uniqueFieldIndexIdentifierPrefix marks an index as coming from a field's
// `unique: true` flag rather than from something the modeler drew by hand.
const uniqueFieldIndexIdentifierPrefix = "uq"

// EnsureUniqueFieldIndexes makes field-level `unique: true` real schema.
//
// nem lets a field declare `unique: true`, but a flag alone produces nothing: the
// generator emits uniqueness only from entity.type_config.standalone.indexes, and
// so does every select it derives. The flag was therefore metadata the database
// never saw — a modeler could mark a column unique and get a table that happily
// accepted duplicates. This function closes that gap by treating the flag as
// sugar for the index the modeler meant: for every unique-flagged field that
// nothing already enforces, it appends a synthesized single-field
// INDEX_TYPE_UNIQUE index to the entity.
//
// Sugar, not a second mechanism. Uniqueness is expressed one way only — an index
// — so the column definition never carries its own UNIQUE. Two overlapping
// declarations would double-enforce the same rule and, worse, present the diff
// engine with a constraint it cannot attribute to any modeled index, which it
// would then propose dropping (or recreating) on every plan.
//
// A synthesized index is suppressed when the entity already has an ACTIVE UNIQUE
// or PRIMARY index over exactly that one field, because that index already
// enforces the rule. Nothing else suppresses it:
//
//   - A plain single-field INDEX enforces no uniqueness at all, so the UNIQUE is
//     still needed. It does mean both can resolve to the same select name; the
//     select resolver already dedupes those by name.
//   - A composite UNIQUE constrains the tuple, not the column — (a, b) unique
//     permits duplicate a's — so it does not cover a single-column rule.
//   - An inactive index enforces nothing: it is not emitted as DDL.
//
// Key fields are skipped: the primary key already enforces uniqueness, and a
// synthesized index over the key would also make the select resolver emit a
// second "<Entity>By<Key>" fetch alongside the primary one.
//
// JSON-mapped fields are skipped because MySQL cannot index a JSON column at all
// (with or without a prefix length), so a synthesized index there would produce
// DDL that fails inside CREATE TABLE and takes the whole table with it. The check
// runs against the MySQL mapping regardless of the dbType being generated, so a
// schema synthesizes the same set of indexes on every engine — a Postgres run
// that invented an index MySQL cannot have would make the two dialects disagree
// about what the model contains.
//
// Synthesized indexes are appended at the END of the index list. Ordering is
// load-bearing: the select resolver walks indexes in order, so appending leaves
// every pre-existing select byte-identical and adds the new ones after. They also
// count toward the select power-set cap (maxPowerSetIndexes) exactly like
// explicit indexes — an entity pushed over the threshold by synthesis degrades to
// one select per index, the same bounded fallback as any other entity.
//
// Mutates pv in place, matching SortStandaloneEntities, which GenerateSQL already
// applies to the caller's project version. It is idempotent: a synthesized index
// satisfies the suppression rule itself, so running it again adds nothing — which
// matters because a normalized project version can reach GenerateSQL after this
// has already run over it once.
func EnsureUniqueFieldIndexes(pv *nemgen.ProjectVersion) {
	if pv == nil {
		return
	}
	for _, e := range pv.Entities {
		if e == nil || e.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			continue
		}
		// Identifiers already in use on this entity, so a synthesized name can
		// never shadow a hand-written index. Synthesized names join the set as
		// they are minted: two unique fields on one entity must not collide
		// with each other either.
		taken := takenIndexIdentifiers(e)
		for _, f := range e.Fields {
			if !needsSynthesizedUniqueIndex(e, f) {
				continue
			}
			identifier := synthesizedIndexIdentifier(e.Identifier, f, taken)
			taken[identifier] = true
			appendIndex(e, &nemgen.Index{
				Uuid:       synthesizedIndexUUID(e.Uuid, f.Uuid),
				Identifier: identifier,
				Type:       nemgen.IndexType_INDEX_TYPE_UNIQUE,
				Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
				Fields: []*nemgen.IndexField{
					{
						FieldUuid: f.Uuid,
						Priority:  0,
						Order:     nemgen.IndexFieldOrder_INDEX_FIELD_ORDER_ASC,
					},
				},
			})
		}
	}
}

// needsSynthesizedUniqueIndex reports whether f's `unique: true` still has to be
// turned into an index — see EnsureUniqueFieldIndexes for why each case is
// excluded.
func needsSynthesizedUniqueIndex(e *nemgen.Entity, f *nemgen.Field) bool {
	if f == nil || !f.Unique || f.Status != nemgen.FieldStatus_FIELD_STATUS_ACTIVE {
		return false
	}
	// The primary key already enforces uniqueness for key fields.
	if f.Key {
		return false
	}
	// A field the mapper refuses to emit as a column (no identifier, no type)
	// cannot be indexed either: mapField drops it and the index would name a
	// column that is not in the table.
	if f.Identifier == "" || f.Type == nemgen.FieldType_FIELD_TYPE_INVALID {
		return false
	}
	if mapsToJSONColumn(f) {
		return false
	}
	return !hasSingleFieldUniquenessIndex(e, f.Uuid)
}

// mapsToJSONColumn reports whether the field becomes a JSON column on MySQL,
// which is the one column type MySQL cannot index.
//
// FieldTypeToMYSQL is the authority on that mapping, but it dereferences
// type_config sub-messages for several field types and GenerateSQL is also called
// on raw, non-normalized project versions where those can be missing. The two
// shapes that would panic are answered first, and only then is the real mapper
// asked.
func mapsToJSONColumn(f *nemgen.Field) bool {
	if f.GetTypeConfig() == nil {
		// JSON and ARRAY are the only types that map to JSON without consulting
		// their config.
		return f.Type == nemgen.FieldType_FIELD_TYPE_JSON || f.Type == nemgen.FieldType_FIELD_TYPE_ARRAY
	}
	if f.Type == nemgen.FieldType_FIELD_TYPE_ENUM && f.GetTypeConfig().GetEnum() == nil {
		return false
	}
	return FieldTypeToMYSQL(f) == "JSON"
}

// hasSingleFieldUniquenessIndex reports whether the entity already enforces
// uniqueness of that one field on its own.
func hasSingleFieldUniquenessIndex(e *nemgen.Entity, fieldUUID string) bool {
	for _, idx := range e.GetTypeConfig().GetStandalone().GetIndexes() {
		if idx == nil || idx.Status != nemgen.IndexStatus_INDEX_STATUS_ACTIVE {
			continue
		}
		if idx.Type != nemgen.IndexType_INDEX_TYPE_UNIQUE && idx.Type != nemgen.IndexType_INDEX_TYPE_PRIMARY {
			continue
		}
		// Exactly this one field: a composite index constrains the tuple, not
		// the column.
		if len(idx.Fields) != 1 {
			continue
		}
		if idx.Fields[0].GetFieldUuid() == fieldUUID {
			return true
		}
	}
	return false
}

// takenIndexIdentifiers is every index identifier already used on the entity,
// including inactive ones — an inactive index enforces nothing but still owns its
// name, and reusing it would make the two indistinguishable in a later diff.
func takenIndexIdentifiers(e *nemgen.Entity) map[string]bool {
	taken := map[string]bool{}
	for _, idx := range e.GetTypeConfig().GetStandalone().GetIndexes() {
		if idx != nil {
			taken[idx.Identifier] = true
		}
	}
	return taken
}

// synthesizedIndexIdentifier names the index uq_<entity>_<field>, which is what a
// modeler would have typed. When that name is too long or already taken it gets
// the field's uuid prefix appended — a stable discriminator, unlike a counter,
// which would reassign names as unrelated fields come and go and make the diff
// engine see renames that never happened.
func synthesizedIndexIdentifier(entityIdentifier string, f *nemgen.Field, taken map[string]bool) string {
	base := fmt.Sprintf("%s_%s_%s", uniqueFieldIndexIdentifierPrefix, entityIdentifier, f.Identifier)
	if len(base) <= maxSynthesizedIndexIdentifierLength && !taken[base] {
		return base
	}
	return withUUIDSuffix(base, f.Uuid)
}

// withUUIDSuffix appends "_" + the first 8 characters of uuid, trimming the base
// as needed to stay within maxSynthesizedIndexIdentifierLength.
func withUUIDSuffix(base string, uuid string) string {
	discriminator := uuid
	if len(discriminator) > 8 {
		discriminator = discriminator[:8]
	}
	suffix := "_" + discriminator

	// Identifiers are ASCII snake_case in practice, but trim on rune boundaries
	// so a non-ASCII one can never be cut into an invalid string.
	runes := []rune(base)
	if len(runes)+len(suffix) > maxSynthesizedIndexIdentifierLength {
		keep := maxSynthesizedIndexIdentifierLength - len(suffix)
		if keep < 0 {
			keep = 0
		}
		if keep < len(runes) {
			runes = runes[:keep]
		}
	}
	return string(runes) + suffix
}

// synthesizedIndexUUID derives the index uuid from the entity and field it was
// synthesized for — see uniqueFieldIndexNamespace.
func synthesizedIndexUUID(entityUUID string, fieldUUID string) string {
	return uuid.NewV5(uuid.FromStringOrNil(uniqueFieldIndexNamespace), entityUUID+":"+fieldUUID).String()
}

// appendIndex adds the index to the entity's standalone config, creating the
// intermediate messages only now that there is something to put in them. An
// entity that synthesizes nothing must keep a nil TypeConfig: the mapper and the
// select resolver both gate on it being non-nil, so materializing an empty one
// would change the output for entities this feature does not touch.
func appendIndex(e *nemgen.Entity, idx *nemgen.Index) {
	if e.TypeConfig == nil {
		e.TypeConfig = &nemgen.EntityTypeConfig{}
	}
	if e.TypeConfig.Standalone == nil {
		e.TypeConfig.Standalone = &nemgen.EntityTypeStandaloneConfig{}
	}
	e.TypeConfig.Standalone.Indexes = append(e.TypeConfig.Standalone.Indexes, idx)
}
