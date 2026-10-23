package router

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSupportsSchema verifies that SupportsSchema reports every registered
// schema in any case, as URL schemes are case-insensitive and Locate accepts
// them that way, and rejects unknown and empty schemas.
func TestSupportsSchema(t *testing.T) {
	t.Parallel()

	for _, schema := range SupportedSchemas() {
		assert.True(t, SupportsSchema(schema), schema)
		assert.True(t, SupportsSchema(strings.ToUpper(schema)), strings.ToUpper(schema))
	}

	assert.True(t, SupportsSchema("Discord"))
	assert.False(t, SupportsSchema("unknown"))
	assert.False(t, SupportsSchema(""))
}
