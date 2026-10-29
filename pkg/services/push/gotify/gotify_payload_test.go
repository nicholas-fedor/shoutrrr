package gotify

import (
	"encoding/json/v2"
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
		{
			name:   "param extras with a repeated key keep the last",
			params: &types.Params{"extras": `{"a":1,"a":"x"}`},
			want:   map[string]any{"a": "x"},
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
// config's title and priority, the extras, and the date only when one is set,
// and leaves the priority out when the config has none of its own.
func TestPrepareRequest(t *testing.T) {
	t.Parallel()

	builder := &DefaultPayloadBuilder{}
	extras := map[string]any{"client::notification": map[string]any{"click": map[string]any{"url": "https://example.com"}}}
	date := "2023-12-25T10:00:00Z"

	assert.Equal(t,
		&MessageRequest{Message: "Hello", Title: "Backup", Priority: 5, Date: &date, Extras: extras},
		builder.PrepareRequest("Hello", &Config{Title: "Backup", Priority: 5}, extras, date))
	assert.Equal(t,
		&MessageRequest{Message: "Hello", Title: "Backup", omitPriority: true},
		builder.PrepareRequest("Hello", &Config{Title: "Backup"}, nil, ""))
	assert.Equal(t,
		&MessageRequest{Message: "Hello", Title: "Backup"},
		builder.PrepareRequest("Hello", &Config{Title: "Backup", prioritySet: true}, nil, ""))
}

// TestMessageRequestMarshalJSON verifies the JSON body of a message: the
// message, title, and extras, the priority unless the request leaves it out,
// and never the date, which Gotify sets itself.
func TestMessageRequestMarshalJSON(t *testing.T) {
	t.Parallel()

	date := "2023-12-25T10:00:00Z"

	// tests lists requests with the JSON each should produce.
	tests := []struct {
		name    string
		want    string
		request MessageRequest
	}{
		{
			name:    "priority",
			request: MessageRequest{Message: "Hello", Title: "Backup", Priority: 5, Date: &date, Extras: map[string]any{"a": 1}},
			want:    `{"message":"Hello","title":"Backup","priority":5,"extras":{"a":1}}`,
		},
		{name: "zero priority", request: MessageRequest{Message: "Hello", Title: "Backup"}, want: `{"message":"Hello","title":"Backup","priority":0}`},
		{name: "priority left out", request: MessageRequest{Message: "Hello", Title: "Backup", omitPriority: true}, want: `{"message":"Hello","title":"Backup"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body, err := json.Marshal(&tt.request)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(body))

			body, err = json.Marshal(tt.request)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(body))
		})
	}
}
