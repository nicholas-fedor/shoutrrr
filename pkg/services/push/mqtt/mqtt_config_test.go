package mqtt

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

// TestConfigGetURL verifies that GetURL encodes every field in the URL, omits
// absent credentials and settings at their defaults, and keeps a password
// without a username so that SetURL restores it.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		resolver := format.NewPropKeyResolver(config)
		require.NoError(t, resolver.SetDefaultProps(config))

		config.Host, config.Port, config.Topic = "broker.example.com", 1883, "notifications/alerts"

		assert.Equal(t, "mqtt://broker.example.com:1883/notifications/alerts?", config.GetURL().String())
	})

	t.Run("all fields", func(t *testing.T) {
		t.Parallel()

		config := &Config{
			Host:          "broker.example.com",
			Port:          1883,
			Topic:         "notifications/alerts",
			Username:      "testuser",
			Password:      "testpass",
			ClientID:      "shoutrrr-client",
			QoS:           QoSAtLeastOnce,
			Retained:      true,
			CleanSession:  false,
			DisableTLS:    true,
			SkipTLSVerify: true,
		}

		result := config.GetURL()
		require.NotNil(t, result)

		assert.Equal(t, "mqtt", result.Scheme)
		assert.Equal(t, "broker.example.com", result.Hostname())
		assert.Equal(t, "1883", result.Port())
		assert.Equal(t, "/notifications/alerts", result.Path)
		assert.Equal(t, "testuser", result.User.Username())

		password, hasPassword := result.User.Password()
		assert.True(t, hasPassword)
		assert.Equal(t, "testpass", password)

		assert.Equal(t, url.Values{
			"clientid":      {"shoutrrr-client"},
			"qos":           {"AtLeastOnce"},
			"retained":      {"Yes"},
			"cleansession":  {"No"},
			"disabletls":    {"Yes"},
			"skiptlsverify": {"Yes"},
		}, result.Query())
	})

	t.Run("deprecated verification field", func(t *testing.T) {
		t.Parallel()

		config := &Config{
			Host: "broker.example.com", Port: 8883, Topic: "t", ClientID: "shoutrrr", CleanSession: true, DisableTLSVerification: true,
		}

		assert.Equal(t, url.Values{"skiptlsverify": {"Yes"}}, config.GetURL().Query())
		assert.False(t, config.SkipTLSVerify, "GetURL must not change the config")
	})

	t.Run("no credentials", func(t *testing.T) {
		t.Parallel()

		result := (&Config{Host: "broker.example.com", Port: 1883, Topic: "notifications/alerts", ClientID: "shoutrrr"}).GetURL()
		require.NotNil(t, result)
		assert.Nil(t, result.User)
	})

	t.Run("username only", func(t *testing.T) {
		t.Parallel()

		result := (&Config{Host: "broker.example.com", Port: 1883, Topic: "t", Username: "testuser", ClientID: "shoutrrr"}).GetURL()
		require.NotNil(t, result)
		assert.Equal(t, "testuser", result.User.Username())

		_, hasPassword := result.User.Password()
		assert.False(t, hasPassword)
	})

	t.Run("password without username", func(t *testing.T) {
		t.Parallel()

		config := &Config{Host: "broker.example.com", Port: 1883, Topic: "t", Password: "secret", ClientID: "shoutrrr"}
		result := config.GetURL()
		require.NotNil(t, result)
		assert.Empty(t, result.User.Username())

		password, hasPassword := result.User.Password()
		assert.True(t, hasPassword)
		assert.Equal(t, "secret", password)

		restored := &Config{}
		require.NoError(t, restored.SetURL(result))
		assert.Empty(t, restored.Username)
		assert.Equal(t, "secret", restored.Password)
	})

	t.Run("exactly once QoS", func(t *testing.T) {
		t.Parallel()

		result := (&Config{Host: "broker.example.com", Port: 1883, Topic: "t", ClientID: "shoutrrr", QoS: QoSExactlyOnce}).GetURL()
		require.NotNil(t, result)
		assert.Equal(t, "ExactlyOnce", result.Query().Get("qos"))
	})
}

