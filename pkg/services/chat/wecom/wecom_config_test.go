package wecom

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// Webhook key and service URL shared by the tests.
const (
	// testKey is the webhook key in fullURL.
	testKey = "693axxx6-7aoc-4bc4-97a0-0ec2sifa5aaa"
	// fullURL is a valid service URL with only the webhook key set.
	fullURL = "wecom://" + testKey
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

// TestConfigURLRoundTrip verifies that a config parsed from a URL produces the
// same URL again.
func TestConfigURLRoundTrip(t *testing.T) {
	t.Parallel()

	config := &Config{}
	resolver := format.NewPropKeyResolver(config)
	require.NoError(t, config.setURL(&resolver, parseTestURL(t, fullURL)))

	assert.Equal(t, fullURL, config.GetURL().String())
}

// TestConfigURLRoundTripKeepsFields verifies that a config restored from its own
// URL keeps the key, including one set through the key query parameter, and
// both mention lists.
func TestConfigURLRoundTripKeepsFields(t *testing.T) {
	t.Parallel()

	// tests lists URLs with the config SetURL and a round trip through GetURL
	// should both produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "mention lists",
			rawURL: fullURL + "?mentioned_list=@all&mentioned_mobile_list=13800001111",
			want:   Config{Key: testKey, MentionedList: "@all", MentionedMobileList: "13800001111"},
		},
		{
			name:   "key query parameter",
			rawURL: fullURL + "?key=other-key_2",
			want:   Config{Key: "other-key_2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))
			assert.Equal(t, tt.want, *config)

			restored := &Config{}
			require.NoError(t, restored.SetURL(config.GetURL()))
			assert.Equal(t, tt.want, *restored)
		})
	}
}

// TestConfigSetURL verifies the fields SetURL sets from the key and the query
// parameters.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "key only",
			rawURL: fullURL,
			want:   Config{Key: testKey},
		},
		{
			name:   "mention query parameters",
			rawURL: fullURL + "?mentioned_list=@all&mentioned_mobile_list=13800001111",
			want:   Config{Key: testKey, MentionedList: "@all", MentionedMobileList: "13800001111"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, *config)
		})
	}
}

// TestConfigSetURLErrors verifies that SetURL rejects an empty key, a key with
// characters other than letters, digits, hyphens, and underscores, whether in
// the host or the key query parameter, and an unknown query parameter, without
// echoing the rejected key.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid URLs with the sentinel error each should produce.
	tests := []struct {
		name    string
		rawURL  string
		wantErr error
	}{
		{name: "empty key", rawURL: "wecom://", wantErr: ErrEmptyKey},
		{name: "invalid key characters", rawURL: "wecom://SECRET.key", wantErr: ErrInvalidKey},
		{name: "tilde in the key", rawURL: "wecom://SECRET~key", wantErr: ErrInvalidKey},
		{name: "invalid key in the query", rawURL: fullURL + "?key=SECRET.key", wantErr: ErrInvalidKey},
		{name: "space in the query key", rawURL: fullURL + "?key=SECRET%20key", wantErr: ErrInvalidKey},
		{name: "empty key in the query", rawURL: fullURL + "?key=", wantErr: ErrEmptyKey},
		{name: "unknown query parameter", rawURL: fullURL + "?foo=bar", wantErr: format.ErrInvalidConfigKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&Config{}).SetURL(parseTestURL(t, tt.rawURL))

			require.ErrorIs(t, err, tt.wantErr)
			assert.NotContains(t, err.Error(), "SECRET")
		})
	}
}

// TestConfigGetInvalidQueryValue verifies that reading an unknown query key
// returns an empty value and ErrInvalidConfigKey.
func TestConfigGetInvalidQueryValue(t *testing.T) {
	t.Parallel()

	value, err := format.GetConfigQueryResolver(&Config{}).Get("invalid query var")

	require.ErrorIs(t, err, format.ErrInvalidConfigKey)
	assert.Empty(t, value)
}

// TestConfigEnumsAndFields verifies that the config has no enums and exposes
// its three fields as query fields.
func TestConfigEnumsAndFields(t *testing.T) {
	t.Parallel()

	config := &Config{}

	assert.Empty(t, config.Enums())
	assert.Len(t, format.GetConfigQueryResolver(config).QueryFields(), 3)
}
