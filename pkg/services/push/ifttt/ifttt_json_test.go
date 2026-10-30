package ifttt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateJSONToSend verifies the webhook payload. It holds the configured
// values, with the message in the value field that messagevalue selects and a
// non-empty title in the field titlevalue selects.
func TestCreateJSONToSend(t *testing.T) {
	t.Parallel()

	// tests lists configs with the payload each should produce.
	tests := []struct {
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
			name:   "the message wins over a configured value in its field",
			config: Config{UseMessageAsValue: 1, Value1: "x", Value2: "y"},
			want:   `{"value1":"hello","value2":"y","value3":""}`,
		},
		{
			name:   "title in value1",
			config: Config{UseMessageAsValue: 2, UseTitleAsValue: 1, Title: "Title"},
			want:   `{"value1":"Title","value2":"hello","value3":""}`,
		},
		{
			name:   "title in value3",
			config: Config{UseMessageAsValue: 1, UseTitleAsValue: 3, Title: "Title"},
			want:   `{"value1":"hello","value2":"","value3":"Title"}`,
		},
		{
			name:   "title disabled",
			config: Config{UseMessageAsValue: 2, UseTitleAsValue: 0, Title: "Title"},
			want:   `{"value1":"","value2":"hello","value3":""}`,
		},
		{
			name:   "title overrides the configured value in its field",
			config: Config{UseMessageAsValue: 2, UseTitleAsValue: 1, Title: "Title", Value1: "x"},
			want:   `{"value1":"Title","value2":"hello","value3":""}`,
		},
		{
			name:   "the message wins over a title in the same field",
			config: Config{UseMessageAsValue: 1, UseTitleAsValue: 1, Title: "Title"},
			want:   `{"value1":"hello","value2":"","value3":""}`,
		},
		{
			name:   "empty title keeps the configured value in its field",
			config: Config{UseMessageAsValue: 2, UseTitleAsValue: 3, Value3: "c"},
			want:   `{"value1":"","value2":"hello","value3":"c"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := createJSONToSend(&tt.config, "hello")

			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(payload))
		})
	}
}
