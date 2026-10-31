package ntfy

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Values shared by the tests.
const (
	// testTopic is the topic in the test URLs.
	testTopic = "alerts"
	// testServiceURL is a service URL for testTopic on ntfy.sh.
	testServiceURL = "ntfy://ntfy.sh/" + testTopic
	// testEndpoint is the publish URL for testServiceURL.
	testEndpoint = "https://ntfy.sh/" + testTopic
	// testToken is an ntfy access token.
	testToken = "tk_AgQdq7mVBoFD37zQVN29RhuMzNIz2"
	// longTopic is a topic of the 64 characters ntfy allows, using every kind
	// of character a topic name may contain.
	longTopic = "Alerts_2026-" + "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
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

// defaultConfig returns the config a URL with only testTopic produces, with
// the given overrides applied.
//
// Parameters:
//   - override: changes to the default config.
//
// Returns:
//   - Config: the config.
func defaultConfig(override func(*Config)) Config {
	config := Config{
		Host:     "ntfy.sh",
		Topic:    testTopic,
		Scheme:   "https",
		Priority: PriorityDefault,
		Cache:    true,
		Firebase: true,
	}
	override(&config)

	return config
}

// withoutEmptyLists returns config with tag and action lists that hold only
// an empty string set to nil. Default props leave those lists as [""], which
// the service sends as no header, so the tests compare them as empty.
//
// Parameters:
//   - original: the config.
//
// Returns:
//   - Config: a copy of the config with empty lists as nil.
func withoutEmptyLists(original *Config) Config {
	config := *original
	if len(config.Tags) == 1 && config.Tags[0] == "" {
		config.Tags = nil
	}

	if len(config.Actions) == 1 && config.Actions[0] == "" {
		config.Actions = nil
	}

	return config
}

// TestConfigSetURL verifies that SetURL reads the credentials and host from
// the URL authority, the topic from the path, and the other settings from the
// query, clearing credentials and a token that the URL does not set.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{name: "topic", rawURL: testServiceURL, want: defaultConfig(func(*Config) {})},
		{
			name:   "omitted host",
			rawURL: "ntfy:///" + testTopic,
			want:   defaultConfig(func(*Config) {}),
		},
		{
			name:   "omitted host with a port",
			rawURL: "ntfy://:8080/" + testTopic,
			want:   defaultConfig(func(c *Config) { c.Host = "ntfy.sh:8080" }),
		},
		{
			name:   "self-hosted server with a port",
			rawURL: "ntfy://ntfy.example.com:8080/" + testTopic,
			want:   defaultConfig(func(c *Config) { c.Host = "ntfy.example.com:8080" }),
		},
		{
			name:   "topic of 64 characters with dashes and underscores",
			rawURL: "ntfy://ntfy.sh/" + longTopic,
			want:   defaultConfig(func(c *Config) { c.Topic = longTopic }),
		},
		{
			name:   "nested topic",
			rawURL: "ntfy://ntfy.example.com/ntfy/" + testTopic,
			want:   defaultConfig(func(c *Config) { c.Host, c.Topic = "ntfy.example.com", "ntfy/"+testTopic }),
		},
		{
			name:   "username and password",
			rawURL: "ntfy://user:pass@ntfy.sh/" + testTopic,
			want:   defaultConfig(func(c *Config) { c.Username, c.Password = "user", "pass" }),
		},
		{
			name:   "username only",
			rawURL: "ntfy://user@ntfy.sh/" + testTopic,
			want:   defaultConfig(func(c *Config) { c.Username = "user" }),
		},
		{
			name:   "token",
			rawURL: testServiceURL + "?token=" + testToken,
			want:   defaultConfig(func(c *Config) { c.Token = testToken }),
		},
		{
			name: "message settings",
			rawURL: testServiceURL + "?title=Backup&tags=warning,skull&priority=high&click=https://example.com" +
				"&attach=https://example.com/log.txt&filename=log.txt&delay=30m&email=ops@example.com&icon=https://example.com/icon.png",
			want: defaultConfig(func(c *Config) {
				c.Title, c.Tags, c.Priority, c.Click = "Backup", []string{"warning", "skull"}, PriorityHigh, "https://example.com"
				c.Attach, c.Filename, c.Delay = "https://example.com/log.txt", "log.txt", "30m"
				c.Email, c.Icon = "ops@example.com", "https://example.com/icon.png"
			}),
		},
		{
			name:   "actions separated by semicolons",
			rawURL: testServiceURL + "?actions=view, Open, https://example.com; http, Ping, https://example.com/ping",
			want: defaultConfig(func(c *Config) {
				c.Actions = []string{"view, Open, https://example.com", " http, Ping, https://example.com/ping"}
			}),
		},
		{
			name:   "delivery switches",
			rawURL: testServiceURL + "?markdown=yes&cache=no&firebase=no&skiptlsverify=yes&disabletls=yes",
			want: defaultConfig(func(c *Config) {
				c.Markdown, c.Cache, c.Firebase, c.SkipTLSVerify, c.DisableTLS = true, false, false, true, true
			}),
		},
		{
			name:   "deprecated verification key",
			rawURL: testServiceURL + "?disabletlsverification=yes",
			want:   defaultConfig(func(c *Config) { c.SkipTLSVerify = true }),
		},
		{
			name:   "replacement key wins over the deprecated key",
			rawURL: testServiceURL + "?disabletlsverification=yes&skiptlsverify=no",
			want:   defaultConfig(func(*Config) {}),
		},
		{
			name:   "deprecated http scheme turns TLS off and is kept",
			rawURL: testServiceURL + "?scheme=http",
			want:   defaultConfig(func(c *Config) { c.Scheme, c.DisableTLS = "http", true }),
		},
		{
			name:   "disabletls wins over the deprecated scheme",
			rawURL: testServiceURL + "?scheme=http&disabletls=no",
			want:   defaultConfig(func(*Config) {}),
		},
		{
			name:   "deprecated https scheme",
			rawURL: testServiceURL + "?scheme=HTTPS",
			want:   defaultConfig(func(c *Config) { c.Scheme = "HTTPS" }),
		},
		{name: "at alias", rawURL: testServiceURL + "?at=tomorrow,%2010am", want: defaultConfig(func(c *Config) { c.Delay = "tomorrow, 10am" })},
		{name: "in alias", rawURL: testServiceURL + "?in=10m", want: defaultConfig(func(c *Config) { c.Delay = "10m" })},
		{name: "numeric priority", rawURL: testServiceURL + "?priority=1", want: defaultConfig(func(c *Config) { c.Priority = PriorityMin })},
		{name: "urgent priority", rawURL: testServiceURL + "?priority=urgent", want: defaultConfig(func(c *Config) { c.Priority = PriorityMax })},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := newDefaultConfig(t)
			// Credentials and a token from an earlier URL do not carry over.
			config.Username, config.Password, config.Token = "old", "old", "old"

			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.want, withoutEmptyLists(config))
		})
	}
}

