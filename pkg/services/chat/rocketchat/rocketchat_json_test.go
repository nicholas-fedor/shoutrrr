package rocketchat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestCreateJSONPayload verifies the payload built from the config and the
// params that override its username and channel, including that empty
// optional fields are omitted.
func TestCreateJSONPayload(t *testing.T) {
	t.Parallel()

	// tests lists configs and params with the JSON payload they should produce.
	tests := []struct {
		name   string
		config *Config
		params *types.Params
		want   string
	}{
		{
			name:   "text only omits empty fields",
			config: &Config{},
			want:   `{"text":"hello"}`,
		},
		{
			name:   "config values with nil params",
			config: &Config{UserName: "bot", Channel: "#general"},
			want:   `{"text":"hello","username":"bot","channel":"#general"}`,
		},
		{
			name:   "username param",
			config: &Config{UserName: "bot", Channel: "#general"},
			params: &types.Params{"username": "override"},
			want:   `{"text":"hello","username":"override","channel":"#general"}`,
		},
		{
			name:   "channel param",
			config: &Config{UserName: "bot", Channel: "#general"},
			params: &types.Params{"channel": "@user"},
			want:   `{"text":"hello","username":"bot","channel":"@user"}`,
		},
		{
			name:   "both params",
			config: &Config{UserName: "bot", Channel: "#general"},
			params: &types.Params{"username": "override", "channel": "@user"},
			want:   `{"text":"hello","username":"override","channel":"@user"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := CreateJSONPayload(tt.config, "hello", tt.params)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(payload))
		})
	}
}
