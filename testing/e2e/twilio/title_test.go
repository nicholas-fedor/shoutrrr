package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTwilioTitle verifies that Twilio accepts an SMS with a title from the
// URL prepended.
//
//nolint:paralleltest // Sends to a live service one message at a time.
func TestTwilioTitle(t *testing.T) {
	require.NoError(t, newService(t, map[string]string{"title": "E2E Alert"}).Send("E2E Test: Message with title", nil))
}
