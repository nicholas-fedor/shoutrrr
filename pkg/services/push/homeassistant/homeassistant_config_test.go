package homeassistant

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

// TestConfigSetURL verifies that SetURL reads the token from the user info,
// the host, port, and path prefix from the URL, and the notification props
// from the query, leaving props the URL omits unset.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "token and host",
			rawURL: "homeassistant://s3cret@ha.example.com",
			want:   Config{Token: "s3cret", Host: "ha.example.com"},
		},
		{
			name:   "port, path prefix, and disabletls",
			rawURL: "homeassistant://s3cret@ha.example.com:8123/hass/?disabletls=yes",
			want:   Config{Token: "s3cret", Host: "ha.example.com", Port: 8123, Path: "/hass", DisableTLS: true},
		},
		{
			name:   "slashes only path",
			rawURL: "homeassistant://s3cret@ha.example.com///",
			want:   Config{Token: "s3cret", Host: "ha.example.com"},
		},
		{
			name: "notification props",
			rawURL: "homeassistant://s3cret@ha.example.com" +
				"?title=Update&service=notify.mobile_app_phone&targets=device1,device2&nid=watchtower&skiptlsverify=yes",
			want: Config{
				Token:         "s3cret",
				Host:          "ha.example.com",
				Title:         "Update",
				Service:       "notify.mobile_app_phone",
				Targets:       []string{"device1", "device2"},
				Nid:           "watchtower",
				SkipTLSVerify: true,
			},
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL without a token or
// host, an invalid port, an unknown query prop, and a service that is not a
// valid action name, including one that would escape the API path.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "missing token", rawURL: "homeassistant://ha.example.com", wantErr: ErrTokenMissing},
		{name: "empty token", rawURL: "homeassistant://@ha.example.com", wantErr: ErrTokenMissing},
		{name: "missing host", rawURL: "homeassistant://s3cret@", wantErr: ErrHostMissing},
		{name: "unbracketed IPv6 host", rawURL: "homeassistant://s3cret@2001:db8::1", wantErr: util.ErrInvalidHost},
		{name: "port out of range", rawURL: "homeassistant://s3cret@ha.example.com:99999", wantErr: ErrInvalidPort},
		{name: "port zero", rawURL: "homeassistant://s3cret@ha.example.com:0", wantErr: ErrInvalidPort},
		{name: "unknown prop", rawURL: "homeassistant://s3cret@ha.example.com?foo=bar", wantErr: format.ErrInvalidConfigKey},
		{name: "empty action", rawURL: "homeassistant://s3cret@ha.example.com?service=notify.", wantErr: ErrInvalidService},
		{name: "empty domain", rawURL: "homeassistant://s3cret@ha.example.com?service=.create", wantErr: ErrInvalidService},
		{name: "invalid bare name", rawURL: "homeassistant://s3cret@ha.example.com?service=mobile-app", wantErr: ErrInvalidService},
		{name: "API path", rawURL: "homeassistant://s3cret@ha.example.com?service=/api/events/foo", wantErr: ErrInvalidService},
		{name: "parent path", rawURL: "homeassistant://s3cret@ha.example.com?service=../config", wantErr: ErrInvalidService},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, newDefaultConfig(t).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL encodes the token as the user, leaves
// out the port the scheme implies, and round-trips through SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()

		config := newDefaultConfig(t)
		config.Token = "s3cret"
		config.Host = "ha.example.com"
		config.Port = 8443
		config.Path = "/hass"
		config.Title = "Update"
		config.Service = "notify.mobile_app_phone"
		config.Targets = []string{"device1"}
		config.Nid = "watchtower"
		config.DisableTLS = true

		serviceURL := config.GetURL()
		assert.Equal(t, Scheme, serviceURL.Scheme)
		assert.Equal(t, "s3cret", serviceURL.User.Username())
		assert.Equal(t, "ha.example.com:8443", serviceURL.Host)
		assert.Equal(t, "/hass", serviceURL.Path)

		roundTrip := newDefaultConfig(t)
		require.NoError(t, roundTrip.SetURL(serviceURL))
		assert.Equal(t, config, roundTrip)
	})

	// hostTests lists ports and TLS settings with the host GetURL should write.
	hostTests := []struct {
		name       string
		host       string
		wantHost   string
		port       int
		disableTLS bool
	}{
		{name: "implied HTTPS port", host: "ha.example.com", port: 443, wantHost: "ha.example.com"},
		{name: "implied HTTP port", host: "homeassistant.local", port: 8123, disableTLS: true, wantHost: "homeassistant.local"},
		{name: "non-default HTTPS port", host: "homeassistant.local", port: 8123, wantHost: "homeassistant.local:8123"},
		{name: "no port", host: "ha.example.com", wantHost: "ha.example.com"},
	}

	for _, tt := range hostTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{Token: "s3cret", Host: tt.host, Port: tt.port, DisableTLS: tt.disableTLS}

			assert.Equal(t, tt.wantHost, config.GetURL().Host)
		})
	}
}

