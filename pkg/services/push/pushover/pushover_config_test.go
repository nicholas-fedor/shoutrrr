package pushover

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// Encryption keys in the 64-character hex format the Pushover apps use.
const (
	// testEncryptionKey is a lowercase key.
	testEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	// testEncryptionKeyMixed is a mixed-case key.
	testEncryptionKeyMixed = "0123456789ABCDEF0123456789abcdef0123456789ABCDEF0123456789abcdef"
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

// TestConfigSetURL verifies that SetURL reads the user key from the host, the
// API token from the password, and the devices, priority, title, and
// encryption key from the query.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config each should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "user and token",
			rawURL: "pushover://Token:apptoken@simme",
			want:   Config{User: "simme", Token: "apptoken"},
		},
		{
			name:   "devices, priority, and title",
			rawURL: "pushover://:apptoken@usertoken?devices=a,b,c,d&priority=-2&title=new+title",
			want:   Config{User: "usertoken", Token: "apptoken", Devices: []string{"a", "b", "c", "d"}, Priority: -2, Title: "new title"},
		},
		{
			name:   "mixed-case encryption key",
			rawURL: "pushover://:apptoken@usertoken?encryptionkey=" + testEncryptionKeyMixed,
			want:   Config{User: "usertoken", Token: "apptoken", EncryptionKey: testEncryptionKeyMixed},
		},
		{
			name:   "encryption key alias",
			rawURL: "pushover://:apptoken@usertoken?key=" + testEncryptionKey,
			want:   Config{User: "usertoken", Token: "apptoken", EncryptionKey: testEncryptionKey},
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

// TestConfigSetURLErrors verifies that SetURL rejects a URL without a user key
// or API token, an unknown or invalid query prop, and an encryption key that
// is not 64 hex characters.
func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid service URLs with the error each should produce.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
	}{
		{name: "missing user", rawURL: "pushover://Token:token@", wantErr: ErrUserMissing},
		{name: "missing token", rawURL: "pushover://Token:@user", wantErr: ErrTokenMissing},
		{name: "unknown prop", rawURL: "pushover://:token@user?devicey=a", wantErr: format.ErrInvalidConfigKey},
		{name: "priority not a number", rawURL: "pushover://:token@user?priority=super-duper", wantErr: format.ErrParseIntFailed},
		{name: "short encryption key", rawURL: "pushover://:token@user?encryptionkey=0123456789abcdef", wantErr: ErrInvalidEncryptionKey},
		{name: "long encryption key", rawURL: "pushover://:token@user?encryptionkey=" + testEncryptionKey + "ab", wantErr: ErrInvalidEncryptionKey},
		{
			name:    "non-hex encryption key",
			rawURL:  "pushover://:token@user?encryptionkey=zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			wantErr: ErrInvalidEncryptionKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}

// TestConfigGetURL verifies that GetURL puts the user key in the host and the
// token in the password, writes the encryption key under its primary name,
// and round-trips through SetURL.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	config := &Config{User: "simme", Token: "test-token", EncryptionKey: testEncryptionKey, Devices: []string{"a", "b"}}
	serviceURL := config.GetURL()

	password, ok := serviceURL.User.Password()
	require.True(t, ok)
	assert.Equal(t, "test-token", password)
	assert.Equal(t, "simme", serviceURL.Host)
	assert.Equal(t, Scheme, serviceURL.Scheme)
	assert.Equal(t, testEncryptionKey, serviceURL.Query().Get("encryptionkey"))
	assert.Empty(t, serviceURL.Query().Get("key"))

	roundTrip := &Config{}
	require.NoError(t, roundTrip.SetURL(serviceURL))
	assert.Equal(t, config, roundTrip)
}

// TestConfigQueryResolver verifies the config's query props: devices join with
// commas, an unknown key is rejected, and the keys include the encryption key
// alias with no enums.
func TestConfigQueryResolver(t *testing.T) {
	t.Parallel()

	t.Run("devices joined by commas", func(t *testing.T) {
		t.Parallel()

		value, err := format.GetConfigQueryResolver(&Config{Devices: []string{"a", "b", "c"}}).Get("devices")

		require.NoError(t, err)
		assert.Equal(t, "a,b,c", value)
	})

	t.Run("unknown key", func(t *testing.T) {
		t.Parallel()

		value, err := format.GetConfigQueryResolver(&Config{}).Get("devicey")

		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
		assert.Empty(t, value)
	})

	t.Run("default values", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		resolver := format.NewPropKeyResolver(config)

		require.NoError(t, resolver.SetDefaultProps(config))
		assert.Equal(t, int8(0), config.Priority)
	})

	t.Run("fields and enums", func(t *testing.T) {
		t.Parallel()

		config := &Config{}

		assert.Empty(t, config.Enums())
		assert.Equal(t,
			[]string{"devices", "encryptionkey", "key", "priority", "timestamp", "title"},
			format.GetConfigQueryResolver(config).QueryFields())
	})
}

// TestMaskURL verifies that MaskURL replaces the token in the user info and
// every secret query parameter, and leaves other parts unchanged.
func TestMaskURL(t *testing.T) {
	t.Parallel()

	// tests lists URLs with the masked URL each should become.
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "token in the user info",
			rawURL: "pushover://Token:apptoken@user?devices=a",
			want:   "pushover://Token:REDACTED@user?devices=a",
		},
		{
			name:   "secret query parameters",
			rawURL: "pushover://host?token=t&user=u&encryptionkey=e&key=k&title=Title",
			want:   "pushover://host?encryptionkey=REDACTED&key=REDACTED&title=Title&token=REDACTED&user=REDACTED",
		},
		{
			name:   "nothing to mask",
			rawURL: "pushover://host?title=Title",
			want:   "pushover://host?title=Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL := parseTestURL(t, tt.rawURL)
			MaskURL(serviceURL)

			assert.Equal(t, tt.want, serviceURL.String())
		})
	}

	t.Run("nil URL", func(t *testing.T) {
		t.Parallel()

		assert.NotPanics(t, func() { MaskURL(nil) })
	})
}
