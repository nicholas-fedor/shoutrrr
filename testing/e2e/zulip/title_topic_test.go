package e2e_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestZulipTitleAndTopic verifies that a real Zulip server accepts messages
// whose topic comes from the title, from an explicit topic that takes
// precedence over the title, or from either alone, and a long title.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestZulipTitleAndTopic(t *testing.T) {
	// sends lists the messages with the title and topic params for each.
	sends := []struct {
		params        types.Params
		name, message string
	}{
		{
			name:    "title as topic when no topic is configured",
			message: "E2E Test: title used as topic",
			params:  types.Params{"title": "e2e-title-as-topic"},
		},
		{
			name:    "explicit topic kept when a title is also given",
			message: "E2E Test: body content",
			params:  types.Params{"topic": "e2e-explicit-topic", "title": "E2E Notification Title"},
		},
		{
			name:    "long title as topic",
			message: "E2E Test: long title as topic",
			params:  types.Params{"title": strings.Repeat("x", 50)},
		},
		{
			name:    "topic only",
			message: "E2E Test: topic only",
			params:  types.Params{"topic": "e2e-topic-only"},
		},
		{
			name:    "title only",
			message: "E2E Test: title only",
			params:  types.Params{"title": "e2e-title-only"},
		},
	}

	for _, tt := range sends {
		t.Run(tt.name, func(t *testing.T) {
			service := newService(t, requireServiceURL(t))

			require.NoError(t, service.Send(tt.message, &tt.params))
		})
	}
}
