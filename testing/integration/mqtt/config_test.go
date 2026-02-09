package mqtt_test

import (
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
)

func TestConfigValidURLParsing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name         string
			url          string
			expectedHost string
			expectedPort int
		}{
			{
				name:         "mqtt scheme with custom port",
				url:          "mqtt://broker.example.com:1884/alerts",
				expectedHost: "broker.example.com",
				expectedPort: 1884,
			},
			{
				name:         "mqtts scheme with custom port",
				url:          "mqtts://broker.example.com:8884/alerts",
				expectedHost: "broker.example.com",
				expectedPort: 8884,
			},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()

			service := createTestService(t, tt.url, mockManager)

			// Verify config was parsed correctly
			require.Equal(t, tt.expectedHost, service.Config.Host, "host mismatch for %s", tt.name)
			require.Equal(t, tt.expectedPort, service.Config.Port, "port mismatch for %s", tt.name)

			// Send a message to trigger connection
			err := service.Send("test message", nil)
			require.NoError(t, err)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestConfigQueryParameters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name                string
			url                 string
			expectedQoS         byte
			expectedRetained    bool
			expectedClientID    string
			expectedCleanSession bool
		}{
			{
				name:                "qos 0",
				url:                 "mqtt://broker.example.com/alerts?qos=0",
				expectedQoS:         0,
				expectedRetained:    false,
				expectedClientID:    "shoutrrr",
				expectedCleanSession: true,
			},
			{
				name:                "qos 1",
				url:                 "mqtt://broker.example.com/alerts?qos=1",
				expectedQoS:         1,
				expectedRetained:    false,
				expectedClientID:    "shoutrrr",
				expectedCleanSession: true,
			},
			{
				name:                "qos 2",
				url:                 "mqtt://broker.example.com/alerts?qos=2",
				expectedQoS:         2,
				expectedRetained:    false,
				expectedClientID:    "shoutrrr",
				expectedCleanSession: true,
			},
			{
				name:                "retained yes",
				url:                 "mqtt://broker.example.com/alerts?retained=yes",
				expectedQoS:         0,
				expectedRetained:    true,
				expectedClientID:    "shoutrrr",
				expectedCleanSession: true,
			},
			{
				name:                "retained true",
				url:                 "mqtt://broker.example.com/alerts?retained=true",
				expectedQoS:         0,
				expectedRetained:    true,
				expectedClientID:    "shoutrrr",
				expectedCleanSession: true,
			},
			{
				name:                "custom client id",
				url:                 "mqtt://broker.example.com/alerts?clientid=custom-client",
				expectedQoS:         0,
				expectedRetained:    false,
				expectedClientID:    "custom-client",
				expectedCleanSession: true,
			},
			{
				name:                "clean session no",
				url:                 "mqtt://broker.example.com/alerts?cleansession=no",
				expectedQoS:         0,
				expectedRetained:    false,
				expectedClientID:    "shoutrrr",
				expectedCleanSession: false,
			},
			{
				name:                "all options combined",
				url:                 "mqtt://broker.example.com/alerts?qos=1&retained=yes&clientid=myclient&cleansession=no",
				expectedQoS:         1,
				expectedRetained:    true,
				expectedClientID:    "myclient",
				expectedCleanSession: false,
			},
		}

		for _, tt := range tests {
			mockManager := &MockConnectionManager{}
			mockManager.On("AwaitConnection", mock.Anything).Return(nil)
			mockManager.On("Publish", mock.Anything, mock.Anything).
				Return(createMockPublishResponse(0), nil).
				Once()

			service := createTestService(t, tt.url, mockManager)

			// Verify config was parsed correctly
			require.Equal(t, tt.expectedQoS, byte(service.Config.QoS))
			require.Equal(t, tt.expectedRetained, service.Config.Retained)
			require.Equal(t, tt.expectedClientID, service.Config.ClientID)
			require.Equal(t, tt.expectedCleanSession, service.Config.CleanSession)

			// Send a message to verify the config is used
			err := service.Send("test message", nil)
			require.NoError(t, err)

			// Verify the publish was called with correct parameters
			publish := getPublishCall(mockManager)
			require.NotNil(t, publish)
			require.Equal(t, tt.expectedQoS, publish.QoS)
			require.Equal(t, tt.expectedRetained, publish.Retain)

			mockManager.AssertExpectations(t)
		}
	})
}

