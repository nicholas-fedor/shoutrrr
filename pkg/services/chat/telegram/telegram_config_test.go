package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// TestConfigQueryAPI verifies that the config rejects unknown query keys when
// reading and parsing, and exposes its parse mode enum and eight query fields.
func TestConfigQueryAPI(t *testing.T) {
	t.Parallel()

	config := newTestService(t, testServiceURL).GetConfig()
	resolver := format.GetConfigQueryResolver(config)

	value, err := resolver.Get("invalid query var")
	require.Error(t, err)
	assert.Empty(t, value)

	require.Error(t, config.SetURL(parseURL(t, testServiceURL+"&foo=bar")))

	assert.Len(t, config.Enums(), 1)
	assert.Len(t, resolver.QueryFields(), 8)
}

// TestConfigGetURLToken verifies that the config URL holds the bot ID and
// secret of a token as its userinfo, and that a token without a colon, such
// as a masked one, becomes the username instead of panicking.
func TestConfigGetURLToken(t *testing.T) {
	t.Parallel()

	config := &Config{Token: "110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw", Chats: []string{"@channel"}}
	assert.Equal(t, "110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw", config.GetURL().User.String())

	config.Token = format.MaskedValue
	assert.Equal(t, format.MaskedValue, config.GetURL().User.String())
	assert.NotContains(t, format.MaskedURL(&Config{Token: "1:secret", Chats: []string{"@c"}}).String(), "secret")
}

// TestConfigAPIURLRoundTrip verifies that the config URL holds a configured API
// URL and skiptlsverify, which read back to the same config, and leaves out
// the defaults.
func TestConfigAPIURLRoundTrip(t *testing.T) {
	t.Parallel()

	config := newTestService(t, testServiceURL+"&apiurl=https://localhost:8443&skiptlsverify=yes").GetConfig()
	assert.Equal(t, "https://localhost:8443", config.APIURL)
	assert.True(t, config.SkipTLSVerify)

	assert.Equal(t, config, newTestService(t, config.GetURL().String()).GetConfig())

	defaultURL := newTestService(t, testServiceURL).GetConfig().GetURL().String()
	assert.NotContains(t, defaultURL, "apiurl")
	assert.NotContains(t, defaultURL, "skiptlsverify")
}

// TestParseChat verifies that a chat reads into its chat ID and thread ID,
// and that a thread ID that is not a positive integer fails with
// [ErrInvalidThreadID] instead of being dropped.
func TestParseChat(t *testing.T) {
	t.Parallel()

	// tests lists chats with the target they read into, or with whether they
	// fail.
	tests := []struct {
		name    string
		chat    string
		want    chatTarget
		wantErr bool
	}{
		{name: "chat ID", chat: "-1001234", want: chatTarget{threadID: nil, chat: "-1001234", id: "-1001234"}},
		{name: "channel", chat: "@alerts", want: chatTarget{threadID: nil, chat: "@alerts", id: "@alerts"}},
		{name: "thread ID", chat: "-1001234:7", want: chatTarget{threadID: new(7), chat: "-1001234:7", id: "-1001234"}},
		{name: "word", chat: "-1001234:general", wantErr: true},
		{name: "empty", chat: "-1001234:", wantErr: true},
		{name: "zero", chat: "-1001234:0", wantErr: true},
		{name: "negative", chat: "-1001234:-7", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target, err := parseChat(tt.chat)

			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidThreadID)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, target)
		})
	}
}
