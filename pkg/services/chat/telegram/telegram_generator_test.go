package telegram

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/mattn/go-colorable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/util/generator"
)

// Fixtures for the generator tests.
const (
	// generatorToken is the bot token the generator tests type in.
	generatorToken = `0:MockToken`
	// generatorAPIBase is the Telegram API base URL for generatorToken.
	generatorAPIBase = "https://api.telegram.org/bot" + generatorToken + "/"
)

// errUnexpectedRequest answers a request that no test responder matches.
var errUnexpectedRequest = errors.New("unexpected request to the Telegram API")

// botInfoResponder answers getMe with a bot named mockbot.
var botInfoResponder = httpmock.NewJsonResponderOrPanic(http.StatusOK, &struct {
	OK     bool
	Result *User
}{
	OK:     true,
	Result: &User{ID: 1, IsBot: true, Username: "mockbot"},
})

// activateTelegramMock routes the default HTTP transport to httpmock for the
// rest of the test. The generator builds its own clients on
// http.DefaultTransport, so these tests cannot inject a client and must not run
// in parallel.
//
// Parameters:
//   - t: the test that owns the mock.
func activateTelegramMock(t *testing.T) {
	t.Helper()

	httpmock.Activate(t)
}

// newTestGenerator returns a generator that reads the given input lines and
// writes its output, without color codes, to output.
//
// Parameters:
//   - input: the lines the user types, in order.
//   - output: receives the generator's output.
//
// Returns:
//   - *Generator: the generator.
func newTestGenerator(input []string, output *bytes.Buffer) *Generator {
	var reader strings.Builder

	for _, line := range input {
		reader.WriteString(line + "\n")
	}

	return &Generator{
		Reader: strings.NewReader(reader.String()),
		Writer: colorable.NewNonColorable(output),
	}
}

// TestGeneratorBuildsURLFromUpdates verifies that the generator reports the
// updates it receives and builds a URL with the chat a message came from.
//
//nolint:paralleltest // httpmock replaces the shared http.DefaultTransport.
func TestGeneratorBuildsURLFromUpdates(t *testing.T) {
	activateTelegramMock(t)

	httpmock.RegisterResponder(http.MethodGet, generatorAPIBase+"getMe", botInfoResponder)
	httpmock.RegisterResponder(http.MethodPost, generatorAPIBase+"getUpdates", httpmock.NewJsonResponderOrPanic(http.StatusOK, &struct {
		OK     bool
		Result []Update
	}{
		OK: true,
		Result: []Update{
			{
				ChatMemberUpdate: &ChatMemberUpdate{
					Chat:          &Chat{Type: `channel`, Title: `mockChannel`},
					OldChatMember: &ChatMember{Status: `kicked`},
					NewChatMember: &ChatMember{Status: `administrator`},
				},
			},
			{
				Message: &Message{
					Text: "hi!",
					From: &User{Username: `mockUser`},
					Chat: &Chat{Type: `private`, ID: 667, Username: `mockUser`},
				},
			},
		},
	}))

	var output bytes.Buffer

	config, err := newTestGenerator([]string{generatorToken, "no"}, &output).Generate(nil, nil, nil)

	require.NoError(t, err, "output:\n%s", output.String())
	require.NotNil(t, config)
	assert.Equal(t, `telegram://0:MockToken@telegram?chats=667&preview=No`, config.GetURL().String())

	assert.Contains(t, output.String(), `Got a bot chat member update for mockChannel, status was changed from kicked to administrator`)
	assert.Contains(t, output.String(), `Got 1 chat ID(s) so far. Want to add some more?`)
	assert.Contains(t, output.String(), `Selected chats:`)
	assert.Contains(t, output.String(), `667 (private) @mockUser`)
}

// TestGeneratorReportsAPIFailures verifies that the generator returns an error
// naming the failed step when Telegram answers with an error status.
//
//nolint:paralleltest // httpmock replaces the shared http.DefaultTransport.
func TestGeneratorReportsAPIFailures(t *testing.T) {
	badGateway := httpmock.NewStringResponder(http.StatusBadGateway, "<html>Bad Gateway</html>")

	// tests lists getMe and getUpdates responders with the step the error should
	// name.
	tests := []struct {
		name       string
		getMe      httpmock.Responder
		getUpdates httpmock.Responder
		wantStep   string
	}{
		{name: "bot info", getMe: badGateway, getUpdates: badGateway, wantStep: "getting bot info"},
		{name: "updates", getMe: botInfoResponder, getUpdates: badGateway, wantStep: "getting updates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activateTelegramMock(t)

			httpmock.RegisterResponder(http.MethodGet, generatorAPIBase+"getMe", tt.getMe)
			httpmock.RegisterResponder(http.MethodPost, generatorAPIBase+"getUpdates", tt.getUpdates)

			var output bytes.Buffer

			_, err := newTestGenerator([]string{generatorToken}, &output).Generate(nil, nil, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantStep)
		})
	}
}

// TestGeneratorStopsWithoutToken verifies that the generator fails before
// contacting Telegram when the input ends before a token is given.
//
//nolint:paralleltest // httpmock replaces the shared http.DefaultTransport.
func TestGeneratorStopsWithoutToken(t *testing.T) {
	activateTelegramMock(t)
	// httpmock counts only requests that reach a responder, so an unmatched
	// request needs one for the call count to include it.
	httpmock.RegisterNoResponder(httpmock.NewErrorResponder(errUnexpectedRequest))

	gen := &Generator{Reader: strings.NewReader(""), Writer: io.Discard}

	_, err := gen.Generate(nil, nil, nil)

	require.ErrorIs(t, err, generator.ErrInputClosed)
	assert.Zero(t, httpmock.GetTotalCallCount())
}
