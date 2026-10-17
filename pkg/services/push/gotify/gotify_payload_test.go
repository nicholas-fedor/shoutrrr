package gotify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestParseExtras verifies that extras from the params replace the config's
// extras, that missing or empty param extras leave the config's extras, and
// that param extras that are not a JSON object are rejected.
func TestParseExtras(t *testing.T) {
	t.Parallel()

	configExtras := map[string]any{"client::display": map[string]any{"contentType": "text/plain"}}

	// tests lists params with the extras each should produce.
	tests := []struct {
		params *types.Params
		want   map[string]any
		name   string
	}{
		{
			name:   "param extras",
			params: &types.Params{"extras": `{"client::display":{"contentType":"text/markdown"}}`},
			want:   map[string]any{"client::display": map[string]any{"contentType": "text/markdown"}},
		},
		{name: "no params", want: configExtras},
		{name: "no extras param", params: &types.Params{"title": "Backup"}, want: configExtras},
		{name: "empty extras param", params: &types.Params{"extras": ""}, want: configExtras},
		{name: "null extras param", params: &types.Params{"extras": "null"}, want: configExtras},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			extras, err := (&DefaultPayloadBuilder{}).ParseExtras(tt.params, &Config{Extras: configExtras})

			require.NoError(t, err)
			assert.Equal(t, tt.want, extras)
		})
	}

	for _, invalid := range []string{"{invalid", `["a"]`} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()

			_, err := (&DefaultPayloadBuilder{}).ParseExtras(&types.Params{"extras": invalid}, &Config{Extras: configExtras})

			require.ErrorIs(t, err, ErrExtrasUnmarshalFailed)
		})
	}
}

// TestPrepareRequest verifies that the request carries the message, the
// config's title and priority, the extras, and the date only when one is set.
func TestPrepareRequest(t *testing.T) {
	t.Parallel()

	builder := &DefaultPayloadBuilder{}
	config := &Config{Title: "Backup", Priority: 5}
	extras := map[string]any{"client::notification": map[string]any{"click": map[string]any{"url": "https://example.com"}}}
	date := "2023-12-25T10:00:00Z"

	assert.Equal(t,
		&MessageRequest{Message: "Hello", Title: "Backup", Priority: 5, Date: &date, Extras: extras},
		builder.PrepareRequest("Hello", config, extras, date))
	assert.Equal(t,
		&MessageRequest{Message: "Hello", Title: "Backup", Priority: 5},
		builder.PrepareRequest("Hello", config, nil, ""))
}
