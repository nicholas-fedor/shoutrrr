package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTwilioBasic verifies that a basic SMS message reaches Twilio.
//
//nolint:paralleltest // Sends to a live service one message at a time.
func TestTwilioBasic(t *testing.T) {
	require.NoError(t, newService(t, nil).Send("E2E Test: Basic SMS notification", nil))
}
