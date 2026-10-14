package ifttt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestCreateJSONToSend verifies the webhook payload: the configured values,
// overridden by value params, with the message in the value field that
// messagevalue selects.
func TestCreateJSONToSend(t *testing.T) {
	t.Parallel()

	// tests lists configs and params with the payload each should produce.
	tests := []struct {
		params *types.Params
		name   string
		want   string
		config Config
	}{
		{
			name:   "message in value1",
			config: Config{UseMessageAsValue: 1},
			want:   `{"value1":"hello","value2":"","value3":""}`,
		},
		{
			name:   "message in value2",
			config: Config{UseMessageAsValue: 2},
			want:   `{"value1":"","value2":"hello","value3":""}`,
		},
		{
			name:   "message in value3",
			config: Config{UseMessageAsValue: 3},
			want:   `{"value1":"","value2":"","value3":"hello"}`,
		},
		{
			name:   "configured values",
			config: Config{UseMessageAsValue: 1, Value2: "b", Value3: "c"},
			want:   `{"value1":"hello","value2":"b","value3":"c"}`,
		},
		{
			name:   "params override the configured values",
			config: Config{UseMessageAsValue: 2, Value1: "a", Value2: "b", Value3: "c"},
			params: &types.Params{"value1": "x", "value3": "z"},
			want:   `{"value1":"x","value2":"hello","value3":"z"}`,
		},
		{
			name:   "the message wins over a value param in its field",
			config: Config{UseMessageAsValue: 1},
			params: &types.Params{"value1": "x", "value2": "y"},
			want:   `{"value1":"hello","value2":"y","value3":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := createJSONToSend(&tt.config, "hello", tt.params)

			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(payload))
		})
	}
}
