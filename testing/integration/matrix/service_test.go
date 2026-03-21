package matrix_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

func TestServiceInitializeWithValidURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceURL string
	}{
		{
			name:       "dummy URL for testing",
			serviceURL: "matrix://dummy@dummy.com",
		},
		{
			name:       "dummy URL with password",
			serviceURL: "matrix://dummy:pass@dummy.com",
		},
		{
			name:       "dummy URL with custom title",
			serviceURL: "matrix://dummy@dummy.com?title=Notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Use createTestService which handles URL parsing internally
			service, err := createTestService(t, tt.serviceURL)
			require.NoError(t, err, "Expected no error for %s", tt.name)
			require.NotNil(t, service, "Service should be initialized for %s", tt.name)
			require.NotNil(t, service.Config, "Config should be initialized for %s", tt.name)
		})
	}
}

func TestServiceInitializeWithInvalidURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceURL string
		wantErr    error
	}{
		{
			name:       "missing host",
			serviceURL: "matrix://user:token@",
			wantErr:    matrix.ErrMissingHost,
		},
		{
			name:       "missing credentials",
			serviceURL: "matrix://matrix.example.com",
			wantErr:    matrix.ErrMissingCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &matrix.Service{}

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			err = service.Initialize(parsedURL, &testLogger{})

			require.Error(t, err, "Expected error for %s", tt.name)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr,
					"Expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestServiceGetID(t *testing.T) {
	t.Parallel()

	service, _ := createTestService(t, "matrix://dummy@dummy.com")

	id := service.GetID()

	require.Equal(t, "matrix", id, "Expected service ID to be 'matrix'")
}

func TestServiceSendWithDummyClient(t *testing.T) {
	t.Parallel()

	// When using the dummy URL, the client is not initialized
	// and Send should return ErrClientNotInitialized
	service, _ := createTestService(t, "matrix://dummy@dummy.com")

	err := service.Send("Test message", nil)

	require.Error(t, err, "Expected error when sending with uninitialized client")
	require.ErrorIs(t, err, matrix.ErrClientNotInitialized,
		"Expected ErrClientNotInitialized, got %v", err)
}

func TestServiceSendWithNilParams(t *testing.T) {
	t.Parallel()

	// Using dummy URL - client won't be initialized
	service, _ := createTestService(t, "matrix://dummy@dummy.com")

	// This will fail because the client is nil, but we can verify the behavior
	err := service.Send("Test message", nil)

	require.Error(t, err)
	require.ErrorIs(t, err, matrix.ErrClientNotInitialized)
}

func TestServiceSendWithEmptyMessage(t *testing.T) {
	t.Parallel()

	service, _ := createTestService(t, "matrix://dummy@dummy.com")

	err := service.Send("", nil)

	// Even empty messages will fail with client not initialized
	require.Error(t, err)
}

func TestServiceSendWithValidConfig(t *testing.T) {
	t.Parallel()

	// This test verifies that the service can be created with valid config
	// and the config values are correctly set
	service, err := createTestService(t, "matrix://dummy:token@dummy.com")
	require.NoError(t, err, "Expected no error for dummy URL")
	require.NotNil(t, service, "Service should not be nil")

	// Verify the config is set correctly
	require.Equal(t, "dummy.com", service.Config.Host)
	require.Equal(t, "token", service.Config.Password)
}
