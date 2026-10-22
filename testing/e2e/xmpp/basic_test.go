package e2e_test

import (
	"log"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mellium.im/xmpp/stanza"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/xmpp"
)

// receiveTimeout is how long the probe waits for a sent message to arrive.
const receiveTimeout = 5 * time.Second

// TestXMPP verifies that messages sent to a real XMPP server over STARTTLS,
// implicit TLS, and plaintext reach a 1:1 recipient and open and
// password-protected rooms, and that a protected room without its password
// and bad credentials fail the send.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestXMPP(t *testing.T) {
	t.Run("1:1 chat over STARTTLS", func(t *testing.T) {
		sendAndReceive(t, envOrSkip(t, "SHOUTRRR_XMPP_URL"), "E2E Test: STARTTLS chat")
	})

	t.Run("1:1 chat over implicit TLS", func(t *testing.T) {
		sendAndReceive(t, envOrSkip(t, "SHOUTRRR_XMPP_TLS_URL"), "E2E Test: implicit TLS chat")
	})

	t.Run("1:1 chat over plaintext when disabletls is set", func(t *testing.T) {
		sendAndReceive(t, envOrSkip(t, "SHOUTRRR_XMPP_PLAIN_URL"), "E2E Test: plaintext chat")
	})

	t.Run("open MUC", func(t *testing.T) {
		sendAndReceive(t, envOrSkip(t, "SHOUTRRR_XMPP_MUC_URL"), "E2E Test: open MUC")
	})

	t.Run("password-protected MUC", func(t *testing.T) {
		sendAndReceive(t, envOrSkip(t, "SHOUTRRR_XMPP_MUC_PASSWORD_URL"), "E2E Test: protected MUC")
	})

	t.Run("password-protected MUC without its password", func(t *testing.T) {
		serviceURL, err := url.Parse(envOrSkip(t, "SHOUTRRR_XMPP_MUC_PASSWORD_URL"))
		require.NoError(t, err)

		query := serviceURL.Query()
		query.Del("roompassword")
		serviceURL.RawQuery = query.Encode()

		service := &xmpp.Service{}
		require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))

		// The room requires its password, so the server refuses the join with a
		// not-authorized error (XEP-0045), which a connection failure would not
		// produce.
		var stanzaErr stanza.Error

		err = service.Send("E2E Test: protected MUC without its password", nil)
		require.ErrorAs(t, err, &stanzaErr)
		assert.Equal(t, stanza.NotAuthorized, stanzaErr.Condition, err.Error())
	})

	t.Run("bad credentials", func(t *testing.T) {
		serviceURL, err := url.Parse(envOrSkip(t, "SHOUTRRR_XMPP_URL"))
		require.NoError(t, err)

		serviceURL.User = url.UserPassword("sender", "wrong-password")

		service := &xmpp.Service{}
		require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))
		require.ErrorIs(t, service.Send("should fail", nil), xmpp.ErrAuthenticationFailed)
	})
}

// sendAndReceive sends a message through the service a URL configures and
// requires the probe account, signed in as the recipient or a room occupant,
// to receive it.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - message: the message to send.
func sendAndReceive(t *testing.T, rawURL, message string) {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	probe, err := startInbox(t.Context(), serviceURL)
	require.NoError(t, err)

	defer probe.close()

	service := &xmpp.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))
	require.NoError(t, service.Send(message, nil))

	waitForBody(t, probe.bodies, message)
}

// waitForBody requires a body equal to want to arrive within [receiveTimeout],
// skipping any other bodies that arrive first.
//
// Parameters:
//   - t: the test.
//   - bodies: the bodies the probe receives.
//   - want: the expected body.
func waitForBody(t *testing.T, bodies <-chan string, want string) {
	t.Helper()

	timeout := time.After(receiveTimeout)

	for {
		select {
		case body := <-bodies:
			if body == want {
				return
			}
		case <-timeout:
			require.Failf(t, "message not received", "no body %q within %s", want, receiveTimeout)

			return
		}
	}
}
