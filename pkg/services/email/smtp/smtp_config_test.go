package smtp

import (
	"crypto/tls"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// allPropsURL is a service URL that sets every property GetURL writes.
const allPropsURL = "smtp://user:password@example.com:2225/?auth=None&clienthost=testhost&encryption=ExplicitTLS" +
	"&fromaddress=sender%40example.com&fromname=Sender&subject=Subject&toaddresses=rec1%40example.com%2Crec2%40example.com" +
	"&usehtml=Yes&usestarttls=No&timeout=10s"

// parseConfigURL parses rawURL and fails the test if it is invalid.
//
// Parameters:
//   - t: the test.
//   - rawURL: the URL to parse.
//
// Returns:
//   - *url.URL: the parsed URL.
func parseConfigURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	return parsed
}

// newInitialConfig returns the config Initialize starts from before it parses
// the service URL.
//
// Returns:
//   - *Config: the config.
func newInitialConfig() *Config {
	return &Config{
		Port:        DefaultSMTPPort,
		Auth:        AuthUnknown,
		UseStartTLS: true,
		Encryption:  EncAuto,
		ClientHost:  "localhost",
		Timeout:     defaultTimeout,
	}
}

// newDefaultConfig returns a config with the default values of its key props.
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

// TestConfigClone verifies that Clone copies every field, with a recipient
// list that later changes to the clone do not affect.
func TestConfigClone(t *testing.T) {
	t.Parallel()

	config := &Config{}
	require.NoError(t, config.SetURL(parseConfigURL(t, allPropsURL)))

	clone := config.Clone()
	assert.Equal(t, *config, clone)

	clone.ToAddresses[0] = "changed@example.com"
	assert.Equal(t, []string{"rec1@example.com", "rec2@example.com"}, config.ToAddresses)
}

// TestConfigEnums verifies that the authentication and encryption fields use
// their enum formatters.
func TestConfigEnums(t *testing.T) {
	t.Parallel()

	enums := (&Config{}).Enums()

	assert.Len(t, enums, 2)
	assert.Same(t, AuthTypes.Enum, enums["Auth"])
	assert.Same(t, EncMethods.Enum, enums["Encryption"])
}

// TestConfigQueryResolverGet verifies that the config's query resolver rejects
// a key the config does not declare.
func TestConfigQueryResolverGet(t *testing.T) {
	t.Parallel()

	value, err := format.GetConfigQueryResolver(&Config{}).Get("unknown")

	require.ErrorIs(t, err, format.ErrInvalidConfigKey)
	assert.Empty(t, value)
}

