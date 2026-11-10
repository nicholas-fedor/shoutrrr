//go:build js && wasm

package main

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenerateURLString verifies that a URL is generated from config values
// set by query key or by Go field name.
//
// Values that do not parse, keys that name no field, and invalid input are
// rejected, and an empty value is skipped.
func TestGenerateURLString(t *testing.T) {
	t.Parallel()

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
			name:       "generates ntfy URL with host and topic",
			service:    "ntfy",
			configJSON: `{"Host":"ntfy.sh","Topic":"mytopic"}`,
			wantSubstr: []string{"ntfy://", "ntfy.sh/mytopic"},
		},
		{
			name:       "sets an int URL field by name",
			service:    "mqtt",
			configJSON: `{"Host":"broker.example.com","Port":"1884","Topic":"alerts"}`,
			wantSubstr: []string{"broker.example.com:1884"},
		},
		{
			name:       "sets a list URL field by name",
			service:    "pushbullet",
			configJSON: `{"Token":"tokentokentokentokentokentokentoke","Targets":"device1,device2"}`,
			wantSubstr: []string{"device1", "device2"},
		},
		{
			name:       "skips an empty value",
			service:    "mqtt",
			configJSON: `{"Host":"broker.example.com","Port":"","Topic":"alerts"}`,
			wantSubstr: []string{"broker.example.com/alerts"},
		},
		{
			name:        "rejects a key that names no field",
			service:     "mqtt",
			configJSON:  `{"Host":"broker.example.com","NoSuchField":"x","Topic":"alerts"}`,
			wantErrResp: true,
		},
		{
			name:        "rejects an int URL field that does not parse",
			service:     "mqtt",
			configJSON:  `{"Host":"broker.example.com","Port":"high","Topic":"alerts"}`,
			wantErrResp: true,
		},
		{
			name:        "rejects a webhook URL that does not parse",
			service:     "generic",
			configJSON:  `{"WebhookURL":"example.com/%zz"}`,
			wantErrResp: true,
		},
		{
			name:       "keeps a webhook query value that differs from its default",
			service:    "generic",
			configJSON: `{"WebhookURL":"example.com/hook?titlekey=&method=GET"}`,
			wantURL:    "generic://example.com/hook?method=GET&titlekey=",
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
			t.Parallel()

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
