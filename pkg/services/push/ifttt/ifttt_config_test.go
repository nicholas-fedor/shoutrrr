package ifttt

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

// TestConfigSetURL verifies that SetURL reads the webhook ID from the host and
// the events, values, and value field choices from the query, defaulting the
// message to the second value.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "one event",
			rawURL: "ifttt://dummyID/?events=event1",
			want:   Config{WebHookID: "dummyID", Events: []string{"event1"}, UseMessageAsValue: DefaultMessageValue},
		},
		{
			name:   "events, values, and value fields",
			rawURL: "ifttt://dummyID/?events=foo,bar&value1=v1&value2=v2&value3=v3&messagevalue=1&titlevalue=3&title=Title",
			want: Config{
				WebHookID:         "dummyID",
				Events:            []string{"foo", "bar"},
				Value1:            "v1",
				Value2:            "v2",
				Value3:            "v3",
				UseMessageAsValue: 1,
				UseTitleAsValue:   3,
				Title:             "Title",
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL without events or
// a webhook ID, a webhook ID with a port or brackets, an unknown or invalid
// query prop, and value field choices outside 1 to 3 or in conflict.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "no arguments", rawURL: "ifttt://", wantErr: ErrMissingEvents},
		{name: "missing webhook ID", rawURL: "ifttt:///?events=event1", wantErr: ErrMissingWebhookID},
		{name: "webhook ID with a port", rawURL: "ifttt://SECRETid:8443/?events=event1", wantErr: util.ErrInvalidHost},
		{name: "webhook ID in brackets", rawURL: "ifttt://[::1]/?events=event1", wantErr: util.ErrInvalidHost},
		{name: "missing events", rawURL: "ifttt://dummyID", wantErr: ErrMissingEvents},
		{name: "empty events", rawURL: "ifttt://dummyID/?events=", wantErr: ErrMissingEvents},
		{name: "unknown prop", rawURL: "ifttt://dummyID/?events=event1&badquery=foo", wantErr: format.ErrInvalidConfigKey},
		{name: "messagevalue not a number", rawURL: "ifttt://dummyID/?events=event1&messagevalue=abc", wantErr: format.ErrParseUintFailed},
		{name: "messagevalue above 3", rawURL: "ifttt://dummyID/?events=event1&messagevalue=8", wantErr: ErrInvalidMessageValue},
		{name: "messagevalue below 1", rawURL: "ifttt://dummyID/?events=event1&messagevalue=0", wantErr: ErrInvalidMessageValue},
		{name: "titlevalue above 3", rawURL: "ifttt://dummyID/?events=event1&titlevalue=4", wantErr: ErrInvalidTitleValue},
		{
			name:    "titlevalue equals messagevalue",
			rawURL:  "ifttt://dummyID/?events=event1&messagevalue=2&titlevalue=2",
			wantErr: ErrTitleMessageConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL serializes a config back to the
// service URL that SetURL read.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs that should survive a round trip unchanged.
	tests := []struct {
		name   string
		rawURL string
	}{
		{name: "events joined by commas", rawURL: "ifttt://dummyID/?events=foo%2Cbar%2Cbaz"},
		{name: "values", rawURL: "ifttt://dummyID/?events=event1&value1=v1&value2=v2&value3=v3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, tt.rawURL)))

			assert.Equal(t, tt.rawURL, config.GetURL().String())
		})
	}
}

// TestConfigQueryResolver verifies the config's query props: an unknown key
// is rejected, the defaults apply cleanly, and there are eight keys and no
// enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	t.Run("unknown key", func(t *testing.T) {
		t.Parallel()

		value, err := format.GetConfigQueryResolver(&Config{}).Get("invalid query var")

		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
		assert.Empty(t, value)
	})

	t.Run("default values", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		resolver := format.NewPropKeyResolver(config)

		require.NoError(t, resolver.SetDefaultProps(config))
		assert.Equal(t, uint8(DefaultMessageValue), config.UseMessageAsValue)
		assert.Equal(t, uint8(DisabledValue), config.UseTitleAsValue)
		assert.Empty(t, config.Title)
	})

	t.Run("fields and enums", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Empty(t, config.Enums())
		assert.Equal(t,
			[]string{"events", "messagevalue", "title", "titlevalue", "value1", "value2", "value3"},
			format.GetConfigQueryResolver(config).QueryFields())
	})
}
