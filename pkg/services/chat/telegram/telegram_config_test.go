package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

// TestConfigQueryAPI verifies that the config rejects unknown query keys when
// reading and parsing, and exposes its parse mode enum and seven query fields.
func TestConfigQueryAPI(t *testing.T) {
	t.Parallel()

	config := newTestService(t, testServiceURL).GetConfig()
	resolver := format.GetConfigQueryResolver(config)

	value, err := resolver.Get("invalid query var")
	require.Error(t, err)
	assert.Empty(t, value)

	require.Error(t, config.SetURL(parseURL(t, testServiceURL+"&foo=bar")))

	assert.Len(t, config.Enums(), 1)
	assert.Len(t, resolver.QueryFields(), 7)
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
// URL, which reads back to the same config, and leaves out the empty default.
func TestConfigAPIURLRoundTrip(t *testing.T) {
	t.Parallel()

	config := newTestService(t, testServiceURL+"&apiurl=http://localhost:8081").GetConfig()
	assert.Equal(t, "http://localhost:8081", config.APIURL)

	assert.Equal(t, config, newTestService(t, config.GetURL().String()).GetConfig())

	assert.NotContains(t, newTestService(t, testServiceURL).GetConfig().GetURL().String(), "apiurl")
}
