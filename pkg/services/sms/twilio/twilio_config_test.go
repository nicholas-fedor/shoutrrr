package twilio

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Test credentials and numbers in the formats Twilio uses.
const (
	// testAccountSID is an account SID.
	testAccountSID = "AC0123456789abcdef0123456789ABCDEF"
	// testMessagingServiceSID is a Messaging Service SID.
	testMessagingServiceSID = "MG0123456789abcdef0123456789ABCDEF"
	// testAPIKeySID is an API key SID.
	testAPIKeySID = "SK0123456789abcdef0123456789ABCDEF"
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
			name:   "API key",
			rawURL: "twilio://" + testAccountSID + ":keySecret@" + testFrom + "/" + testTo + "?apikey=" + testAPIKeySID,
			want: Config{
				AccountSID: testAccountSID,
				AuthToken:  "keySecret",
				APIKey:     testAPIKeySID,
				FromNumber: testFrom,
				ToNumbers:  []string{testTo},
			},
		},
		{
			name:   "escaped plus sign and slash",
			rawURL: "twilio://" + testAccountSID + ":authToken@" + testFrom + "/%2B15551111111/%2B1555%2F2222222",
			want:   Config{AccountSID: testAccountSID, AuthToken: "authToken", FromNumber: testFrom, ToNumbers: []string{"+15551111111", "+1555/2222222"}},
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
		{name: "account SID that is not an account SID", rawURL: "twilio://bob:authToken@" + testFrom + "/" + testTo, wantErr: ErrInvalidAccountSID},
		{name: "account SID with a question mark", rawURL: "twilio://AC%3F" + testAccountSID[3:] + ":authToken@" + testFrom + "/" + testTo, wantErr: ErrInvalidAccountSID},
		{name: "account SID with a slash", rawURL: "twilio://AC%2F" + testAccountSID[3:] + ":authToken@" + testFrom + "/" + testTo, wantErr: ErrInvalidAccountSID},
		{name: "account SID one character long", rawURL: "twilio://" + testAccountSID + "0:authToken@" + testFrom + "/" + testTo, wantErr: ErrInvalidAccountSID},
		{name: "account SID one character short", rawURL: "twilio://" + testAccountSID[:33] + ":authToken@" + testFrom + "/" + testTo, wantErr: ErrInvalidAccountSID},
		{name: "account SID that is not hexadecimal", rawURL: "twilio://AC" + strings.Repeat("g", 32) + ":authToken@" + testFrom + "/" + testTo, wantErr: ErrInvalidAccountSID},
		{name: "API key SID", rawURL: "twilio://SK" + testAccountSID[2:] + ":authToken@" + testFrom + "/" + testTo, wantErr: ErrInvalidAccountSID},
		{name: "API key that is not an API key SID", rawURL: "twilio://" + testAccountSID + ":keySecret@" + testFrom + "/" + testTo + "?apikey=bob", wantErr: ErrInvalidAPIKey},
		{name: "account SID as the API key", rawURL: "twilio://" + testAccountSID + ":keySecret@" + testFrom + "/" + testTo + "?apikey=" + testAccountSID, wantErr: ErrInvalidAPIKey},
		{name: "sender with a port", rawURL: "twilio://" + testAccountSID + ":authToken@+15551234567:443/" + testTo, wantErr: util.ErrInvalidHost},
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
// round-trips through SetURL. A slash in a recipient is escaped, so the
// recipient reads back whole.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	config := &Config{
		AccountSID: testAccountSID,
		AuthToken:  "authToken",
		APIKey:     testAPIKeySID,
		FromNumber: testFrom,
		ToNumbers:  []string{testTo, "+15551111111", "+1555/2222222"},
		Title:      "Alert",
	}

	serviceURL := config.GetURL()
	assert.Equal(t,
		"twilio://"+testAccountSID+":authToken@"+testFrom+"/"+testTo+"/+15551111111/+1555%2F2222222?apikey="+testAPIKeySID+"&title=Alert",
		serviceURL.String())

	roundTrip := &Config{}
	require.NoError(t, roundTrip.SetURL(serviceURL))
	assert.Equal(t, config, roundTrip)
}

// TestNormalizeSender verifies that spaces, dashes, parentheses, and dots are
// removed from a phone number or short code, while a sender with a letter,
// such as a Messaging Service SID or an alphanumeric sender ID, is kept as
// written.
func TestNormalizeSender(t *testing.T) {
	t.Parallel()

	// tests lists inputs with their normalized form.
	tests := []struct {
		input string
		want  string
	}{
		{input: "+1 (555) 123-4567", want: testFrom},
		{input: "+1.555.123.4567", want: testFrom},
		{input: testFrom, want: testFrom},
		{input: "123-456", want: "123456"},
		{input: testMessagingServiceSID, want: testMessagingServiceSID},
		{input: "Acme-Alerts", want: "Acme-Alerts"},
		{input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, normalizeSender(tt.input))
		})
	}
}

// TestParseToNumbers verifies that the escaped path splits into its decoded
// segments, and that an escaped slash stays in its number.
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
		{name: "several numbers", path: "/" + testTo + "/+15551111111", want: []string{testTo, "+15551111111"}},
		{name: "escaped slash stays in its number", path: "/%2B1555%2F1111111", want: []string{"+1555/1111111"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, parseToNumbers(tt.path))
		})
	}
}

// TestNormalizeRecipients verifies that formatting is stripped from each
// number and that empty numbers are dropped. Numbers already in shape are
// returned without a copy. A send never writes to the stored config.
func TestNormalizeRecipients(t *testing.T) {
	t.Parallel()

	numbers := []string{" +1 (555) 987-6543", "", "+15551111111", " "}
	assert.Equal(t, []string{testTo, "+15551111111"}, normalizeRecipients(numbers))
	assert.Equal(t, []string{" +1 (555) 987-6543", "", "+15551111111", " "}, numbers)

	clean := []string{testTo, "+15551111111"}
	assert.Same(t, &clean[0], &normalizeRecipients(clean)[0])
	assert.Empty(t, normalizeRecipients([]string{"", " "}))
}

// TestConfigQueryResolver verifies the config's query props. The title can be
// set and read. An unknown key is rejected. The only keys are apikey and
// title, and there are no enums.
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
	assert.Equal(t, []string{"apikey", "title"}, resolver.QueryFields())
}
