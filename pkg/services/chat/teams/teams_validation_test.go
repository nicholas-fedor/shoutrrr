package teams

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateWebhookURL verifies that ValidateWebhookURL accepts the Logic
// Apps and Power Platform workflow URL formats and rejects everything else
// with ErrInvalidWebhookURL.
func TestValidateWebhookURL(t *testing.T) {
	t.Parallel()

	// tests lists webhook URLs and whether ValidateWebhookURL accepts them.
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "logic apps with port", url: "https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke"},
		{name: "logic apps without port", url: "https://prod-00.westus.logic.azure.com/workflows/abc123/triggers/manual/paths/invoke"},
		{name: "logic apps with query", url: "https://mytenant.logic.azure.com/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00"},
		{name: "logic apps US government cloud", url: "https://prod-00.westus.logic.azure.us/workflows/abc123/triggers/manual/paths/invoke"},
		{name: "logic apps China cloud", url: "https://prod-00.westus.logic.azure.cn/workflows/abc123/triggers/manual/paths/invoke"},
		{name: "logic apps Germany cloud", url: "https://prod-00.westus.logic.azure.de/workflows/abc123/triggers/manual/paths/invoke"},
		{name: "logic apps bare TLD", url: "https://prod-00.westus.logic.azure/workflows/abc123"},
		{name: "logic apps with routing segments", url: "https://prod-00.westus.logic.azure.com/powerautomate/automations/direct/cu/04/workflows/abc123/triggers/manual/paths/invoke"},
		{
			name: "power platform",
			url:  "https://default.environment.api.powerplatform.com/powerautomate/automations/direct/workflows/abc123/triggers/manual/paths/invoke?api-version=1&sp=%2Ftriggers%2Fmanual%2Frun&sv=1.0&sig=XXXXXXXX",
		},
		{
			name: "power platform with nested subdomain and port",
			url:  "https://prefix.environment.environment.api.powerplatform.com:443/powerautomate/automations/direct/workflows/abc123/triggers/manual/paths/invoke",
		},
		{
			name: "power platform with routing segments",
			url:  "https://default0123456789abcdef.00.environment.api.powerplatform.com:443/powerautomate/automations/direct/cu/04/workflows/abc123/triggers/manual/paths/invoke?api-version=1&sp=%2Ftriggers%2Fmanual%2Frun&sv=1.0&sig=XXXXXXXX",
		},
		{name: "empty", url: "", wantErr: true},
		{name: "unrelated host", url: "https://example.com/webhook", wantErr: true},
		{name: "plain HTTP", url: "http://prod-00.westus.logic.azure.com/workflows/abc123", wantErr: true},
		{name: "logic apps without workflows path", url: "https://prod-00.westus.logic.azure.com/triggers/manual", wantErr: true},
		{name: "logic apps host without subdomain", url: "https://logic.azure.com/workflows/abc123", wantErr: true},
		{name: "logic apps host as subdomain of another host", url: "https://prod.logic.azure.com.example.com/workflows/abc123", wantErr: true},
		{name: "logic apps host in userinfo", url: "https://prod.logic.azure.com@example.com/workflows/abc123", wantErr: true},
		{name: "logic apps host in path", url: "https://example.com/prod.logic.azure.com/workflows/abc123", wantErr: true},
		{
			name:    "power platform without direct segment",
			url:     "https://default.environment.api.powerplatform.com/powerautomate/automations/workflows/abc123/triggers/manual/paths/invoke",
			wantErr: true,
		},
		{
			name:    "power platform without powerautomate path",
			url:     "https://default.environment.api.powerplatform.com/workflows/abc123",
			wantErr: true,
		},
		{
			name:    "power platform without environment subdomain",
			url:     "https://default.api.powerplatform.com/powerautomate/automations/direct/workflows/abc123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateWebhookURL(tt.url)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidWebhookURL)

				return
			}

			assert.NoError(t, err)
		})
	}
}
