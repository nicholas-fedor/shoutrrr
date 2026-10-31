package signalgrid

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// parseTestURL parses rawURL and fails the test if it is invalid.
//
// Parameters:
//   - t: the test.
//   - rawURL: the URL to parse.
//
// Returns:
//   - *url.URL: the parsed URL.
func parseTestURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	return parsed
}

// newDefaultConfig returns a config with the default props applied.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - *Config: the config.
func newDefaultConfig(t *testing.T) *Config {
	t.Helper()

	config := &Config{}
	resolver := format.NewPropKeyResolver(config)
	require.NoError(t, resolver.SetDefaultProps(config))

	return config
}

// TestConfigSetURL verifies that SetURL reads the client key from the user
// info, the channel from the host, and the title, type, and critical props
// from the query.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "client key and channel",
			rawURL: "signalgrid://abc123@channel",
			want:   Config{ClientKey: "abc123", Channel: "channel", Type: TypeINFO},
		},
		{
			name:   "title, type, and critical",
			rawURL: "signalgrid://key@channel?title=Server%20Down&type=CRIT&critical=true",
			want:   Config{ClientKey: "key", Channel: "channel", Title: "Server Down", Type: TypeCRIT, Critical: true},
		},
		{
			name:   "lowercase type",
			rawURL: "signalgrid://key@channel?type=warn",
			want:   Config{ClientKey: "key", Channel: "channel", Type: TypeWARN},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := newDefaultConfig(t)
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, *config)
		})
	}
}

// TestConfigSetURLErrors verifies that SetURL rejects a URL without a client
// key or channel, a channel token with a port or brackets, and a URL with an
// unknown or invalid query prop.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "missing client key", rawURL: "signalgrid://channel", wantErr: ErrClientKeyMissing},
		{name: "empty client key", rawURL: "signalgrid://@channel", wantErr: ErrClientKeyMissing},
		{name: "missing channel", rawURL: "signalgrid://key@", wantErr: ErrChannelMissing},
		{name: "channel with a port", rawURL: "signalgrid://key@SECRETchannel:8443", wantErr: util.ErrInvalidHost},
		{name: "channel in brackets", rawURL: "signalgrid://key@[::]", wantErr: util.ErrInvalidHost},
		{name: "unknown prop", rawURL: "signalgrid://key@channel?foo=bar", wantErr: format.ErrInvalidConfigKey},
		{name: "invalid type", rawURL: "signalgrid://key@channel?type=nope", wantErr: format.ErrInvalidEnumValue},
		{name: "invalid critical", rawURL: "signalgrid://key@channel?critical=maybe", wantErr: format.ErrInvalidBoolValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, newDefaultConfig(t).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL encodes the client key as the user
// and the channel as the host, leaves default props out of the query, and
// round-trips through SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		config := newDefaultConfig(t)
		config.ClientKey = "key"
		config.Channel = "channel"

		assert.Equal(t, "signalgrid://key@channel", config.GetURL().String())
	})

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()

		config := newDefaultConfig(t)
		config.ClientKey = "abc123"
		config.Channel = "channel"
		config.Title = "Alert"
		config.Type = TypeCRIT
		config.Critical = true

		serviceURL := config.GetURL()
		assert.Equal(t, "signalgrid://abc123@channel?critical=Yes&title=Alert&type=CRIT", serviceURL.String())

		roundTrip := newDefaultConfig(t)
		require.NoError(t, roundTrip.SetURL(serviceURL))
		assert.Equal(t, config, roundTrip)
	})
}

// TestConfigQueryResolver verifies the config's query props: the default type
// is INFO, the keys are critical, title, and type, and type is an enum.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	t.Run("default values", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, Config{Type: TypeINFO}, *newDefaultConfig(t))
	})

	t.Run("fields and enums", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Equal(t, []string{"critical", "title", "type"}, format.GetConfigQueryResolver(config).QueryFields())
		assert.Equal(t, []string{"Type"}, keys(config.Enums()))
	})
}

// keys returns the keys of m.
//
// Parameters:
//   - m: the map.
//
// Returns:
//   - []string: the keys, in no particular order.
func keys[V any](m map[string]V) []string {
	result := make([]string, 0, len(m))
	for key := range m {
		result = append(result, key)
	}

	return result
}
