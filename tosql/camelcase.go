package tosql

import (
	"strings"

	"github.com/iancoleman/strcase"
)

// ToCamelCase converts a snake_case identifier to CamelCase while keeping common
// initialisms (ID, UUID, JSON, URL, HTTP/HTTPS) fully upper-cased, so generated
// query names match go-code-gen's identifier casing (e.g. FetchProjectByUUID,
// not FetchProjectByUuid). Mirrors go-code-gen's strings.ToCamelCase.
func ToCamelCase(str string) string {
	if str == "id" {
		return "ID"
	}
	if str == "uuid" {
		return "UUID"
	}

	key := strcase.ToCamel(str)
	if strings.Contains(key, "_Id") {
		key = strings.ReplaceAll(key, "Id", "ID")
	}
	key = strings.ReplaceAll(key, "uuid", "UUID")
	key = strings.ReplaceAll(key, "Uuid", "UUID")
	key = strings.ReplaceAll(key, "Json", "JSON")
	key = strings.ReplaceAll(key, "Url", "URL")
	key = strings.ReplaceAll(key, "Https", "HTTPS")
	key = strings.ReplaceAll(key, "Http", "HTTP")
	return key
}
