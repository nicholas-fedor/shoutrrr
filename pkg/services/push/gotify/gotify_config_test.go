package gotify

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// Values shared by the tests.
const (
	// testToken is a legacy application token.
	testToken = "AzyoeNS.D4iJLVa"
	// testHost is the Gotify server in the test URLs.
	testHost = "gotify.example.com"
	// testServiceURL is a service URL with only the host and token.
	testServiceURL = "gotify://" + testHost + "/" + testToken
	// defaultTitle is the title Initialize gives the config.
	defaultTitle = "Shoutrrr notification"
	// enhancedToken is a Gotify 3 application token: the gtfya. prefix and
	// 43 base64url characters.
	enhancedToken = "gtfya.Kq3x9ZP0m4tW7vL2sR8yB1cN5hJ6dF0gUeXoAiQrTzM"
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

// TestConfigSetURL verifies that SetURL reads the host from the URL authority,
// the token from the last path segment with any subpath before it, and the
// other settings and the extras JSON from the query.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{name: "host and token", rawURL: testServiceURL, want: Config{Host: testHost, Token: testToken}},
		{name: "host with a port", rawURL: "gotify://" + testHost + ":8443/" + testToken, want: Config{Host: testHost + ":8443", Token: testToken}},
		{name: "trailing slash", rawURL: testServiceURL + "/", want: Config{Host: testHost, Token: testToken}},
		{name: "subpath", rawURL: "gotify://" + testHost + "/gotify/" + testToken, want: Config{Host: testHost, Path: "/gotify/", Token: testToken}},
		{
			name:   "subpath with a trailing slash",
			rawURL: "gotify://" + testHost + "/push/gotify/" + testToken + "/",
			want:   Config{Host: testHost, Path: "/push/gotify/", Token: testToken},
		},
		{name: "Gotify 3 token", rawURL: "gotify://" + testHost + "/" + enhancedToken, want: Config{Host: testHost, Token: enhancedToken}},
		{name: "no token", rawURL: "gotify://" + testHost + "/", want: Config{Host: testHost}},
		{name: "no path", rawURL: "gotify://" + testHost, want: Config{Host: testHost}},
		{
			name:   "query settings",
			rawURL: testServiceURL + "?priority=5&title=Backup&disabletls=yes&insecureskipverify=yes&useheader=yes&date=2023-12-25T10:00:00Z",
			want: Config{
				Host: testHost, Token: testToken, Priority: 5, Title: "Backup",
				DisableTLS: true, InsecureSkipVerify: true, UseHeader: true, Date: "2023-12-25T10:00:00Z",
			},
		},
		{
			name:   "extras",
			rawURL: testServiceURL + "?extras=" + url.QueryEscape(`{"client::display":{"contentType":"text/markdown"}}`),
			want: Config{
				Host: testHost, Token: testToken,
				Extras: map[string]any{"client::display": map[string]any{"contentType": "text/markdown"}},
			},
		},
		{name: "empty extras", rawURL: testServiceURL + "?extras=", want: Config{Host: testHost, Token: testToken}},
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

// TestConfigSetURLErrors verifies that SetURL rejects extras that are not a
// JSON object and query values that do not set a config property.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "extras not JSON", rawURL: testServiceURL + "?extras=" + url.QueryEscape("{invalid"), wantErr: ErrExtrasParseFailed},
		{name: "extras not an object", rawURL: testServiceURL + "?extras=" + url.QueryEscape(`["a"]`), wantErr: ErrExtrasParseFailed},
		{name: "unknown key", rawURL: testServiceURL + "?foo=bar", wantErr: ErrConfigPropertyFailed},
		{name: "priority not a number", rawURL: testServiceURL + "?priority=high", wantErr: ErrConfigPropertyFailed},
		{name: "invalid bool", rawURL: testServiceURL + "?useheader=maybe", wantErr: ErrConfigPropertyFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL writes the host, subpath, and token
// into the URL, the settings that differ from their defaults and the extras
// JSON into the query, and round-trips through SetURL. The configs start with
// the default title, as Initialize creates them.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the URL each should produce.
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "host and token", config: Config{Host: testHost, Token: testToken, Title: defaultTitle}, want: testServiceURL},
		{
			name:   "subpath",
			config: Config{Host: testHost + ":8443", Path: "/gotify/", Token: testToken, Title: defaultTitle},
			want:   "gotify://" + testHost + ":8443/gotify/" + testToken,
		},
		{
			name: "every setting",
			config: Config{
				Host: testHost, Token: testToken, Priority: 5, Title: "Backup",
				DisableTLS: true, InsecureSkipVerify: true, UseHeader: true, Date: "2023-12-25T10:00:00Z",
				Extras: map[string]any{"client::display": map[string]any{"contentType": "text/markdown"}},
			},
			want: testServiceURL + "?date=2023-12-25T10%3A00%3A00Z&disabletls=Yes&insecureskipverify=Yes&priority=5&title=Backup&useheader=Yes" +
				"&extras=" + url.QueryEscape(`{"client::display":{"contentType":"text/markdown"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL := tt.config.GetURL()
			assert.Equal(t, tt.want, serviceURL.String())

			roundTrip := &Config{Title: defaultTitle}
			require.NoError(t, roundTrip.SetURL(serviceURL))
			assert.Equal(t, tt.config, *roundTrip)
		})
	}
}

// TestConfigGetURLSkipsUnmarshalableExtras verifies that GetURL leaves out
// extras that cannot be written as JSON and keeps the rest of the URL.
func TestConfigGetURLSkipsUnmarshalableExtras(t *testing.T) {
	t.Parallel()

	config := Config{Host: testHost, Token: testToken, Title: defaultTitle, Priority: 2, Extras: map[string]any{"channel": make(chan int)}}

	assert.Equal(t, testServiceURL+"?priority=2", config.GetURL().String())
}

// TestConfigQueryResolver verifies the config's query keys and that it has
// no enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	config := &Config{}

	assert.Empty(t, config.Enums())
	assert.Equal(t,
		[]string{"date", "disabletls", "insecureskipverify", "priority", "title", "useheader"},
		format.GetConfigQueryResolver(config).QueryFields())
}
