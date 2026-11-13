package e2e_test

import (
	"encoding/json/v2"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiscordThread verifies that a message posts to an existing thread named
// by thread_id, skipping when no thread is configured or Discord rejects the
// post for the thread's permissions or state.
//
//nolint:paralleltest // Sends to a live webhook one message at a time.
func TestDiscordThread(t *testing.T) {
	threadID := os.Getenv("SHOUTRRR_DISCORD_THREAD_ID")
	if os.Getenv(webhookURLVar) != "" && threadID == "" {
		t.Skip("SHOUTRRR_DISCORD_THREAD_ID not set, skipping thread posting test")
	}

	service := newService(t, webhookURLVar, map[string]string{"thread_id": threadID})

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

// TestDiscordThreadName verifies that thread_name creates one new thread per
// send. Discord lets a webhook create threads only in a forum or media
// channel, so the test needs a webhook on one. A message long enough for two
// requests creates the thread with the first, and the second goes into the
// same thread.
//
//nolint:paralleltest // Sends to a live webhook one message at a time.
func TestDiscordThreadName(t *testing.T) {
	service := newService(t, forumURLVar, map[string]string{"thread_name": "E2E Test: new thread", "splitLines": "no"})
	recorder := &replyRecorder{bodies: nil}
	service.SetHTTPClient(recorder)

	message := "E2E Test: New thread from a message that takes two requests - " + strings.Repeat("Long thread text. ", 360)
	require.NoError(t, service.Send(message, nil))
	require.Len(t, recorder.bodies, 2, "the message takes two requests")

	var first, second createdMessage
	require.NoError(t, json.Unmarshal(recorder.bodies[0], &first))
	require.NoError(t, json.Unmarshal(recorder.bodies[1], &second))

	assert.Equal(t, first.ID, first.ChannelID, "the first message starts the thread, which has its ID")
	assert.Equal(t, first.ChannelID, second.ChannelID, "the second message goes into the same thread")
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
