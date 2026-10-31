package twilio

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// Test credentials and numbers in the formats Twilio uses.
const (
	// testAccountSID is an account SID.
	testAccountSID = "ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	// testMessagingServiceSID is a Messaging Service SID.
	testMessagingServiceSID = "MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	// testFrom is a sender number in E.164 format.
	testFrom = "+15551234567"
	// testTo is a recipient number in E.164 format.
	testTo = "+15559876543"
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

// TestConfigSetURL verifies that SetURL reads the account SID and auth token
// from the user info, the sender from the host, the recipients from the path,
// and the title from the query, normalizing phone number formatting.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "one recipient",
			rawURL: "twilio://" + testAccountSID + ":authToken@" + testFrom + "/" + testTo,
			want:   Config{AccountSID: testAccountSID, AuthToken: "authToken", FromNumber: testFrom, ToNumbers: []string{testTo}},
		},
		{
			name:   "several recipients and a title",
			rawURL: "twilio://" + testAccountSID + ":authToken@" + testFrom + "/" + testTo + "/+15551111111?title=Alert",
			want: Config{
				AccountSID: testAccountSID,
				AuthToken:  "authToken",
				FromNumber: testFrom,
				ToNumbers:  []string{testTo, "+15551111111"},
				Title:      "Alert",
			},
		},
		{
			name:   "formatted numbers and empty path segments",
			rawURL: "twilio://" + testAccountSID + ":authToken@" + "+1.555.123.4567" + "//+1(555)987-6543/",
			want:   Config{AccountSID: testAccountSID, AuthToken: "authToken", FromNumber: testFrom, ToNumbers: []string{testTo}},
		},
		{
			name:   "Messaging Service SID",
			rawURL: "twilio://" + testAccountSID + ":authToken@" + testMessagingServiceSID + "/" + testTo,
			want: Config{
				AccountSID: testAccountSID,
				AuthToken:  "authToken",
				FromNumber: testMessagingServiceSID,
				ToNumbers:  []string{testTo},
			},
		},
		{
			name:   "Messaging Service SID as a recipient is not checked against the sender",
			rawURL: "twilio://" + testAccountSID + ":authToken@" + testMessagingServiceSID + "/" + testMessagingServiceSID,
			want: Config{
				AccountSID: testAccountSID,
				AuthToken:  "authToken",
				FromNumber: testMessagingServiceSID,
				ToNumbers:  []string{testMessagingServiceSID},
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL missing any
// required part, a recipient equal to the sender number, and an unknown query
// prop.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "missing account SID", rawURL: "twilio://:authToken@" + testFrom + "/" + testTo, wantErr: ErrAccountSIDMissing},
		{name: "missing auth token", rawURL: "twilio://" + testAccountSID + "@" + testFrom + "/" + testTo, wantErr: ErrAuthTokenMissing},
		{name: "missing sender", rawURL: "twilio://" + testAccountSID + ":authToken@/" + testTo, wantErr: ErrFromNumberMissing},
		{name: "missing recipients", rawURL: "twilio://" + testAccountSID + ":authToken@" + testFrom, wantErr: ErrToNumbersMissing},
		{name: "recipient equals sender", rawURL: "twilio://" + testAccountSID + ":authToken@" + testFrom + "/" + testTo + "/" + testFrom, wantErr: ErrToFromNumberSame},
		{name: "unknown prop", rawURL: "twilio://" + testAccountSID + ":authToken@" + testFrom + "/" + testTo + "?foo=bar", wantErr: format.ErrInvalidConfigKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL writes the credentials as the user
// info, the sender as the host, and the recipients as the path, and
// round-trips through SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	config := &Config{
		AccountSID: testAccountSID,
		AuthToken:  "authToken",
		FromNumber: testFrom,
		ToNumbers:  []string{testTo, "+15551111111"},
		Title:      "Alert",
	}

	serviceURL := config.GetURL()
	assert.Equal(t, "twilio://"+testAccountSID+":authToken@"+testFrom+"/"+testTo+"/+15551111111?title=Alert", serviceURL.String())

	roundTrip := &Config{}
	require.NoError(t, roundTrip.SetURL(serviceURL))
	assert.Equal(t, config, roundTrip)
}

// TestNormalizePhoneNumber verifies that spaces, dashes, parentheses, and dots
// are removed from phone numbers, while Messaging Service SIDs are kept as is.
func TestNormalizePhoneNumber(t *testing.T) {
	t.Parallel()

	// tests lists inputs with their normalized form.
	tests := []struct {
		input string
		want  string
	}{
		{input: "+1 (555) 123-4567", want: testFrom},
		{input: "+1.555.123.4567", want: testFrom},
		{input: testFrom, want: testFrom},
		{input: testMessagingServiceSID, want: testMessagingServiceSID},
		{input: "MG-ABC.DEF", want: "MG-ABC.DEF"},
		{input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, normalizePhoneNumber(tt.input))
		})
	}
}

// TestParseToNumbers verifies that the path splits into normalized numbers,
// skipping empty and blank segments.
func TestParseToNumbers(t *testing.T) {
	t.Parallel()

	// tests lists paths with the numbers each should produce.
	tests := []struct {
		name string
		path string
		want []string
	}{
		{name: "empty", path: "", want: nil},
		{name: "root only", path: "/", want: nil},
		{name: "one number", path: "/" + testTo, want: []string{testTo}},
		{name: "several numbers", path: "/" + testTo + "/+15551111111", want: []string{testTo, "+15551111111"}},
		{name: "empty and blank segments", path: "//" + testTo + "/ /", want: []string{testTo}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, parseToNumbers(tt.path))
		})
	}
}

// TestConfigQueryResolver verifies the config's query props: the title can be
// set and read, an unknown key is rejected, and title is the only key with no
// enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	config := &Config{}
	resolver := format.NewPropKeyResolver(config)

	require.NoError(t, resolver.Set("title", "new title"))
	assert.Equal(t, "new title", config.Title)

	value, err := resolver.Get("title")
	require.NoError(t, err)
	assert.Equal(t, "new title", value)

	require.ErrorIs(t, resolver.Set("foo", "bar"), format.ErrInvalidConfigKey)

	_, err = resolver.Get("foo")
	require.ErrorIs(t, err, format.ErrInvalidConfigKey)

	assert.Empty(t, config.Enums())
	assert.Equal(t, []string{"title"}, resolver.QueryFields())
}
