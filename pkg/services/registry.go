package services

import (
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

// SupportedSchemas returns the sorted list of service schemas supported by this build.
//
// Returns:
//   - []string: the supported service schemas.
func SupportedSchemas() []string {
	return router.SupportedSchemas()
}

// SupportsSchema reports whether schema is supported by this build.
//
// Parameters:
//   - schema: the service schema to check.
//
// Returns:
//   - bool: true if the schema is supported.
func SupportsSchema(schema string) bool {
	return router.SupportsSchema(schema)
}
