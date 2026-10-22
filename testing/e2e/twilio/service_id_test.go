package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTwilioServiceID verifies the service ID of a service initialized from
// the configured URL.
func TestTwilioServiceID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "twilio", newService(t, nil).GetID())
}
