package e2e_test

import (
	"testing"
)

// TestSignalgridBasic verifies that a basic notification reaches Signalgrid.
//
//nolint:paralleltest // Sends to a live service one message at a time.
func TestSignalgridBasic(t *testing.T) {
	sendWithQuery(t, nil, "E2E Test: Basic Signalgrid notification")
}
