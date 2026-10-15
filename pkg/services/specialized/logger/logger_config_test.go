package logger

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigGetURL verifies that GetURL serializes the config as logger://.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	serviceURL := (&Config{}).GetURL()

	assert.Equal(t, Scheme, serviceURL.Scheme)
	assert.Equal(t, "logger://", serviceURL.String())
}

// TestConfigSetURL verifies that SetURL accepts any logger URL, since the
// config has no fields.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	for _, rawURL := range []string{"logger://", "logger://host/path?key=value"} {
		serviceURL, err := url.Parse(rawURL)
		require.NoError(t, err)

		config := &Config{}
		require.NoError(t, config.SetURL(serviceURL))
		assert.Equal(t, &Config{}, config)
	}
}

// TestConfigEnums verifies that the config has no enum fields.
func TestConfigEnums(t *testing.T) {
	t.Parallel()

	assert.Empty(t, (&Config{}).Enums())
}
