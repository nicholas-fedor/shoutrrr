package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
)

// TestSupportedSchemas verifies that SupportedSchemas lists the router's
// sorted service schemas.
func TestSupportedSchemas(t *testing.T) {
	t.Parallel()

	schemas := SupportedSchemas()

	assert.Equal(t, router.SupportedSchemas(), schemas)
	assert.IsNonDecreasing(t, schemas)
	assert.Contains(t, schemas, "smtp")
}

// TestSupportsSchema verifies that SupportsSchema reports registered schemas
// only.
func TestSupportsSchema(t *testing.T) {
	t.Parallel()

	for _, schema := range SupportedSchemas() {
		assert.True(t, SupportsSchema(schema), schema)
	}

	assert.False(t, SupportsSchema("unknown"))
	assert.False(t, SupportsSchema(""))
}
