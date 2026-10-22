package e2e_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ntfyMessage is a message as the ntfy JSON poll API returns it.
type ntfyMessage struct {
	ID          string   `json:"id"`
	Event       string   `json:"event"`
	Topic       string   `json:"topic"`
	Message     string   `json:"message"`
	Title       string   `json:"title"`
	Priority    int      `json:"priority"`
	Tags        []string `json:"tags"`
	ContentType string   `json:"content_type"`
}

// defaultMessageTimeout bounds each send and each poll for a sent message.
const defaultMessageTimeout = 5 * time.Second

// errMessageNotFound indicates that the poll response did not contain the
// expected message.
var errMessageNotFound = errors.New("message not found in response")

// TestNtfyBasic verifies that messages sent to a real ntfy server arrive with
// their text, title, priority, tags, and Markdown content type, and that
// params apply to one send only.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestNtfyBasic(t *testing.T) {
	t.Run("basic text content", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Basic text content notification")

		require.NoError(t, service.Send(message, nil))

		verifyMessageReceived(t, service.Config.Topic, message)
	})

	t.Run("message sent with a caller's context", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: SendContext notification")

		ctx, cancel := context.WithTimeout(t.Context(), defaultMessageTimeout)
		defer cancel()

		require.NoError(t, service.SendContext(ctx, message, nil))

		verifyMessageReceived(t, service.Config.Topic, message)
	})

	t.Run("params apply to one send only", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		withTitle := uniqueMessage("E2E Test: message with a title param")
		withoutTitle := uniqueMessage("E2E Test: message sent after the title param")

		require.NoError(t, service.Send(withTitle, &types.Params{"title": "Per-send title"}))
		require.NoError(t, service.Send(withoutTitle, nil))

		msg, err := pollForMessage(t.Context(), service.Config.Topic, withTitle)
		require.NoError(t, err)
		assert.Equal(t, "Per-send title", msg.Title)

		msg, err = pollForMessage(t.Context(), service.Config.Topic, withoutTitle)
		require.NoError(t, err)
		assert.Empty(t, msg.Title)
	})

	t.Run("empty message", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		require.NoError(t, service.Send("", nil))
	})

	t.Run("title via params", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Message with title")

		require.NoError(t, service.Send(message, &types.Params{"title": "Test Title"}))

		msg := verifyMessageReceived(t, service.Config.Topic, message)
		assert.Equal(t, "Test Title", msg.Title)
	})

	t.Run("priority via params", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: High priority message")

		require.NoError(t, service.Send(message, &types.Params{"priority": "5"}))

		msg := verifyMessageReceived(t, service.Config.Topic, message)
		assert.Equal(t, 5, msg.Priority)
	})

	t.Run("tags", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Tagged message")

		require.NoError(t, service.Send(message, &types.Params{"tags": "warning,skull"}))

		msg := verifyMessageReceived(t, service.Config.Topic, message)
		assert.ElementsMatch(t, []string{"warning", "skull"}, msg.Tags)
	})

	t.Run("markdown", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL()+"&markdown=yes")

		message := uniqueMessage("E2E Test: **bold** and *italic* markdown")

		require.NoError(t, service.Send(message, nil))

		msg := verifyMessageReceived(t, service.Config.Topic, message)
		assert.Equal(t, "text/markdown", msg.ContentType, "received event should reflect Markdown Content-Type")
	})

	t.Run("special characters and unicode", func(t *testing.T) {
		requireServer(t)

		service := initializeService(t, buildServiceURL())

		message := uniqueMessage("E2E Test: Special chars <>@#$% and unicode 世界 🌍")

		require.NoError(t, service.Send(message, nil))

		verifyMessageReceived(t, service.Config.Topic, message)
	})
}

// pollForMessage fetches the cached messages of a topic and returns the one
// with the expected text.
//
// Parameters:
//   - ctx: cancellation for the poll.
//   - topic: the topic the message was sent to.
//   - expectedMessage: the message text.
//
// Returns:
//   - *ntfyMessage: the message.
//   - error: a request or decoding error, or [errMessageNotFound].
func pollForMessage(ctx context.Context, topic, expectedMessage string) (*ntfyMessage, error) {
	apiURL := fmt.Sprintf("%s/%s/json?poll=1&message=%s", getNtfyBaseURL(), topic, url.QueryEscape(expectedMessage))

	ctx, cancel := context.WithTimeout(ctx, defaultMessageTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	client := &http.Client{Timeout: defaultMessageTimeout}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	for line := range strings.SplitSeq(strings.TrimSpace(string(body)), "\n") {
		if line == "" {
			continue
		}

		var msg ntfyMessage
		if err := json.Unmarshal([]byte(line), &msg, jsonopts.Default); err != nil {
			continue
		}

		if msg.Message == expectedMessage {
			return &msg, nil
		}
	}

	return nil, errMessageNotFound
}

// verifyMessageReceived requires the ntfy server to have received a message
// event with the given text on the topic.
//
// Parameters:
//   - t: the test.
//   - topic: the topic the message was sent to.
//   - message: the message text.
//
// Returns:
//   - *ntfyMessage: the received message, for further checks.
func verifyMessageReceived(t *testing.T, topic, message string) *ntfyMessage {
	t.Helper()

	msg, err := pollForMessage(t.Context(), topic, message)
	require.NoError(t, err, "message should be received by ntfy server")
	require.NotNil(t, msg)
	assert.Equal(t, "message", msg.Event)
	assert.Contains(t, msg.Topic, topic)

	return msg
}