// TestConfigSetURLErrors verifies that SetURL requires a topic, rejects a
// topic name outside ntfy's rule and a path with an empty segment, without
// echoing the topic, and rejects query values that do not set a config
// property.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "no topic", rawURL: "ntfy://ntfy.sh", wantErr: ErrTopicRequired},
		{name: "no topic with a slash", rawURL: "ntfy://ntfy.sh/", wantErr: ErrTopicRequired},
		{name: "only slashes", rawURL: "ntfy://ntfy.sh//", wantErr: ErrInvalidTopic},
		{name: "leading empty segment", rawURL: "ntfy://ntfy.sh//SECRETtopic", wantErr: ErrInvalidTopic},
		{name: "empty base path segment", rawURL: "ntfy://ntfy.sh/ntfy//SECRETtopic", wantErr: ErrInvalidTopic},
		{name: "trailing slash", rawURL: "ntfy://ntfy.sh/SECRETtopic/", wantErr: ErrInvalidTopic},
		{name: "space in the topic", rawURL: "ntfy://ntfy.sh/SECRET%20topic", wantErr: ErrInvalidTopic},
		{name: "dot in the topic", rawURL: "ntfy://ntfy.sh/SECRET.topic", wantErr: ErrInvalidTopic},
		{name: "topic over 64 characters", rawURL: "ntfy://ntfy.sh/SECRET" + strings.Repeat("a", 59), wantErr: ErrInvalidTopic},
		{name: "unbracketed IPv6 host", rawURL: "ntfy://2001:db8::1/" + testTopic, wantErr: util.ErrInvalidHost},
		{name: "unknown key", rawURL: testServiceURL + "?foo=bar", wantErr: format.ErrInvalidConfigKey},
		{name: "invalid priority", rawURL: testServiceURL + "?priority=extreme", wantErr: format.ErrInvalidEnumValue},
		{name: "invalid bool", rawURL: testServiceURL + "?cache=maybe", wantErr: format.ErrInvalidBoolValue},
		{name: "invalid scheme", rawURL: testServiceURL + "?scheme=ftp", wantErr: ErrInvalidScheme},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newDefaultConfig(t).SetURL(parseTestURL(t, tt.rawURL))

			require.ErrorIs(t, err, tt.wantErr)
			assert.NotContains(t, err.Error(), "SECRET")
		})
	}
}

