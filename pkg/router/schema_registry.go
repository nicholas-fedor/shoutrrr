package router

import (
	"sort"
	"strings"
)

// SupportedSchemas returns the sorted list of service schemas supported by this build.
//
// Returns:
//   - []string: the supported service schemas.
func SupportedSchemas() []string {
	schemas := make([]string, 0, len(serviceMap))
	for scheme := range serviceMap {
		schemas = append(schemas, scheme)
	}

	sort.Strings(schemas)

	return schemas
}

// SupportsSchema reports whether schema is supported by this build.
//
// Schemes are matched in any case, as URL schemes are case-insensitive and
// [ServiceRouter.Locate] accepts them that way.
//
// Parameters:
//   - schema: the service schema to check.
//
// Returns:
//   - bool: true if the schema is supported.
func SupportsSchema(schema string) bool {
	_, ok := serviceMap[strings.ToLower(schema)]

	return ok
}
