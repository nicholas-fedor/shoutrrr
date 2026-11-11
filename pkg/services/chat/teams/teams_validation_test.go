package teams

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateWebhookURL verifies that ValidateWebhookURL accepts signed Power
// Platform and sovereign-cloud Logic Apps workflow URLs and rejects everything
// else with ErrInvalidWebhookURL.
//
// Logic Apps URLs on logic.azure.com stopped working on 2025-11-30, and Azure
// Germany closed in 2021, so both are rejected.
func TestValidateWebhookURL(t *testing.T) {
	t.Parallel()

	const (
		// pp is a Power Platform workflow URL without its query.
		pp = "https://default.environment.api.powerplatform.com/powerautomate/automations/direct/workflows/abc123/triggers/manual/paths/invoke"
		// sig is a signed query, as every workflow URL that anyone can call has.
		sig = "?api-version=1&sp=%2Ftriggers%2Fmanual%2Frun&sv=1.0&sig=XXXXXXXX"
		// logicPath is a Logic Apps workflow path.
		logicPath = "/workflows/abc123/triggers/manual/paths/invoke"
	)

	// tests lists webhook URLs and whether ValidateWebhookURL accepts them.
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "power platform", url: pp + sig},
		{
			name: "power platform with nested subdomain and port",
			url:  "https://prefix.environment.environment.api.powerplatform.com:443/powerautomate/automations/direct/workflows/abc123" + sig,
		},
		{
			name: "power platform with routing segments",
			url:  "https://default0123456789abcdef.00.environment.api.powerplatform.com:443/powerautomate/automations/direct/cu/04/workflows/abc123" + sig,
		},
		{name: "logic apps US government cloud", url: "https://prod-00.usgovvirginia.logic.azure.us" + logicPath + sig},
		{name: "logic apps US government cloud with port", url: "https://prod-00.usgovvirginia.logic.azure.us:443" + logicPath + sig},
		{name: "logic apps China cloud", url: "https://prod-00.chinaeast.logic.azure.cn" + logicPath + sig},
		{
			name: "logic apps with routing segments",
			url:  "https://prod-00.usgovvirginia.logic.azure.us/powerautomate/automations/direct/cu/04/workflows/abc123" + sig,
		},
		{
			name: "power platform with an uppercase host",
			url:  "https://DEFAULT.ENVIRONMENT.API.POWERPLATFORM.COM/powerautomate/automations/direct/workflows/abc123" + sig,
		},
		{name: "signature only", url: pp + "?sig=X"},
		{name: "percent-encoded signature key", url: pp + "?%73ig=X"},
		{name: "signature before a fragment", url: pp + "?sig=X#top"},
		{name: "retired logic apps cloud", url: "https://prod-00.westus.logic.azure.com:443" + logicPath + sig, wantErr: true},
		{name: "closed Germany cloud", url: "https://prod-00.germanycentral.logic.azure.de" + logicPath + sig, wantErr: true},
		{name: "logic apps bare TLD", url: "https://prod-00.westus.logic.azure" + logicPath + sig, wantErr: true},
		{name: "no query", url: pp, wantErr: true},
		{name: "no signature", url: pp + "?api-version=1&sp=%2Ftriggers&sv=1.0", wantErr: true},
		{name: "empty signature", url: pp + "?sig=", wantErr: true},
		{name: "signature emptied by a fragment", url: pp + "?sig=#X", wantErr: true},
		{name: "signature only in the fragment", url: pp + "?a=1#sig=X", wantErr: true},
		{name: "parameter that ends in sig", url: pp + "?xsig=X", wantErr: true},
		{name: "malformed query escape", url: pp + "?sig=X&a=%zz", wantErr: true},
		{name: "unparsable path", url: "https://default.environment.api.powerplatform.com/powerautomate/automations/direct/workflows/%zz" + sig, wantErr: true},
		{name: "port 0", url: "https://default.environment.api.powerplatform.com:0/powerautomate/automations/direct/workflows/abc123" + sig, wantErr: true},
		{name: "port above 65535", url: "https://default.environment.api.powerplatform.com:65536/powerautomate/automations/direct/workflows/abc123" + sig, wantErr: true},
		{name: "empty", url: "", wantErr: true},
		{name: "unrelated host", url: "https://example.com/webhook" + sig, wantErr: true},
		{name: "plain HTTP", url: "http://default.environment.api.powerplatform.com/powerautomate/automations/direct/workflows/abc123" + sig, wantErr: true},
		{name: "logic apps without workflows path", url: "https://prod-00.usgovvirginia.logic.azure.us/triggers/manual" + sig, wantErr: true},
		{name: "logic apps host without subdomain", url: "https://logic.azure.us" + logicPath + sig, wantErr: true},
		{
			name:    "power platform host with an empty label",
			url:     "https://default..environment.api.powerplatform.com/powerautomate/automations/direct/workflows/abc123" + sig,
			wantErr: true,
		},
		{
			name:    "power platform host with a symbol in a label",
			url:     "https://a&b.environment.api.powerplatform.com/powerautomate/automations/direct/workflows/abc123" + sig,
			wantErr: true,
		},
		{name: "logic apps host as subdomain of another host", url: "https://prod.logic.azure.us.example.com" + logicPath + sig, wantErr: true},
		{name: "userinfo before a workflow host", url: "https://user@prod-00.usgovvirginia.logic.azure.us" + logicPath + sig, wantErr: true},
		{name: "logic apps host in userinfo", url: "https://prod.logic.azure.us@example.com" + logicPath + sig, wantErr: true},
		{name: "logic apps host in path", url: "https://example.com/prod.logic.azure.us" + logicPath + sig, wantErr: true},
		{
			name:    "power platform without direct segment",
			url:     "https://default.environment.api.powerplatform.com/powerautomate/automations/workflows/abc123" + sig,
			wantErr: true,
		},
		{name: "power platform without powerautomate path", url: "https://default.environment.api.powerplatform.com/workflows/abc123" + sig, wantErr: true},
		{
			name:    "power platform without environment subdomain",
			url:     "https://default.api.powerplatform.com/powerautomate/automations/direct/workflows/abc123" + sig,
			wantErr: true,
		},
		{
			name:    "power platform with a dot in a routing segment",
			url:     "https://default.environment.api.powerplatform.com/powerautomate/automations/direct/c.u/workflows/abc123" + sig,
			wantErr: true,
		},
		{
			name:    "power platform with an empty routing segment",
			url:     "https://default.environment.api.powerplatform.com/powerautomate/automations/direct//workflows/abc123" + sig,
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
