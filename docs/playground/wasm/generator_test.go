//go:build js && wasm

package main

import (
	"encoding/json"
	"testing"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateURLString(t *testing.T) {
	tests := []struct {
		name        string
		service     string
		configJSON  string
		wantURL     string
		wantSubstr  []string
		wantErrResp bool
	}{
		{
			name:       "generates discord URL with webhook and token",
			service:    "discord",
			configJSON: `{"WebhookID":"123456789","Token":"mytoken"}`,
			wantSubstr: []string{"discord://", "123456789", "mytoken"},
		},
		{
			name:       "generates ntfy URL with host and path",
			service:    "ntfy",
			configJSON: `{"Host":"ntfy.sh","Path":"mytopic"}`,
			wantSubstr: []string{"ntfy://", "ntfy.sh"},
		},
		{
			name:       "generates generic URL with webhook",
			service:    "generic",
			configJSON: `{"WebhookURL":"192.168.1.100:8123/api/webhook/abc123?foo=bar"}`,
			wantSubstr: []string{"generic://", "192.168.1.100", "foo=bar"},
		},
		{
			name:        "returns error for invalid service",
			service:     "nonexistent",
			configJSON:  `{}`,
			wantErrResp: true,
		},
		{
			name:        "returns error for invalid JSON",
			service:     "discord",
			configJSON:  "not-json",
			wantErrResp: true,
		},
		{
			name:       "generates logger URL",
			service:    "logger",
			configJSON: `{}`,
			wantURL:    "logger://",
		},
		{
			name:       "generates smtp URL with host and port",
			service:    "smtp",
			configJSON: `{"Host":"smtp.example.com","Port":"587","FromAddress":"sender@example.com","ToAddresses":"recipient@example.com"}`,
			wantSubstr: []string{"smtp://", "smtp.example.com:587"},
		},
		{
			name:        "returns error for invalid port value",
			service:     "smtp",
			configJSON:  `{"Host":"smtp.example.com","Port":"notanumber","FromAddress":"sender@example.com","ToAddresses":"recipient@example.com"}`,
			wantErrResp: true,
		},
		{
			name:        "returns error for port exceeding uint16 max",
			service:     "smtp",
			configJSON:  `{"Host":"smtp.example.com","Port":"65536","FromAddress":"sender@example.com","ToAddresses":"recipient@example.com"}`,
			wantErrResp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateURLString(tt.service, tt.configJSON)

			if tt.wantErrResp {
				var errResp errorResult
				err := json.Unmarshal([]byte(result), &errResp)
				require.NoError(t, err)
				assert.NotEmpty(t, errResp.Error)

				return
			}

			var parsed map[string]string
			err := json.Unmarshal([]byte(result), &parsed)
			require.NoError(t, err)

			if tt.wantURL != "" {
				assert.Equal(t, tt.wantURL, parsed["url"])
			}

			for _, substr := range tt.wantSubstr {
				assert.Contains(t, parsed["url"], substr)
			}
		})
	}
}

func TestGetServiceConfigFromService(t *testing.T) {
	r := router.ServiceRouter{}
	service, err := r.NewService("discord")
	require.NoError(t, err)

	config, ok := getServiceConfigFromService(service)
	assert.False(t, ok)
	assert.Nil(t, config)
}
