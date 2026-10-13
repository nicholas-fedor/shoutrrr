package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// TestConfigQueryAPI verifies that the config rejects unknown query keys when
// reading and parsing, and exposes its parse mode enum and six query fields.
func TestConfigQueryAPI(t *testing.T) {
	t.Parallel()

	config := newTestService(t, testServiceURL).GetConfig()
	resolver := format.GetConfigQueryResolver(config)

	value, err := resolver.Get("invalid query var")
	require.Error(t, err)
	assert.Empty(t, value)

	require.Error(t, config.SetURL(parseURL(t, testServiceURL+"&foo=bar")))

	assert.Len(t, config.Enums(), 1)
	assert.Len(t, resolver.QueryFields(), 6)
}
