package transport

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