func TestConfigUsernamePassword(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name             string
			url              string
			expectedUsername string
			expectedPassword string
		}{
			{
				name:             "username only",
				url:              "mqtt://user@broker.example.com/alerts",
				expectedUsername: "user",
				expectedPassword: "",
			},
			{
				name:             "username and password",
				url:              "mqtt://user:pass@broker.example.com/alerts",
				expectedUsername: "user",
				expectedPassword: "pass",
			},
			{
				name:             "username with special characters",
				url:              "mqtt://user%40domain:pass%40word@broker.example.com/alerts",
				expectedUsername: "user@domain",
				expectedPassword: "pass@word",
			},
			{
				name:             "no credentials",
				url:              "mqtt://broker.example.com/alerts",
				expectedUsername: "",
				expectedPassword: "",
			},
		}

		for _, tt := range tests {
			service := createTestService(t, tt.url)

			// Verify credentials were parsed correctly
			require.Equal(t, tt.expectedUsername, service.Config.Username)
			require.Equal(t, tt.expectedPassword, service.Config.Password)
		}
	})
}

func TestConfigTLSOptions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name                       string
			url                        string
			expectedDisableTLS         bool
			expectedDisableTLSVerify   bool
		}{
			{
				name:                     "tls enabled by default with mqtts",
				url:                      "mqtts://broker.example.com/alerts",
				expectedDisableTLS:       false,
				expectedDisableTLSVerify: false,
			},
			{
				name:                     "tls disabled explicitly",
				url:                      "mqtts://broker.example.com/alerts?disabletls=yes",
				expectedDisableTLS:       true,
				expectedDisableTLSVerify: false,
			},
			{
				name:                     "tls verification disabled",
				url:                      "mqtts://broker.example.com/alerts?disabletlsverification=yes",
				expectedDisableTLS:       false,
				expectedDisableTLSVerify: true,
			},
			{
				name:                     "both tls options",
				url:                      "mqtts://broker.example.com/alerts?disabletls=yes&disabletlsverification=yes",
				expectedDisableTLS:       true,
				expectedDisableTLSVerify: true,
			},
		}

		for _, tt := range tests {
			service := createTestService(t, tt.url)

			// Verify TLS options were parsed correctly
			require.Equal(t, tt.expectedDisableTLS, service.Config.DisableTLS)
			require.Equal(t, tt.expectedDisableTLSVerify, service.Config.DisableTLSVerification)
		}
	})
}

func TestConfigInvalidURLs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name        string
			url         string
			expectError bool
		}{
			{
				name:        "missing topic",
				url:         "mqtt://broker.example.com",
				expectError: true,
			},
			{
				name:        "missing topic with trailing slash",
				url:         "mqtt://broker.example.com/",
				expectError: true,
			},
			{
				name:        "empty topic",
				url:         "mqtt://broker.example.com/",
				expectError: true,
			},
		}

		for _, tt := range tests {
			service := &mqtt.Service{}
			parsedURL, err := parseURLHelper(tt.url)
			if err != nil {
				continue // URL parsing failed, which is acceptable
			}

			err = service.Initialize(parsedURL, &mockLogger{})
			if tt.expectError {
				require.Error(t, err)
			}
		}
	})
}

func TestConfigInvalidQoS(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name string
			url  string
		}{
			{
				name: "qos too high",
				url:  "mqtt://broker.example.com/alerts?qos=3",
			},
			{
				name: "qos invalid string",
				url:  "mqtt://broker.example.com/alerts?qos=invalid",
			},
		}

		for _, tt := range tests {
			// Create service directly without mock - initialization should fail
			service := &mqtt.Service{}
			parsedURL, err := url.Parse(tt.url)
			require.NoError(t, err)

			// Initialize should fail due to invalid QoS
			err = service.Initialize(parsedURL, &mockLogger{})
			require.Error(t, err, "expected initialization error for %s", tt.name)
			require.Contains(t, err.Error(), "qos")
		}
	})
}

// parseURLHelper is a helper to parse URL strings, returning any parsing errors.
func parseURLHelper(urlStr string) (*url.URL, error) {
	return url.Parse(urlStr)
}
