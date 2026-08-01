package router

import "sort"

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
// Parameters:
//   - schema: the service schema to check.
//
// Returns:
//   - bool: true if the schema is supported.
func SupportsSchema(schema string) bool {
	_, ok := serviceMap[schema]

	return ok
}
