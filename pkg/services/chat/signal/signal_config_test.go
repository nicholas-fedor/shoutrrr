package signal

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
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

// newDefaultConfig returns a config with the default values, as Initialize
// creates it.
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

// TestConfigSetURL verifies that SetURL reads the host, port, and credentials
// from the URL authority, the source and recipients from the path, and the
// other settings from the query.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// defaults returns the config a URL with only a host, source, and
	// recipients produces, with the given overrides applied.
	defaults := func(override func(*Config)) Config {
		config := Config{
			Host:       "localhost",
			Port:       8080,
			Source:     testSource,
			Recipients: []string{testRecipient},
			NotifySelf: true,
		}
		override(&config)

		return config
	}

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{name: "minimal", rawURL: testServiceURL, want: defaults(func(*Config) {})},
		{
			name:   "default port",
			rawURL: "signal://myserver/" + testSource + "/" + testRecipient,
			want:   defaults(func(c *Config) { c.Host = "myserver" }),
		},
		{
			name:   "custom port",
			rawURL: "signal://myserver:9999/" + testSource + "/" + testRecipient,
			want:   defaults(func(c *Config) { c.Host, c.Port = "myserver", 9999 }),
		},
		{
			name:   "IPv6 host without brackets",
			rawURL: "signal://[::1]:9999/" + testSource + "/" + testRecipient,
			want:   defaults(func(c *Config) { c.Host, c.Port = "::1", 9999 }),
		},
		{
			name:   "user and password",
			rawURL: "signal://user:pass@localhost:8080/" + testSource + "/" + testRecipient,
			want:   defaults(func(c *Config) { c.User, c.Password = "user", "pass" }),
		},
		{
			name:   "user without a password",
			rawURL: "signal://user@localhost:8080/" + testSource + "/" + testRecipient,
			want:   defaults(func(c *Config) { c.User = "user" }),
		},
		{
			name:   "query settings",
			rawURL: testServiceURL + "?token=tok&title=Alert&textmode=styled&notifyself=no&disabletls=yes&skiptlsverify=yes&attachments=aGVsbG8%3D",
			want: defaults(func(c *Config) {
				c.Token, c.Title, c.TextMode, c.NotifySelf = "tok", "Alert", TextModeStyled, false
				c.DisableTLS, c.SkipTLSVerify, c.Attachments = true, true, "aGVsbG8="
			}),
		},
		{
			name:   "query aliases",
			rawURL: testServiceURL + "?apikey=tok&text_mode=normal",
			want:   defaults(func(c *Config) { c.Token, c.TextMode = "tok", TextModeNormal }),
		},
		{
			name:   "recipients in the query",
			rawURL: testServiceURL + "?to=%2B15559876543,u:alice.01",
			want:   defaults(func(c *Config) { c.Recipients = []string{"+15559876543", "u:alice.01"} }),
		},
		{
			name:   "recipient types",
			rawURL: testServiceURL + "/group.ABCD/EFGH=/u:alice.01/group.ABCD+EFGH",
			want: defaults(func(c *Config) {
				c.Recipients = []string{testRecipient, "group.ABCD/EFGH=", "u:alice.01", "group.ABCD+EFGH"}
			}),
		},
		{
			name:   "docs command placeholder",
			rawURL: "signal://dummy@dummy.com",
			want: defaults(func(c *Config) {
				c.Source, c.Recipients = "+1234567890", []string{"+0987654321"}
			}),
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL without a source
// or recipients, an invalid source or recipient, and invalid query values.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "no path", rawURL: "signal://localhost:8080", wantErr: ErrNoRecipients},
		{name: "no recipients", rawURL: "signal://localhost:8080/" + testSource, wantErr: ErrNoRecipients},
		{name: "invalid source", rawURL: "signal://localhost:8080/invalid-phone/" + testRecipient, wantErr: ErrInvalidPhoneNumber},
		{name: "invalid group ID", rawURL: testServiceURL + "/group.invalid!", wantErr: ErrInvalidRecipient},
		{name: "invalid recipient", rawURL: testServiceURL + "/alice", wantErr: ErrInvalidRecipient},
		{name: "empty username", rawURL: testServiceURL + "/u:", wantErr: ErrInvalidRecipient},
		{name: "unknown query key", rawURL: testServiceURL + "?foo=bar", wantErr: format.ErrInvalidConfigKey},
		{name: "invalid text mode", rawURL: testServiceURL + "?textmode=markdown", wantErr: format.ErrInvalidEnumValue},
		{name: "invalid bool", rawURL: testServiceURL + "?notifyself=maybe", wantErr: format.ErrInvalidBoolValue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, newDefaultConfig(t).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL writes the credentials, host, port,
// source, and recipients back into the URL, and round-trips through SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the parts GetURL should write for them.
	tests := []struct {
		name     string
		rawURL   string
		wantUser string
		wantHost string
		wantPath string
	}{
		{
			name:     "phone number",
			rawURL:   testServiceURL,
			wantHost: "localhost:8080",
			wantPath: "/" + testSource + "/" + testRecipient,
		},
		{
			name:     "every setting",
			rawURL:   "signal://user:pass@myserver:9999/" + testSource + "/" + testRecipient + "/group.ABCD/EFGH=/u:alice.01?token=tok&title=Alert&textmode=styled&notifyself=no&disabletls=yes&skiptlsverify=yes&attachments=aGVsbG8%3D",
			wantUser: "user:pass",
			wantHost: "myserver:9999",
			wantPath: "/" + testSource + "/" + testRecipient + "/group.ABCD/EFGH=/u:alice.01",
		},
		{
			name:     "user without a password",
			rawURL:   "signal://user@localhost:8080/" + testSource + "/" + testRecipient,
			wantUser: "user",
			wantHost: "localhost:8080",
			wantPath: "/" + testSource + "/" + testRecipient,
		},
		{
			name:     "IPv6 host",
			rawURL:   "signal://[::1]:9999/" + testSource + "/" + testRecipient,
			wantHost: "[::1]:9999",
			wantPath: "/" + testSource + "/" + testRecipient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := newDefaultConfig(t)
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			serviceURL := config.GetURL()

			assert.Equal(t, Scheme, serviceURL.Scheme)
			assert.Equal(t, tt.wantUser, serviceURL.User.String())
			assert.Equal(t, tt.wantHost, serviceURL.Host)
			assert.Equal(t, tt.wantPath, serviceURL.Path)

			roundTrip := newDefaultConfig(t)
			require.NoError(t, roundTrip.SetURL(serviceURL))
			assert.Equal(t, config, roundTrip)
		})
	}
}

// TestConfigQueryResolver verifies the config's query keys, that text mode is
// its only enum, and that unknown keys are rejected.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	config := newDefaultConfig(t)
	resolver := format.GetConfigQueryResolver(config)

	assert.Equal(t, []string{
		"apikey", "attachments", "disabletls", "host", "notifyself", "password", "port", "recipients",
		"skiptlsverify", "source", "text_mode", "textmode", "title", "to", "token", "user",
	}, resolver.QueryFields())

	enums := config.Enums()
	assert.Len(t, enums, 1)
	assert.Same(t, TextModes.Enum, enums["TextMode"])

	_, err := resolver.Get("foo")
	require.ErrorIs(t, err, format.ErrInvalidConfigKey)
	require.ErrorIs(t, resolver.Set("foo", "bar"), format.ErrInvalidConfigKey)
}
