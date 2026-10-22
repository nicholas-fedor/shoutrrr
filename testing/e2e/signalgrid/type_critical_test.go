package e2e_test

import (
	"testing"
)

// TestSignalgridTypeAndCritical verifies that Signalgrid accepts a CRIT
// notification without critical delivery and an INFO notification with it.
//
//nolint:paralleltest // Sends to a live service one message at a time.
func TestSignalgridTypeAndCritical(t *testing.T) {
	t.Run("CRIT type without critical delivery", func(t *testing.T) {
		sendWithQuery(t, map[string]string{"type": "CRIT", "title": "E2E CRIT"},
			"E2E Test: CRIT type without critical flag")
	})

	t.Run("critical delivery", func(t *testing.T) {
		sendWithQuery(t, map[string]string{"type": "INFO", "critical": "true", "title": "E2E Critical"},
			"E2E Test: critical delivery flag")
	})
}
