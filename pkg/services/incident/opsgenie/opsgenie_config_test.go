package opsgenie

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Test credentials in the format the OpsGenie API documentation uses.
const (
	// testAPIKey is an API key.
	testAPIKey = "eb243592-faa2-4ba2-a551q-1afdf565c889"
	// testTeamID is a team ID.
	testTeamID = "4513b7ea-3b91-438f-b7e4-e3e54af9147c"
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

// TestConfigSetURL verifies that SetURL reads the host and port, the API key
// from the path, and the alert props from the query, and applies the default
// host when the URL omits it.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "host and API key",
			rawURL: "opsgenie://api.opsgenie.com/" + testAPIKey,
			want:   Config{Host: "api.opsgenie.com", Port: 443, APIKey: testAPIKey},
		},
		{
			name:   "omitted host",
			rawURL: "opsgenie:///" + testAPIKey,
			want:   Config{Host: "api.opsgenie.com", Port: 443, APIKey: testAPIKey},
		},
		{
			name:   "omitted host with a port",
			rawURL: "opsgenie://:8443/" + testAPIKey,
			want:   Config{Host: "api.opsgenie.com", Port: 8443, APIKey: testAPIKey},
		},
		{
			name:   "port",
			rawURL: "opsgenie://api.opsgenie.com:12345/" + testAPIKey,
			want:   Config{Host: "api.opsgenie.com", Port: 12345, APIKey: testAPIKey},
		},
		{
			name: "alert props",
			rawURL: "opsgenie://api.eu.opsgenie.com/" + testAPIKey +
				"?alias=Life+is+too+short+for+no+alias&description=Every+alert+needs+a+description&actions=An+action" +
				"&tags=tag1,tag2&details=key:value,key2:value2&entity=An+example+entity&source=The+source&priority=P1" +
				"&user=Dracula&note=Here+is+a+note&responders=user:Test,team:NOC&visibleTo=user:A+User&title=Title",
			want: Config{
				Host:        "api.eu.opsgenie.com",
				Port:        443,
				APIKey:      testAPIKey,
				Alias:       "Life is too short for no alias",
				Description: "Every alert needs a description",
				Responders:  []Entity{{Type: "user", Username: "Test"}, {Type: "team", Name: "NOC"}},
				VisibleTo:   []Entity{{Type: "user", Username: "A User"}},
				Actions:     []string{"An action"},
				Tags:        []string{"tag1", "tag2"},
				Details:     map[string]string{"key": "value", "key2": "value2"},
				Entity:      "An example entity",
				Source:      "The source",
				Priority:    "P1",
				Note:        "Here is a note",
				User:        "Dracula",
				Title:       "Title",
			},
		},
		{
			name:   "differently escaped spaces",
			rawURL: "opsgenie://api.opsgenie.com/" + testAPIKey + "?alias=Life is+too%20short+for+no+alias",
			want:   Config{Host: "api.opsgenie.com", Port: 443, APIKey: testAPIKey, Alias: "Life is too short for no alias"},
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL without an API
// key or with an empty one, port 0 or a port above 65535, an unknown query prop, an invalid responder, and a
// responder or visibleTo entry without an identifier.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should match, or
	// only an error when wantErr is nil.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "missing API key", rawURL: "opsgenie://api.opsgenie.com", wantErr: ErrAPIKeyMissing},
		{name: "empty API key", rawURL: "opsgenie://api.opsgenie.com/", wantErr: ErrAPIKeyMissing},
		{name: "port zero", rawURL: "opsgenie://api.opsgenie.com:0/" + testAPIKey, wantErr: ErrInvalidPort},
		{name: "unbracketed IPv6 host", rawURL: "opsgenie://2001:db8::1/" + testAPIKey, wantErr: util.ErrInvalidHost},
		{name: "port out of range", rawURL: "opsgenie://api.opsgenie.com:70000/" + testAPIKey, wantErr: ErrInvalidPort},
		{name: "unknown prop", rawURL: "opsgenie://api.opsgenie.com/" + testAPIKey + "?foo=bar", wantErr: format.ErrInvalidConfigKey},
		{
			name:    "responder without a type",
			rawURL:  "opsgenie://api.opsgenie.com/" + testAPIKey + "?responders=NOC",
			wantErr: ErrInvalidEntityFormat,
		},
		{
			name:    "responder without an identifier",
			rawURL:  "opsgenie://api.opsgenie.com/" + testAPIKey + "?responders=team:NOC,user:",
			wantErr: ErrMissingEntityIdentity,
		},
		{
			name:    "visibleTo entry without an identifier",
			rawURL:  "opsgenie://api.opsgenie.com/" + testAPIKey + "?visibleTo=team:",
			wantErr: ErrMissingEntityIdentity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := (&Config{}).SetURL(parseTestURL(t, tt.rawURL))

			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// TestConfigGetURL verifies that GetURL writes the API key as the path, the
// port when set, and the alert props as the query.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	// tests lists configs with the URL each should produce.
	tests := []struct {
		name   string
		want   string
		config Config
	}{
		{
			name:   "host and API key",
			config: Config{Host: "api.opsgenie.com", APIKey: testAPIKey},
			want:   "opsgenie://api.opsgenie.com/" + testAPIKey,
		},
		{
			name:   "port",
			config: Config{Host: "api.opsgenie.com", APIKey: testAPIKey, Port: 12345},
			want:   "opsgenie://api.opsgenie.com:12345/" + testAPIKey,
		},
		{
			name: "alert props",
			config: Config{
				Host:        "api.opsgenie.com",
				APIKey:      testAPIKey,
				Alias:       "Life is too short for no alias",
				Description: "Every alert needs a description",
				Responders: []Entity{
					{Type: "user", Username: "Test"},
					{Type: "team", Name: "NOC"},
					{Type: "team", ID: testTeamID},
				},
				VisibleTo: []Entity{{Type: "user", Username: "A User"}},
				Actions:   []string{"action1", "action2"},
				Tags:      []string{"tag1", "tag2"},
				Details:   map[string]string{"key": "value"},
				Entity:    "An example entity",
				Source:    "The source",
				Priority:  "P1",
				User:      "Dracula",
				Note:      "Here is a note",
			},
			want: "opsgenie://api.opsgenie.com/" + testAPIKey +
				"?actions=action1%2Caction2&alias=Life+is+too+short+for+no+alias&description=Every+alert+needs+a+description" +
				"&details=key%3Avalue&entity=An+example+entity&note=Here+is+a+note&priority=P1" +
				"&responders=user%3ATest%2Cteam%3ANOC%2Cteam%3A" + testTeamID +
				"&source=The+source&tags=tag1%2Ctag2&user=Dracula&visibleto=user%3AA+User",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.config.GetURL().String())
		})
	}

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()

		rawURL := "opsgenie://api.opsgenie.com:443/" + testAPIKey + "?priority=P2&responders=team%3ANOC&tags=a%2Cb"
		config := &Config{}
		require.NoError(t, config.SetURL(parseTestURL(t, rawURL)))

		assert.Equal(t, rawURL, config.GetURL().String())
	})
}

// TestConfigQueryResolver verifies the config's query props: an unknown key
// is rejected, and the keys cover every alert prop with no enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	t.Run("unknown key", func(t *testing.T) {
		t.Parallel()

		value, err := format.GetConfigQueryResolver(&Config{}).Get("invalid query var")

		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
		assert.Empty(t, value)
	})

	t.Run("fields and enums", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Empty(t, config.Enums())
		assert.Equal(t,
			[]string{
				"actions", "alias", "description", "details", "entity", "note", "priority",
				"responders", "source", "tags", "title", "user", "visibleto",
			},
			format.GetConfigQueryResolver(config).QueryFields())
	})
}
