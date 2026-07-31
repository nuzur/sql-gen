package tosql

import (
	"strings"

	nemgen "github.com/nuzur/nem/idl/gen"
)

// referentialActionSQL renders a relationship's referential action as the SQL
// keyword that follows ON DELETE / ON UPDATE, or "" when no clause should be
// emitted.
//
// INVALID (unset) and NO_ACTION both render nothing. NO ACTION is the SQL
// default, so a clause spelling it out changes no behavior — but it does change
// the DDL text, and information_schema reports a constraint created without a
// clause as NO ACTION on both engines. Emitting it would therefore make the
// generated DDL permanently differ from what introspection reconstructs, which
// is a foreign key the mysql diff proposes dropping and recreating on every
// plan. Rendering nothing for both makes the round trip a fixed point.
func ReferentialActionSQL(action nemgen.RelationshipReferentialAction) string {
	switch action {
	case nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_RESTRICT:
		return "RESTRICT"
	case nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_CASCADE:
		return "CASCADE"
	case nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_SET_NULL:
		return "SET NULL"
	case nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_SET_DEFAULT:
		return "SET DEFAULT"
	}
	return ""
}

// referentialActionFromSQL is the inverse, for introspection. Both engines
// report a constraint created without an explicit clause as "NO ACTION", which
// maps back to unset so the re-rendered DDL carries no clause either.
func ReferentialActionFromSQL(rule string) nemgen.RelationshipReferentialAction {
	switch normalizeRule(rule) {
	case "RESTRICT":
		return nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_RESTRICT
	case "CASCADE":
		return nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_CASCADE
	case "SET NULL":
		return nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_SET_NULL
	case "SET DEFAULT":
		return nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_SET_DEFAULT
	}
	return nemgen.RelationshipReferentialAction_RELATIONSHIP_REFERENTIAL_ACTION_INVALID
}

// normalizeRule folds the spellings the two engines use for a referential rule
// into one: postgres reports "NO ACTION"/"SET NULL" and mysql the same, but a
// caller may hand over the proto-ish "SET_NULL" form, and case is not
// guaranteed.
func normalizeRule(rule string) string {
	rule = strings.ToUpper(strings.TrimSpace(rule))
	rule = strings.ReplaceAll(rule, "_", " ")
	return strings.Join(strings.Fields(rule), " ")
}