// TestConfigSetURL verifies that SetURL reads the credentials, host, and port
// from the URL authority and the other settings from the query, restoring plus
// tags in addresses that query decoding turned into spaces and dropping
// recipient entries that are blank after trimming.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// base returns the config a URL with only the addresses produces from the
	// initial config, with the given overrides applied.
	base := func(override func(*Config)) Config {
		config := *newInitialConfig()
		config.Host = "mail.example.com"
		config.FromAddress = "sender@example.com"
		config.ToAddresses = []string{"rec@example.com"}
		override(&config)

		return config
	}

	// tests lists service URLs with the overrides of the config each should
	// produce.
	tests := []struct {
		override func(*Config)
		name     string
		rawURL   string
	}{
		{
			name:     "addresses only",
			rawURL:   "smtp://mail.example.com/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			override: func(*Config) {},
		},
		{
			name:   "every setting",
			rawURL: allPropsURL + "&requirestarttls=yes&skiptlsverify=yes",
			override: func(c *Config) {
				c.Host, c.Port, c.Username, c.Password = "example.com", 2225, "user", "password"
				c.Auth, c.ClientHost, c.Encryption, c.FromName, c.Subject = AuthNone, "testhost", EncExplicitTLS, "Sender", "Subject"
				c.ToAddresses = []string{"rec1@example.com", "rec2@example.com"}
				c.UseHTML, c.UseStartTLS, c.RequireStartTLS, c.SkipTLSVerify = true, false, true, true
			},
		},
		{
			name:     "aliases",
			rawURL:   "smtp://mail.example.com/?from=sender@example.com&to=rec@example.com&title=Alert&starttls=no",
			override: func(c *Config) { c.Subject, c.UseStartTLS = "Alert", false },
		},
		{
			name:     "password with @ and spaces",
			rawURL:   "smtp://user:p%40ss%20word@mail.example.com:587/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			override: func(c *Config) { c.Username, c.Password, c.Port = "user", "p@ss word", 587 },
		},
		{
			name:     "IPv6 host",
			rawURL:   "smtp://[::1]:587/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			override: func(c *Config) { c.Host, c.Port = "::1", 587 },
		},
		{
			name:   "plus tags",
			rawURL: "smtp://mail.example.com/?fromaddress=sender+alerts@example.com&toaddresses=rec+a@example.com,other+b@example.com",
			override: func(c *Config) {
				c.FromAddress, c.ToAddresses = "sender+alerts@example.com", []string{"rec+a@example.com", "other+b@example.com"}
			},
		},
		{
			name:   "spaces around addresses",
			rawURL: "smtp://mail.example.com/?fromaddress=%20sender+alerts@example.com%20&toaddresses=a@example.com,%20c+x@example.com%20,d@example.com",
			override: func(c *Config) {
				c.FromAddress, c.ToAddresses = "sender+alerts@example.com", []string{"a@example.com", "c+x@example.com", "d@example.com"}
			},
		},
		{
			name:     "blank recipient entries",
			rawURL:   "smtp://mail.example.com/?fromaddress=sender@example.com&toaddresses=rec@example.com,%20,",
			override: func(*Config) {},
		},
		{
			name:   "display name with a plus-tagged address",
			rawURL: "smtp://mail.example.com/?fromaddress=Ops%20Team%20%3Cops+alerts@example.com%3E&toaddresses=Jane%20Doe%20%3Cjane+x@example.com%3E",
			override: func(c *Config) {
				c.FromAddress, c.ToAddresses = "Ops Team <ops+alerts@example.com>", []string{"Jane Doe <jane+x@example.com>"}
			},
		},
		{
			name:     "encoded semicolon",
			rawURL:   "smtp://mail.example.com/?fromaddress=sender@example.com&toaddresses=a@b.com%3Bc@d.com",
			override: func(c *Config) { c.ToAddresses = []string{"a@b.com;c@d.com"} },
		},
		{
			name:     "timeout",
			rawURL:   "smtp://mail.example.com/?fromaddress=sender@example.com&toaddresses=rec@example.com&timeout=45s",
			override: func(c *Config) { c.Timeout = 45 * time.Second },
		},
		{
			name:     "skiptlsverify no",
			rawURL:   "smtp://mail.example.com/?fromaddress=sender@example.com&toaddresses=rec@example.com&skiptlsverify=no",
			override: func(*Config) {},
		},
		{
			name:     "port out of range keeps the previous port",
			rawURL:   "smtp://mail.example.com:99999/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			override: func(c *Config) { c.Port = DefaultSMTPPort },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := newInitialConfig()
			require.NoError(t, config.SetURL(parseConfigURL(t, tt.rawURL)))

			want := base(tt.override)
			assert.Equal(t, want, *config)
		})
	}
}