// TestConfigGetAPIURL verifies that the publish URL joins the scheme, host,
// and topic with one slash. It uses HTTP when TLS is disabled or the scheme
// is http in any case, and HTTPS otherwise.
func TestConfigGetAPIURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the publish URL each should produce.
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{name: "topic", config: Config{Scheme: "https", Host: "ntfy.sh", Topic: testTopic}, want: testEndpoint},
		{name: "topic with a slash", config: Config{Scheme: "https", Host: "ntfy.sh", Topic: "/" + testTopic}, want: testEndpoint},
		{name: "nested topic", config: Config{Scheme: "https", Host: "ntfy.example.com", Topic: "ntfy/" + testTopic}, want: "https://ntfy.example.com/ntfy/" + testTopic},
		{name: "HTTP", config: Config{Scheme: "http", Host: "ntfy.example.com:8080", Topic: testTopic}, want: "http://ntfy.example.com:8080/" + testTopic},
		{name: "TLS disabled", config: Config{Scheme: "https", Host: "ntfy.example.com:8080", Topic: testTopic, DisableTLS: true}, want: "http://ntfy.example.com:8080/" + testTopic},
		{name: "uppercase HTTP", config: Config{Scheme: "HTTP", Host: "ntfy.example.com", Topic: testTopic}, want: "http://ntfy.example.com/" + testTopic},
		{name: "no scheme", config: Config{Host: "ntfy.sh", Topic: testTopic}, want: testEndpoint},
		{name: "invalid scheme", config: Config{Scheme: "ftp", Host: "ntfy.sh", Topic: testTopic}, want: testEndpoint},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.config.GetAPIURL())
		})
	}
}

// TestConfigGetURL verifies that GetURL writes the credentials, host, and
// topic into the URL and only the settings that differ from their defaults
// into the query, and round-trips through SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the URL parts and query values each should
	// produce.
	tests := []struct {
		wantQuery url.Values
		name      string
		wantUser  string
		config    Config
	}{
		{name: "defaults", config: defaultConfig(func(*Config) {}), wantQuery: url.Values{}},
		{name: "username only", config: defaultConfig(func(c *Config) { c.Username = "user" }), wantUser: "user", wantQuery: url.Values{}},
		{
			name: "every setting",
			config: defaultConfig(func(c *Config) {
				c.Username, c.Password, c.Token = "user", "pass", testToken
				c.Title, c.Tags, c.Priority = "Backup", []string{"warning", "skull"}, PriorityMax
				c.Actions = []string{"view, Open, https://example.com", "http, Ping, https://example.com/ping"}
				c.Click, c.Attach, c.Filename, c.Delay = "https://example.com", "https://example.com/log.txt", "log.txt", "30m"
				c.Email, c.Icon, c.Markdown, c.Cache, c.Firebase = "ops@example.com", "https://example.com/icon.png", true, false, false
				c.SkipTLSVerify = true
			}),
			wantUser: "user:pass",
			wantQuery: url.Values{
				"actions": {"view, Open, https://example.com;http, Ping, https://example.com/ping"}, "attach": {"https://example.com/log.txt"},
				"cache": {"No"}, "click": {"https://example.com"}, "delay": {"30m"}, "skiptlsverify": {"Yes"},
				"email": {"ops@example.com"}, "filename": {"log.txt"}, "firebase": {"No"}, "icon": {"https://example.com/icon.png"},
				"markdown": {"Yes"}, "priority": {"Max"}, "tags": {"warning,skull"}, "title": {"Backup"}, "token": {testToken},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL := tt.config.GetURL()

			assert.Equal(t, Scheme, serviceURL.Scheme)
			assert.Equal(t, tt.wantUser, serviceURL.User.String())
			assert.Equal(t, "ntfy.sh", serviceURL.Host)
			assert.Equal(t, "/"+testTopic, serviceURL.Path)

			assert.Equal(t, tt.wantQuery, serviceURL.Query())

			roundTrip := newDefaultConfig(t)
			require.NoError(t, roundTrip.SetURL(serviceURL))
			assert.Equal(t, tt.config, withoutEmptyLists(roundTrip))
		})
	}
}

