package shoutrrr

import (
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/mqtttest"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// rootTestSecret is a credential embedded in test URLs that must never appear in errors.
const rootTestSecret = "SECRETrootTOKEN"

// Service URLs that fail without any network traffic.
const (
	// initFailureURL fails to initialize because pushover requires a user.
	initFailureURL = "pushover://:" + rootTestSecret + "@"
	// sendFailureURL fails to send because nothing listens on port 1 of the loopback address.
	sendFailureURL = "smtp://user:" + rootTestSecret + "@127.0.0.1:1/?fromaddress=from@example.invalid&toaddresses=to@example.invalid"
)

// TestSendErrorsOmitURL verifies that Send reports initialization and send failures
// without echoing the service URL, which carries credentials.
func TestSendErrorsOmitURL(t *testing.T) {
	t.Parallel()

	for name, rawURL := range map[string]string{"initialization": initFailureURL, "send": sendFailureURL} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Send(rawURL, "message")

			require.Error(t, err)
			assert.NotContains(t, err.Error(), rootTestSecret)
		})
	}
}

// TestSenderConstructorErrorsOmitURLs verifies that every sender constructor reports
// an invalid URL without echoing any of the configured URLs.
func TestSenderConstructorErrorsOmitURLs(t *testing.T) {
	t.Parallel()

	urls := []string{"logger://", initFailureURL}

	constructors := map[string]func() error{
		"CreateSender": func() error {
			_, err := CreateSender(urls...)

			return err
		},
		"CreateSenderWithOptions": func() error {
			_, err := CreateSenderWithOptions(types.SenderOptions{}, urls...)

			return err
		},
		"NewSender": func() error {
			_, err := NewSender(nil, urls...)

			return err
		},
		"NewSenderWithOptions": func() error {
			_, err := NewSenderWithOptions(nil, types.SenderOptions{}, urls...)

			return err
		},
	}

	for name, construct := range constructors {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := construct()

			require.Error(t, err)
			assert.NotContains(t, err.Error(), rootTestSecret)
			assert.Contains(t, err.Error(), "URL 1", "the error should identify the failing URL by position")
		})
	}
}

// TestSendClosesOneShotService verifies that Send closes the service it created,
// so a one-shot MQTT send disconnects from the broker instead of holding the
// connection until its idle teardown.
func TestSendClosesOneShotService(t *testing.T) {
	t.Parallel()

	broker := mqtttest.NewBroker(t)

	require.NoError(t, Send("mqtt://"+broker.Addr()+"/shoutrrr/test", "message"))

	require.Eventually(t, func() bool { return broker.Disconnected() == 1 },
		2*time.Second, 10*time.Millisecond, "Send must disconnect the one-shot MQTT service")

	// The broker reads packets in order, so the publish was recorded before the disconnect.
	assert.Equal(t, int32(1), broker.Published())
}

// TestSetLoggerDuringSends verifies that SetLogger can run while sends are in
// progress. Run with -race, it guards against unsynchronized access to the logger.
func TestSetLoggerDuringSends(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			SetLogger(log.New(io.Discard, "", 0))
		})
		wg.Go(func() {
			assert.NoError(t, Send("logger://", "message"))
		})
	}

	wg.Wait()
}
