package lark

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// fullURL is a service URL with a host, a token path, and a secret.
const fullURL = "lark://open.larksuite.com/token?secret=sss"

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

// TestConfigURLRoundTrip verifies that a config parsed from a URL builds the
// same URL again.
func TestConfigURLRoundTrip(t *testing.T) {
	t.Parallel()

	config := &Config{}
	require.NoError(t, config.SetURL(parseTestURL(t, fullURL)))

	assert.Equal(t, fullURL, config.GetURL().String())
}

// TestConfigSetURL verifies the host, path, and query values that SetURL
// stores for valid URLs.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists valid URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "lark host with query values",
			rawURL: "lark://open.larksuite.com/token?secret=sss&title=Hello&link=https%3A%2F%2Fexample.com",
			want:   Config{Host: larkHost, Path: "token", Secret: "sss", Title: "Hello", Link: "https://example.com"},
		},
		{
			name:   "feishu host",
			rawURL: "lark://open.feishu.cn/token",
			want:   Config{Host: feishuHost, Path: "token"},
		},
		{
			name:   "surrounding slashes are trimmed",
			rawURL: "lark://open.larksuite.com/token/",
			want:   Config{Host: larkHost, Path: "token"},
		},
		{
			name:   "empty host defaults to lark",
			rawURL: "lark:///token",
			want:   Config{Host: larkHost, Path: "token"},
		},
		{
			name:   "user named dummy keeps the feishu host",
			rawURL: "lark://dummy@open.feishu.cn/token",
			want:   Config{Host: feishuHost, Path: "token"},
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

// TestConfigSetURLRejectsInvalidURLs verifies that SetURL rejects unknown
// hosts, missing paths, and unknown query keys.
func TestConfigSetURLRejectsInvalidURLs(t *testing.T) {
	t.Parallel()

	// tests lists invalid URLs with the error each should produce.
	tests := []struct {
		name    string
		rawURL  string
		wantErr error
	}{
		{name: "invalid host", rawURL: "lark://invalid.com/token", wantErr: ErrInvalidHost},
		{name: "no path", rawURL: "lark://open.larksuite.com", wantErr: ErrNoPath},
		{name: "user named dummy on an invalid host", rawURL: "lark://dummy@invalid.com/token", wantErr: ErrInvalidHost},
		{name: "user named dummy without a path", rawURL: "lark://dummy@open.larksuite.com", wantErr: ErrNoPath},
		{name: "unknown query key", rawURL: "lark://open.larksuite.com/token?secret=sss&foo=bar", wantErr: format.ErrInvalidConfigKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&Config{}).SetURL(parseTestURL(t, tt.rawURL))
			require.ErrorIs(t, err, tt.wantErr)
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

// TestConfigFieldsAndEnums verifies that the config exposes the secret, title,
// and link query fields and no enums.
func TestConfigFieldsAndEnums(t *testing.T) {
	t.Parallel()

	config := &Config{}

	assert.Empty(t, config.Enums())
	assert.ElementsMatch(t, []string{"secret", "title", "link"}, format.GetConfigQueryResolver(config).QueryFields())
}
