package signalgrid_test

import (
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/signalgrid"
)

func TestServiceURLRoundTrip(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := createTestService(t, validSignalgridURL)

		serviceURL := service.Config.GetURL()
		assert.Equal(t, "signalgrid", serviceURL.Scheme)
		assert.Equal(t, "clientkey", serviceURL.User.Username())
		assert.Equal(t, "channeltoken", serviceURL.Host)
	})
}

func TestConfigMixedCaseChannel(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := createTestService(t, "signalgrid://abc123@channel")

		assert.Equal(t, "abc123", service.Config.ClientKey)
		assert.Equal(t, "channel", service.Config.Channel)
		assert.Equal(t, "channel", service.Config.GetURL().Host)
	})
}

func TestConfigQueryParameters(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := createTestService(
			t,
			validSignalgridURL+"?title=Alert&type=CRIT&critical=true",
		)

		assert.Equal(t, "Alert", service.Config.Title)
		assert.True(t, service.Config.Critical)
		assert.Equal(t, "CRIT", service.Config.Type.String())
	})
}

func TestInitializeRejectsMissingFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{"missing client key", "signalgrid://channel"},
		{"missing channel", "signalgrid://key@"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.raw)
			require.NoError(t, err)

			service := &signalgrid.Service{}
			err = service.Initialize(parsedURL, &mockLogger{})
			require.Error(t, err)
		})
	}
}