// TestConfigValidateCredentials verifies that only a password without a
// username is rejected.
func TestConfigValidateCredentials(t *testing.T) {
	t.Parallel()

	// tests lists credential combinations with the error they should produce.
	tests := []struct {
		name     string
		username string
		password string
		wantErr  error
	}{
		{name: "password without username", password: "secret", wantErr: ErrPasswordWithoutUsername},
		{name: "username and password", username: "testuser", password: "secret"},
		{name: "username only", username: "testuser"},
		{name: "no credentials"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&Config{Username: tt.username, Password: tt.password}).ValidateCredentials()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

// TestConfigSetURL verifies that SetURL reads the host, port, topic,
// credentials, and query flags, applies the scheme's default port, and accepts
// QoS as a number or a name.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists URLs with the config SetURL should produce. The unexported
	// scheme is checked separately through GetURL.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "every component",
			rawURL: "mqtt://testuser:testpass@broker.example.com:1883/notifications/alerts?clientid=myclient&qos=1&retained=yes&cleansession=no&disabletls=yes&skiptlsverify=yes",
			want: Config{
				Host: "broker.example.com", Port: 1883, Topic: "notifications/alerts",
				Username: "testuser", Password: "testpass", ClientID: "myclient", QoS: QoSAtLeastOnce,
				Retained: true, CleanSession: false, DisableTLS: true, SkipTLSVerify: true,
			},
		},
		{
			name:   "deprecated verification key",
			rawURL: "mqtts://broker.example.com:8883/t?disabletlsverification=yes",
			want:   Config{Host: "broker.example.com", Port: 8883, Topic: "t", SkipTLSVerify: true},
		},
		{
			name:   "omitted host",
			rawURL: "mqtt:///notifications/alerts",
			want:   Config{Host: "localhost", Port: 1883, Topic: "notifications/alerts"},
		},
		{
			name:   "omitted host with a port",
			rawURL: "mqtts://:8884/notifications/alerts",
			want:   Config{Host: "localhost", Port: 8884, Topic: "notifications/alerts"},
		},
		{
			name:   "no credentials",
			rawURL: "mqtt://broker.example.com:1883/notifications/alerts",
			want:   Config{Host: "broker.example.com", Port: 1883, Topic: "notifications/alerts"},
		},
		{
			name:   "username only",
			rawURL: "mqtt://testuser@broker.example.com:1883/test/topic",
			want:   Config{Host: "broker.example.com", Port: 1883, Topic: "test/topic", Username: "testuser"},
		},
		{
			name:   "nested topic",
			rawURL: "mqtt://broker.example.com:1883/level1/level2/level3",
			want:   Config{Host: "broker.example.com", Port: 1883, Topic: "level1/level2/level3"},
		},
		{
			name:   "escaped topic",
			rawURL: "mqtt://broker.example.com:1883/sensor%20device/temp%C3%A9rature",
			want:   Config{Host: "broker.example.com", Port: 1883, Topic: "sensor device/température"},
		},
		{
			name:   "default mqtts port",
			rawURL: "mqtts://broker.example.com/test/topic",
			want:   Config{Host: "broker.example.com", Port: 8883, Topic: "test/topic"},
		},
		{
			name:   "QoS by number",
			rawURL: "mqtt://broker.example.com:1883/test/topic?qos=2",
			want:   Config{Host: "broker.example.com", Port: 1883, Topic: "test/topic", QoS: QoSExactlyOnce},
		},
		{
			name:   "QoS by name",
			rawURL: "mqtt://broker.example.com:1883/test/topic?qos=AtLeastOnce",
			want:   Config{Host: "broker.example.com", Port: 1883, Topic: "test/topic", QoS: QoSAtLeastOnce},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			got := *config
			got.scheme = ""
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestConfigSetURLRejectsInvalidValues verifies that SetURL requires a topic,
// rejects a topic with a wildcard or a null character, which MQTT forbids in a
// topic name, rejects a URL whose unescaped # starts a fragment, and rejects
// QoS values outside the enum.
func TestConfigSetURLRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	t.Run("empty topic", func(t *testing.T) {
		t.Parallel()

		require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, "mqtt://broker.example.com:1883/")), ErrTopicRequired)
	})

	for _, path := range []string{"/+", "/sensors/+/temperature", "/%23", "/home/%23", "/sensor%00/temperature"} {
		t.Run("topic "+path, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, "mqtt://broker.example.com:1883"+path)), ErrInvalidTopic)
		})
	}

	// An unescaped # starts the URL fragment, which would cut the intended
	// topic short.
	for _, rawURL := range []string{
		"mqtt://broker.example.com:1883/sensors/#/temperature",
		"mqtt://broker.example.com:1883/home/alerts#urgent",
		"mqtt://broker.example.com:1883/#sensors",
	} {
		t.Run("fragment in "+rawURL, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, rawURL)), ErrInvalidTopic)
		})
	}

	t.Run("unbracketed IPv6 host", func(t *testing.T) {
		t.Parallel()

		require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, "mqtt://2001:db8::1/test/topic")), util.ErrInvalidHost)
	})

	t.Run("invalid QoS", func(t *testing.T) {
		t.Parallel()

		err := (&Config{}).SetURL(parseTestURL(t, "mqtt://broker.example.com:1883/test/topic?qos=5"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a valid enum value")
	})
}

// TestConfigKeepsTLSScheme verifies that a config parsed from an mqtts URL
// builds an mqtts URL again.
func TestConfigKeepsTLSScheme(t *testing.T) {
	t.Parallel()

	config := &Config{}
	require.NoError(t, config.SetURL(parseTestURL(t, "mqtts://broker.example.com/test/topic")))

	assert.Equal(t, SchemeTLS, config.GetURL().Scheme)
}

// TestConfigURLRoundTripKeepsTopic verifies that a config restored from its own
// URL keeps the topic exactly, since MQTT treats a leading or trailing slash
// as part of a distinct topic and accepts a topic of only a slash.
func TestConfigURLRoundTripKeepsTopic(t *testing.T) {
	t.Parallel()

	// tests lists topics that must survive GetURL and SetURL unchanged.
	tests := []struct {
		name  string
		topic string
	}{
		{name: "plain", topic: "home/alerts"},
		{name: "leading slash", topic: "/home/alerts"},
		{name: "trailing slash", topic: "home/alerts/"},
		{name: "only a slash", topic: "/"},
		{name: "empty levels", topic: "home//alerts"},
		{name: "characters to escape", topic: "home alerts/100%/a?b&c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{Host: "broker.example.com", Port: DefaultPort, Topic: tt.topic, ClientID: "shoutrrr"}

			restored := &Config{}
			require.NoError(t, restored.SetURL(config.GetURL()))
			assert.Equal(t, tt.topic, restored.Topic)
		})
	}
}

