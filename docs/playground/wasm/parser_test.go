//go:build js && wasm

package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseURLString(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantService     string
		wantConfigKey   string
		wantConfigValue string
		wantContains    bool
		wantError       bool
	}{
		{
			name:        "parses discord URL",
			input:       "discord://token@123456789",
			wantService: "discord",
		},
		{
			name:        "parses ntfy URL",
			input:       "ntfy://ntfy.sh/mytopic",
			wantService: "ntfy",
		},
		{
			name:            "parses generic URL with webhook",
			input:           "generic://192.168.1.100:8123/api/webhook/abc123",
			wantService:     "generic",
			wantConfigKey:   "WebhookURL",
			wantConfigValue: "192.168.1.100",
			wantContains:    true,
		},
		{
			name:      "returns error for invalid URL",
			input:     "not-a-valid-url",
			wantError: true,
		},
		{
			name:            "handles URL with query parameters - color",
			input:           "discord://token@webhook?color=0x50D9ff&splitlines=Yes",
			wantConfigKey:   "Color",
			wantConfigValue: "0x50d9ff",
		},
		{
			name:            "handles URL with query parameters - splitlines",
			input:           "discord://token@webhook?color=0x50D9ff&splitlines=Yes",
			wantConfigKey:   "SplitLines",
			wantConfigValue: "Yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseURLString(tt.input)

			if tt.wantError {
				var errResp errorResult
				err := json.Unmarshal([]byte(result), &errResp)
				require.NoError(t, err)
				assert.NotEmpty(t, errResp.Error)

				return
			}

			var parsed parseResult
			err := json.Unmarshal([]byte(result), &parsed)
			require.NoError(t, err)
			assert.Equal(t, tt.wantService, parsed.Service)

			if tt.wantConfigKey != "" {
				if tt.wantContains {
					assert.Contains(t, parsed.Config[tt.wantConfigKey], tt.wantConfigValue)
				} else {
					assert.Equal(t, tt.wantConfigValue, parsed.Config[tt.wantConfigKey])
				}
			} else {
				assert.NotEmpty(t, parsed.Config)
			}
		})
	}
}

func TestValidateURLString(t *testing.T) {
	t.Run("returns valid for discord URL", func(t *testing.T) {
		result := validateURLString("discord://token@123456789")

		var valid map[string]bool
		err := json.Unmarshal([]byte(result), &valid)
		require.NoError(t, err)
		assert.True(t, valid["valid"])
	})

	t.Run("returns valid for ntfy URL", func(t *testing.T) {
		result := validateURLString("ntfy://ntfy.sh/mytopic")

		var valid map[string]bool
		err := json.Unmarshal([]byte(result), &valid)
		require.NoError(t, err)
		assert.True(t, valid["valid"])
	})

	t.Run("returns error for invalid URL", func(t *testing.T) {
		result := validateURLString("not-valid")

		var errResp errorResult
		err := json.Unmarshal([]byte(result), &errResp)
		require.NoError(t, err)
		assert.NotEmpty(t, errResp.Error)
	})

	t.Run("returns error for unknown service", func(t *testing.T) {
		result := validateURLString("unknown://something")

		var errResp errorResult
		err := json.Unmarshal([]byte(result), &errResp)
		require.NoError(t, err)
		assert.NotEmpty(t, errResp.Error)
	})
}
