package e2e_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestZulipBasic verifies that a real Zulip server accepts a stream message,
// one with a topic or stream from params, and a direct message.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestZulipBasic(t *testing.T) {
	t.Run("basic text message to a stream", func(t *testing.T) {
		service := newService(t, requireServiceURL(t))

		require.NoError(t, service.Send("E2E Test: Basic text notification", nil))
	})

	t.Run("custom topic via params", func(t *testing.T) {
		service := newService(t, requireServiceURL(t))

		require.NoError(t, service.Send("E2E Test: Custom topic notification", &types.Params{
			"topic": "e2e-testing",
		}))
	})

	t.Run("stream via params override", func(t *testing.T) {
		service := newService(t, requireServiceURL(t))

		// The params name the same stream as the config.
		stream := os.Getenv("SHOUTRRR_ZULIP_STREAM")
		if stream == "" {
			stream = "general"
		}

		require.NoError(t, service.Send("E2E Test: Stream via params notification", &types.Params{
			"stream": stream,
		}))
	})

	t.Run("direct message when a recipient is configured", func(t *testing.T) {
		serviceURL := requireServiceURL(t)

		dmTo := os.Getenv("SHOUTRRR_ZULIP_DM_TO")
		if dmTo == "" {
			// Fall back to the bot's own email to exercise the DM code path
			// without requiring an extra env var. Self-DMs are usually allowed
			// and validate the type/to/JSON recipient formatting.
			dmTo = os.Getenv("SHOUTRRR_ZULIP_BOT_MAIL")
		}

		if dmTo == "" {
			t.Skip("Zulip DM recipient not configured, skipping direct message E2E test")
		}

		service := newService(t, serviceURL)

		require.NoError(t, service.Send("E2E Test: Direct message notification", &types.Params{
			"type": "direct",
			"to":   dmTo,
		}))
	})
}
