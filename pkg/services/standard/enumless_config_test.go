package standard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEnumlessConfigEnums verifies that a config without enum fields reports
// an empty, non-nil enum map.
func TestEnumlessConfigEnums(t *testing.T) {
	t.Parallel()

	enums := (&EnumlessConfig{}).Enums()

	assert.NotNil(t, enums)
	assert.Empty(t, enums)
}
