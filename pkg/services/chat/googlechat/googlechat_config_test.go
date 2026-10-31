package googlechat

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigSetURL verifies that SetURL reads the host, path, key, and token
// from a service URL, applies the default host when the URL omits it, and
// requires both the key and the token.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config or error SetURL produces.
	tests := []struct {
		name    string
		rawURL  string
		want    Config
		wantErr error
	}{
		{
			name:   "key and token",
			rawURL: "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=baz",
			want:   Config{Host: "chat.googleapis.com", Path: "/v1/spaces/FOO/messages", Key: "bar", Token: "baz"},
		},
		{
			name:   "omitted host",
			rawURL: "googlechat:///v1/spaces/FOO/messages?key=bar&token=baz",
			want:   Config{Host: "chat.googleapis.com", Path: "/v1/spaces/FOO/messages", Key: "bar", Token: "baz"},
		},
		{
			name:    "no query",
			rawURL:  "googlechat://chat.googleapis.com/v1/spaces/FOO/messages",
			wantErr: ErrMissingKey,
		},
		{
			name:    "missing key",
			rawURL:  "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?token=baz",
			wantErr: ErrMissingKey,
		},
		{
			name:    "missing token",
			rawURL:  "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=bar",
			wantErr: ErrMissingToken,
		},
		{
			name:    "bare scheme",
			rawURL:  "googlechat:",
			wantErr: ErrMissingKey,
		},
		{
			name:    "empty key",
			rawURL:  "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=&token=baz",
			wantErr: ErrMissingKey,
		},
		{
			name:    "empty token",
			rawURL:  "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=",
			wantErr: ErrMissingToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, err := url.Parse(tt.rawURL)
			require.NoError(t, err)

			config := &Config{}
			err = config.SetURL(serviceURL)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, *config)
		})
	}
}

// TestConfigGetURL verifies that GetURL rebuilds the service URL that SetURL
// read, so a config survives a round trip through its URL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	const rawURL = "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=baz"

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	config := &Config{}
	require.NoError(t, config.SetURL(serviceURL))

	assert.Equal(t, rawURL, config.GetURL().String())
}

// TestConfigGetURLOmittedHost verifies that a config read from a URL without a
// host writes the default host, and that SetURL restores the same config.
func TestConfigGetURLOmittedHost(t *testing.T) {
	t.Parallel()

	serviceURL, err := url.Parse("googlechat:///v1/spaces/FOO/messages?key=bar&token=baz")
	require.NoError(t, err)

	config := &Config{}
	require.NoError(t, config.SetURL(serviceURL))

	got := config.GetURL()
	assert.Equal(t, "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=baz", got.String())

	restored := &Config{}
	require.NoError(t, restored.SetURL(got))
	assert.Equal(t, *config, *restored)
}
