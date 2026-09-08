package homeassistant_test

import (
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/homeassistant"
)

func TestServiceURLRoundTrip(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := createTestService(t, validHomeAssistantURL)

		serviceURL := service.Config.GetURL()
		assert.Equal(t, "homeassistant", serviceURL.Scheme)
		assert.Equal(t, "s3cret", serviceURL.User.Username())
		assert.Equal(t, "ha.example.com", serviceURL.Hostname())
	})
}

func TestConfigQueryParameters(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := createTestService(
			t,
			validHomeAssistantURL+"?title=Alert&nid=watchtower&disabletls=yes",
		)

		assert.Equal(t, "Alert", service.Config.Title)
		assert.Equal(t, "watchtower", service.Config.Nid)
		assert.True(t, service.Config.DisableTLS)
	})
}

func TestInitializeRejectsMissingFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{"missing token", "homeassistant://ha.example.com"},
		{"missing host", "homeassistant://s3cret@"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.raw)
			require.NoError(t, err)

			service := &homeassistant.Service{}
			err = service.Initialize(parsedURL, &mockLogger{})
			require.Error(t, err)
		})
	}
}
