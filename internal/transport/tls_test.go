package transport

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
)

// recordingLogger records what is logged through it.
type recordingLogger struct {
	// lines holds each Log call's arguments.
	lines [][]any
}

// Log records v.
//
// Parameters:
//   - v: the values to log.
func (l *recordingLogger) Log(v ...any) {
	l.lines = append(l.lines, v)
}

// TestTLSConfig verifies that both helpers require TLS 1.2 and set the server
// name. Only the insecure settings skip verification. Each call returns new
// settings.
func TestTLSConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		build      func(string) *tls.Config
		name       string
		skipVerify bool
	}{
		{name: "verifying", build: TLSConfig, skipVerify: false},
		{name: "insecure", build: InsecureTLSConfig, skipVerify: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := tt.build("mail.example.com")

			assert.Equal(t, "mail.example.com", config.ServerName)
			assert.Equal(t, uint16(tls.VersionTLS12), config.MinVersion)
			assert.Equal(t, tt.skipVerify, config.InsecureSkipVerify)
			assert.Empty(t, tt.build("").ServerName)
			assert.NotSame(t, config, tt.build("mail.example.com"))
		})
	}
}

// TestWarnInsecure verifies that the warning is logged once with the shared
// text.
func TestWarnInsecure(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	WarnInsecure(logger)

	assert.Equal(t, [][]any{{insecureWarning}}, logger.lines)
}