// TestConfigGetURLDeprecated verifies that GetURL writes a value set through
// a deprecated field under its replacement's key, and an http scheme as
// disabletls, without changing the config. An invalid scheme is kept, so
// reading the URL reports it.
func TestConfigGetURLDeprecated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		wantQuery url.Values
		config    Config
	}{
		{
			name:      "deprecated verification field",
			config:    defaultConfig(func(c *Config) { c.DisableTLSVerification = true }),
			wantQuery: url.Values{"skiptlsverify": {"Yes"}},
		},
		{
			name:      "http scheme",
			config:    defaultConfig(func(c *Config) { c.Scheme = "http" }),
			wantQuery: url.Values{"disabletls": {"Yes"}},
		},
		{
			name:      "uppercase https scheme",
			config:    defaultConfig(func(c *Config) { c.Scheme = "HTTPS" }),
			wantQuery: url.Values{},
		},
		{
			name:      "invalid scheme",
			config:    defaultConfig(func(c *Config) { c.Scheme = "ftp" }),
			wantQuery: url.Values{"scheme": {"ftp"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			original := tt.config

			assert.Equal(t, tt.wantQuery, tt.config.GetURL().Query())
			assert.Equal(t, original, tt.config)
		})
	}
}

// TestConfigSetURLKeepsCallerURL verifies that reading a URL whose query has
// a literal semicolon leaves the caller's URL as it was.
func TestConfigSetURLKeepsCallerURL(t *testing.T) {
	t.Parallel()

	serviceURL := parseTestURL(t, testServiceURL+"?actions=view, Open, https://example.com; http, Ping, https://example.com/ping")
	rawQuery := serviceURL.RawQuery

	require.NoError(t, newDefaultConfig(t).SetURL(serviceURL))
	assert.Equal(t, rawQuery, serviceURL.RawQuery)
}

// TestConfigReadAfterParse verifies the fields that a program reading the
// parsed config sees, as programs that build their own requests from it do.
// An http scheme keeps Scheme as written and sets DisableTLS too.
func TestConfigReadAfterParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		rawURL         string
		wantScheme     string
		wantDisableTLS bool
	}{
		{name: "defaults", rawURL: testServiceURL, wantScheme: "https"},
		{name: "http scheme", rawURL: testServiceURL + "?scheme=http", wantScheme: "http", wantDisableTLS: true},
		{name: "disabletls", rawURL: testServiceURL + "?disabletls=yes", wantScheme: "https", wantDisableTLS: true},
		{name: "disabletls over an http scheme", rawURL: testServiceURL + "?scheme=http&DisableTLS=no", wantScheme: "https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := newDefaultConfig(t)
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.wantScheme, config.Scheme)
			assert.Equal(t, tt.wantDisableTLS, config.DisableTLS)
		})
	}
}

// TestConfigQueryResolver verifies the config's query keys and that priority
// is its only enum.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	config := &Config{}

	enums := config.Enums()
	assert.Len(t, enums, 1)
	assert.Same(t, Priority.Enum, enums["Priority"])
	assert.Equal(t, []string{
		"actions", "at", "attach", "cache", "click", "delay", "disabletls", "disabletlsverification", "email", "filename",
		"firebase", "icon", "in", "markdown", "priority", "scheme", "skiptlsverify", "tags", "title", "token",
	}, config.QueryFields())
}