// TestConfigSetURLErrors verifies that SetURL requires the sender and the
// recipient addresses, including ones that are blank after trimming, and
// rejects invalid query values.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "no sender", rawURL: "smtp://mail.example.com/?toaddresses=rec@example.com", wantErr: ErrFromAddressMissing},
		{name: "missing host", rawURL: "smtp:///?fromaddress=sender@example.com&toaddresses=rec@example.com", wantErr: ErrHostMissing},
		{name: "port without a host", rawURL: "smtp://:587/?fromaddress=sender@example.com&toaddresses=rec@example.com", wantErr: ErrHostMissing},
		{name: "no recipients", rawURL: "smtp://mail.example.com/?fromaddress=sender@example.com", wantErr: ErrToAddressMissing},
		{name: "blank sender", rawURL: "smtp://mail.example.com/?fromaddress=%20&toaddresses=rec@example.com", wantErr: ErrFromAddressMissing},
		{name: "blank recipients", rawURL: "smtp://mail.example.com/?fromaddress=sender@example.com&toaddresses=%20,", wantErr: ErrToAddressMissing},
		{name: "unknown key", rawURL: "smtp://mail.example.com/?foo=bar", wantErr: format.ErrInvalidConfigKey},
		{
			name:    "unbracketed IPv6 host",
			rawURL:  "smtp://2001:db8::1/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			wantErr: util.ErrInvalidHost,
		},
		{name: "invalid encryption", rawURL: "smtp://mail.example.com/?encryption=SSL", wantErr: format.ErrInvalidEnumValue},
		{
			name:    "invalid timeout",
			rawURL:  "smtp://mail.example.com/?fromaddress=sender@example.com&toaddresses=rec@example.com&timeout=soon",
			wantErr: format.ErrParseDurationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, newInitialConfig().SetURL(parseConfigURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigSetURLResolverError verifies that SetURL names the query key whose
// value the resolver rejects, without the value.
func TestConfigSetURLResolverError(t *testing.T) {
	t.Parallel()

	errRejected := errors.New("rejected")

	resolver := typesmocks.NewMockConfigQueryResolver(t)
	resolver.EXPECT().QueryFields().Return([]string{"foo"}).Once()
	resolver.EXPECT().Set("foo", "SECRETvalue").Return(errRejected).Once()

	err := (&Config{}).setURL(resolver, parseConfigURL(t, "smtp://user@mail.example.com?foo=SECRETvalue"))

	require.ErrorIs(t, err, errRejected)
	assert.EqualError(t, err, `setting query parameter "foo": rejected`)
}

// TestConfigGetURL verifies that GetURL writes the credentials, host, and port
// into the URL authority and the settings into the query, adding
// requirestarttls and skiptlsverify only when they are on, and round-trips
// through SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	config := &Config{}
	require.NoError(t, config.SetURL(parseConfigURL(t, allPropsURL)))
	assert.Equal(t, allPropsURL, config.GetURL().String())

	config.RequireStartTLS, config.SkipTLSVerify = true, true
	serviceURL := config.GetURL()
	assert.Equal(t, allPropsURL+"&requirestarttls=Yes&skiptlsverify=Yes", serviceURL.String())

	roundTrip := &Config{}
	require.NoError(t, roundTrip.SetURL(serviceURL))
	assert.Equal(t, config, roundTrip)

	ipv6 := Config{Host: "::1", Port: 587}
	assert.Equal(t, "[::1]:587", ipv6.GetURL().Host)
}

// TestConfigGetURLResolver verifies that getURL writes the resolver's values
// for the query keys and skips keys the resolver cannot get.
func TestConfigGetURLResolver(t *testing.T) {
	t.Parallel()

	t.Run("values", func(t *testing.T) {
		t.Parallel()

		resolver := typesmocks.NewMockConfigQueryResolver(t)
		resolver.EXPECT().Get(mock.Anything).Return("v", nil).Times(10)

		got := (&Config{Host: "mail.example.com", Port: 587, Username: "user", Password: "pass"}).getURL(resolver)

		assert.Equal(t,
			"smtp://user:pass@mail.example.com:587/?auth=v&clienthost=v&encryption=v&fromaddress=v&fromname=v&subject=v&toaddresses=v&usehtml=v&usestarttls=v&timeout=v",
			got.String())
	})

	t.Run("missing keys", func(t *testing.T) {
		t.Parallel()

		resolver := typesmocks.NewMockConfigQueryResolver(t)
		resolver.EXPECT().Get(mock.Anything).Return("", errors.New("missing")).Times(10)

		got := (&Config{Host: "example.com", Port: 25}).getURL(resolver)

		assert.Equal(t, "smtp://example.com:25/?", got.String())
	})
}

// TestConfigDefaults verifies the default timeout and that the timeout reads
// as a duration in the URL.
func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	config := newDefaultConfig(t)
	assert.Equal(t, 10*time.Second, config.Timeout)

	config.Timeout = 90 * time.Second
	assert.Equal(t, "1m30s", config.GetURL().Query().Get("timeout"))
}

// TestRestorePlusAddress verifies that spaces in the local part of an
// address, which query decoding makes from plus signs, become plus signs
// again, leaving the domain and any display name unchanged. Whitespace around
// the address and inside the angle brackets is separator space, not a plus
// sign, and is trimmed.
func TestRestorePlusAddress(t *testing.T) {
	t.Parallel()

	// tests lists decoded addresses with the address each should restore to.
	tests := []struct {
		address string
		want    string
	}{
		{address: "user tag@example.com", want: "user+tag@example.com"},
		{address: "user a b@example.com", want: "user+a+b@example.com"},
		{address: "user@example.com", want: "user@example.com"},
		{address: "user tag@mail host", want: "user+tag@mail host"},
		{address: "Jane Doe <jane x@example.com>", want: "Jane Doe <jane+x@example.com>"},
		{address: "Jane Doe <jane@example.com>", want: "Jane Doe <jane@example.com>"},
		{address: "no at sign", want: "no+at+sign"},
		{address: " c x@example.com", want: "c+x@example.com"},
		{address: "b@example.com ", want: "b@example.com"},
		{address: "\tuser tag@example.com\n", want: "user+tag@example.com"},
		{address: " Jane Doe <jane x@example.com> ", want: "Jane Doe <jane+x@example.com>"},
		{address: "Jane Doe < jane x@example.com >", want: "Jane Doe <jane+x@example.com>"},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, restorePlusAddress(tt.address))
		})
	}
}

// TestNewTLSConfig verifies that the TLS config names the server, requires TLS
// 1.2 without a maximum version, and skips verification only when configured.
func TestNewTLSConfig(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		&tls.Config{ServerName: "mail.example.com", MinVersion: tls.VersionTLS12},
		newTLSConfig(&Config{Host: "mail.example.com"}))
	assert.Equal(t,
		&tls.Config{ServerName: "mail.example.com", MinVersion: tls.VersionTLS12, InsecureSkipVerify: true},
		newTLSConfig(&Config{Host: "mail.example.com", SkipTLSVerify: true}))
}
