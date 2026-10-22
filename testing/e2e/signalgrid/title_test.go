package e2e_test

import (
	"testing"
)

// TestSignalgridTitle verifies that a notification with a title from the URL
// reaches Signalgrid.
//
//nolint:paralleltest // Sends to a live service one message at a time.
func TestSignalgridTitle(t *testing.T) {
	sendWithQuery(t, map[string]string{"title": "E2E Alert"}, "E2E Test: Message with title")
}
