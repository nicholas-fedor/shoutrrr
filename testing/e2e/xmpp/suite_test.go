package e2e_test

import (
	"os"
	"testing"
)

// envOrSkip returns the value of an environment variable, skipping the test
// when it is unset.
//
// Parameters:
//   - t: the test.
//   - key: the environment variable, one of the SHOUTRRR_XMPP_* URLs listed in
//     the README.
//
// Returns:
//   - string: the variable's value.
func envOrSkip(t *testing.T, key string) string {
	t.Helper()

	value := os.Getenv(key)
	if value == "" {
		t.Skip(key + " not set, skipping XMPP e2e test")
	}

	return value
}
