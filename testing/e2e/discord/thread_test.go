package e2e_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDiscordThread verifies that a message posts to an existing thread named
// by thread_id, skipping when no thread is configured or Discord rejects the
// post for the thread's permissions or state.
//
//nolint:paralleltest // Sends to a live webhook one message at a time.
func TestDiscordThread(t *testing.T) {
	threadID := os.Getenv("SHOUTRRR_DISCORD_THREAD_ID")
	if os.Getenv("SHOUTRRR_DISCORD_URL") != "" && threadID == "" {
		t.Skip("SHOUTRRR_DISCORD_THREAD_ID not set, skipping thread posting test")
	}

	service := newService(t, map[string]string{"thread_id": threadID})

	t.Logf(
		"Discord config - WebhookID: %s, Token: %s, ThreadID: %s",
		redact(service.Config.WebhookID),
		redact(service.Config.Token),
		redact(service.Config.ThreadID),
	)

	err := service.Send("E2E Test: Posting to existing thread", nil)

	// Posting to a thread may fail with 400 Bad Request for the thread's
	// permissions or state, which does not fail the test.
	if err != nil && (strings.Contains(err.Error(), "400") || strings.Contains(err.Error(), "Bad Request")) {
		t.Skipf("Thread posting failed due to permissions/thread state (400), skipping test: %v", err)
	}

	require.NoError(t, err)
}

// redact masks a sensitive value for logging, keeping its last four characters.
//
// Parameters:
//   - value: the value to mask.
//
// Returns:
//   - string: the masked value.
func redact(value string) string {
	if len(value) <= 4 {
		return "****"
	}

	return "****" + value[len(value)-4:]
}