// TestConfigQueryFieldsAndEnums verifies the query parameters the config
// exposes and its single QoS enum.
func TestConfigQueryFieldsAndEnums(t *testing.T) {
	t.Parallel()

	config := &Config{}

	assert.Subset(t, config.QueryFields(), []string{"clientid", "qos", "retained", "cleansession", "disabletls", "disabletlsverification", "skiptlsverify"})

	enums := config.Enums()
	assert.Len(t, enums, 1)
	assert.Contains(t, enums, "QoS")
}

// TestQoS verifies the QoS names, which values are valid, and the enum
// formatter's values.
func TestQoS(t *testing.T) {
	t.Parallel()

	// tests lists QoS values with their name and validity. An empty name means
	// only a non-empty string is expected.
	tests := []struct {
		qos       QoS
		wantName  string
		wantValid bool
	}{
		{qos: QoSAtMostOnce, wantName: "AtMostOnce", wantValid: true},
		{qos: QoSAtLeastOnce, wantName: "AtLeastOnce", wantValid: true},
		{qos: QoSExactlyOnce, wantName: "ExactlyOnce", wantValid: true},
		{qos: QoS(3)},
		{qos: QoS(100)},
		{qos: QoS(255)},
		{qos: QoS(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.qos.String(), func(t *testing.T) {
			t.Parallel()

			if tt.wantName != "" {
				assert.Equal(t, tt.wantName, tt.qos.String())
			} else {
				assert.NotEmpty(t, tt.qos.String())
			}

			assert.Equal(t, tt.wantValid, tt.qos.IsValid())
		})
	}

	assert.Equal(t, QoSAtMostOnce, QoSValues.AtMostOnce)
	assert.Equal(t, QoSAtLeastOnce, QoSValues.AtLeastOnce)
	assert.Equal(t, QoSExactlyOnce, QoSValues.ExactlyOnce)
	assert.NotNil(t, QoSValues.Enum)
}
