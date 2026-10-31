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
// other settings and the extras JSON from the query, noting a priority the
// query sets.
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
		{
			name:   "query settings",
			rawURL: testServiceURL + "?priority=5&title=Backup&disabletls=yes&skiptlsverify=yes&useheader=yes&date=2023-12-25T10:00:00Z",
			want: Config{
				Host: testHost, Token: testToken, Priority: 5, Title: "Backup",
				DisableTLS: true, SkipTLSVerify: true, UseHeader: true, Date: "2023-12-25T10:00:00Z", prioritySet: true,
			},
		},
		{name: "zero priority", rawURL: testServiceURL + "?priority=0", want: Config{Host: testHost, Token: testToken, prioritySet: true}},
		{name: "zero priority in another case", rawURL: testServiceURL + "?PRIORITY=0", want: Config{Host: testHost, Token: testToken, prioritySet: true}},
		{
			name:   "deprecated key",
			rawURL: testServiceURL + "?insecureskipverify=yes",
			want:   Config{Host: testHost, Token: testToken, SkipTLSVerify: true},
		},
		{
			name:   "replacement key wins over the deprecated key",
			rawURL: testServiceURL + "?insecureskipverify=yes&skiptlsverify=no",
			want:   Config{Host: testHost, Token: testToken},
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
		{
			name:   "extras with a repeated key keep the last",
			rawURL: testServiceURL + "?extras=" + url.QueryEscape(`{"a":1,"a":"x"}`),
			want:   Config{Host: testHost, Token: testToken, Extras: map[string]any{"a": "x"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// The priority flag from an earlier URL does not carry over.
			config := &Config{prioritySet: true}
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, *config)
		})
	}
}

// TestConfigSetURLErrors verifies that SetURL rejects a URL without a host, a
// missing or malformed token, extras that are not a JSON object, and query
// values that do not set a config property, keeping the key and the cause in
// the error.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce and
	// the cause it should keep.
	tests := []struct {
		wantErr   error
		wantCause error
		name      string
		rawURL    string
		wantInErr string
	}{
		{name: "extras not JSON", rawURL: testServiceURL + "?extras=" + url.QueryEscape("{invalid"), wantErr: ErrExtrasParseFailed, wantInErr: "invalid character"},
		{name: "extras not an object", rawURL: testServiceURL + "?extras=" + url.QueryEscape(`["a"]`), wantErr: ErrExtrasParseFailed, wantInErr: "unmarshal JSON array into Go map"},
		{name: "missing host", rawURL: "gotify:///Aaa.bbb.ccc.ddd", wantErr: ErrHostMissing},
		{name: "port without a host", rawURL: "gotify://:8080/Aaa.bbb.ccc.ddd", wantErr: ErrHostMissing},
		{name: "token as an opaque path", rawURL: "gotify:/00", wantErr: ErrHostMissing},
		{name: "no token", rawURL: "gotify://" + testHost + "/", wantErr: ErrInvalidToken},
		{name: "no path", rawURL: "gotify://" + testHost, wantErr: ErrInvalidToken},
		{name: "only slashes", rawURL: "gotify://" + testHost + "///", wantErr: ErrInvalidToken},
		{name: "legacy token too short", rawURL: "gotify://" + testHost + "/Aaa", wantErr: ErrInvalidToken},
		{name: "legacy token without the A prefix", rawURL: "gotify://" + testHost + "/Bzyoe.S.D4iJLVa", wantErr: ErrInvalidToken},
		{name: "unknown key", rawURL: testServiceURL + "?foo=bar", wantErr: ErrConfigPropertyFailed, wantCause: format.ErrInvalidConfigKey, wantInErr: `"foo"`},
		{name: "priority not a number", rawURL: testServiceURL + "?priority=high", wantErr: ErrConfigPropertyFailed, wantInErr: `"priority"`},
		{name: "invalid bool", rawURL: testServiceURL + "?useheader=maybe", wantErr: ErrConfigPropertyFailed, wantCause: format.ErrInvalidBoolValue, wantInErr: `"useheader"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&Config{}).SetURL(parseTestURL(t, tt.rawURL))

			require.ErrorIs(t, err, tt.wantErr)

			if tt.wantCause != nil {
				require.ErrorIs(t, err, tt.wantCause)
			}

			assert.Contains(t, err.Error(), tt.wantInErr)
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
				DisableTLS: true, SkipTLSVerify: true, UseHeader: true, Date: "2023-12-25T10:00:00Z",
				Extras:      map[string]any{"client::display": map[string]any{"contentType": "text/markdown"}},
				prioritySet: true,
			},
			want: testServiceURL + "?date=2023-12-25T10%3A00%3A00Z&disabletls=Yes&priority=5&skiptlsverify=Yes&title=Backup&useheader=Yes" +
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

// TestConfigGetURLDeprecatedField verifies that a config built with the old
// InsecureSkipVerify field, as programs that build the config themselves do,
// writes skiptlsverify to its URL. The config is not changed, and the URL
// reads back with SkipTLSVerify set.
func TestConfigGetURLDeprecatedField(t *testing.T) {
	t.Parallel()

	config := &Config{Host: testHost, Path: "/gotify", DisableTLS: true, InsecureSkipVerify: true, Token: testToken}

	serviceURL := config.GetURL()
	assert.Equal(t, "gotify://"+testHost+"/gotify/"+testToken+"?disabletls=Yes&skiptlsverify=Yes&title=", serviceURL.String())
	assert.Equal(t, &Config{Host: testHost, Path: "/gotify", DisableTLS: true, InsecureSkipVerify: true, Token: testToken}, config)

	roundTrip := &Config{}
	require.NoError(t, roundTrip.SetURL(serviceURL))
	assert.True(t, roundTrip.SkipTLSVerify)
	assert.False(t, roundTrip.InsecureSkipVerify)
}

// TestConfigGetURLSubpath verifies that GetURL separates a subpath without a
// trailing slash from the token.
func TestConfigGetURLSubpath(t *testing.T) {
	t.Parallel()

	config := Config{Host: testHost, Path: "/gotify", Token: testToken, Title: defaultTitle}
	serviceURL := config.GetURL()

	assert.Equal(t, "gotify://"+testHost+"/gotify/"+testToken, serviceURL.String())

	roundTrip := &Config{Title: defaultTitle}
	require.NoError(t, roundTrip.SetURL(serviceURL))
	assert.Equal(t, Config{Host: testHost, Path: "/gotify/", Token: testToken, Title: defaultTitle}, *roundTrip)
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
		[]string{"date", "disabletls", "insecureskipverify", "priority", "skiptlsverify", "title", "useheader"},
		format.GetConfigQueryResolver(config).QueryFields())
}