// TestConfigDomainService verifies how the service prop maps to a domain and
// action: empty creates a persistent notification, a bare name calls a notify
// target, and a dotted value names both.
func TestConfigDomainService(t *testing.T) {
	t.Parallel()

	// tests lists service props with the domain, action, and persistence each
	// should resolve to.
	tests := []struct {
		name           string
		service        string
		wantDomain     string
		wantService    string
		wantPersistent bool
	}{
		{name: "empty", service: "", wantDomain: persistentDomain, wantService: persistentService, wantPersistent: true},
		{name: "bare name", service: "mobile_app_phone", wantDomain: notifyDomain, wantService: "mobile_app_phone"},
		{name: "domain and action", service: "notify.mobile_app_phone", wantDomain: "notify", wantService: "mobile_app_phone"},
		{
			name:           "explicit persistent notification",
			service:        "persistent_notification.create",
			wantDomain:     persistentDomain,
			wantService:    persistentService,
			wantPersistent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{Service: tt.service}
			domain, service, err := config.domainService()

			require.NoError(t, err)
			assert.Equal(t, tt.wantDomain, domain)
			assert.Equal(t, tt.wantService, service)
			assert.Equal(t, tt.wantPersistent, config.isPersistent())
		})
	}

	t.Run("invalid service", func(t *testing.T) {
		t.Parallel()

		config := &Config{Service: "notify.bad-name"}
		_, _, err := config.domainService()

		require.ErrorIs(t, err, ErrInvalidService)
		assert.False(t, config.isPersistent())
	})
}

// TestConfigAPIURL verifies the REST URL for a service call: HTTPS on 443 or
// HTTP on 8123 when the port is omitted, the configured port otherwise, and
// the reverse-proxy prefix before the API path.
func TestConfigAPIURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the URL each should produce for the given
	// domain and action.
	tests := []struct {
		name    string
		domain  string
		service string
		want    string
		config  Config
	}{
		{
			name:    "HTTPS default port",
			config:  Config{Host: "ha.example.com"},
			domain:  persistentDomain,
			service: persistentService,
			want:    "https://ha.example.com:443/api/services/persistent_notification/create",
		},
		{
			name:    "HTTP default port",
			config:  Config{Host: "homeassistant.local", DisableTLS: true},
			domain:  persistentDomain,
			service: persistentService,
			want:    "http://homeassistant.local:8123/api/services/persistent_notification/create",
		},
		{
			name:    "configured port and prefix",
			config:  Config{Host: "ha.example.com", Port: 8443, Path: "/hass"},
			domain:  "notify",
			service: "mobile_app_phone",
			want:    "https://ha.example.com:8443/hass/api/services/notify/mobile_app_phone",
		},
		{
			name:    "IPv6 host",
			config:  Config{Host: "::1", Port: 8123, DisableTLS: true},
			domain:  "notify",
			service: "notify",
			want:    "http://[::1]:8123/api/services/notify/notify",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.config.apiURL(tt.domain, tt.service))
		})
	}
}

// TestConfigQueryResolver verifies the config's query props: the TLS flags
// default to off, and the keys cover the notification and TLS props with no
// enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	t.Run("default values", func(t *testing.T) {
		t.Parallel()

		config := newDefaultConfig(t)

		assert.False(t, config.DisableTLS)
		assert.False(t, config.SkipTLSVerify)
	})

	t.Run("fields and enums", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Empty(t, config.Enums())
		assert.Equal(t,
			[]string{"disabletls", "nid", "service", "skiptlsverify", "targets", "title"},
			format.GetConfigQueryResolver(config).QueryFields())
	})
}
