package gotify

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// TestResponseErrorError verifies that a Gotify error response reads as its
// name, code, and description.
func TestResponseErrorError(t *testing.T) {
	t.Parallel()

	err := &responseError{Name: "Unauthorized", Code: 401, Description: "you need to provide a valid access token or user credentials to access this api"}

	assert.EqualError(t, err, "server responded with Unauthorized (401): you need to provide a valid access token or user credentials to access this api")
}

// TestMessageRequestJSON verifies the exact JSON body of a message with
// Shoutrrr's JSON options, by value and through a pointer. The priority is
// left out when nil and the date is never sent. Nil extras are left out and
// empty extras are sent. Extras keys are sorted at every level.
func TestMessageRequestJSON(t *testing.T) {
	t.Parallel()

	date := "2023-12-25T10:00:00Z"

	// tests lists requests with the JSON each should produce.
	tests := []struct {
		name    string
		want    string
		request MessageRequest
	}{
		{
			name:    "priority and extras",
			request: MessageRequest{Message: "Hello", Title: "Backup", Priority: new(5), Date: &date, Extras: Extras{"a": 1}},
			want:    `{"extras":{"a":1},"priority":5,"message":"Hello","title":"Backup"}`,
		},
		{
			name:    "zero priority",
			request: MessageRequest{Message: "Hello", Title: "Backup", Priority: new(0)},
			want:    `{"priority":0,"message":"Hello","title":"Backup"}`,
		},
		{
			name:    "priority left out",
			request: MessageRequest{Message: "Hello", Title: "Backup"},
			want:    `{"message":"Hello","title":"Backup"}`,
		},
		{
			name:    "empty extras and text to escape",
			request: MessageRequest{Message: "<b>&</b> \"q\" \xff", Priority: new(-2), Extras: Extras{}},
			want:    "{\"extras\":{},\"priority\":-2,\"message\":\"<b>&</b> \\\"q\\\" \uFFFD\",\"title\":\"\"}",
		},
		{
			name: "nested extras in key order",
			request: MessageRequest{Message: "m", Title: "t", Priority: new(1), Extras: Extras{
				"z": "last", "a": []any{1, "x"}, "client::display": map[string]any{"contentType": "text/markdown"},
			}},
			want: `{"extras":{"a":[1,"x"],"client::display":{"contentType":"text/markdown"},"z":"last"},"priority":1,"message":"m","title":"t"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body, err := json.Marshal(&tt.request, jsonopts.Default)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(body))

			body, err = json.Marshal(tt.request, jsonopts.Default)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(body))
		})
	}
}

// TestExtrasMarshalJSONToError verifies that extras holding a value JSON
// cannot encode fail the body and name the value's path.
func TestExtrasMarshalJSONToError(t *testing.T) {
	t.Parallel()

	_, err := json.Marshal(&MessageRequest{Extras: Extras{"c": make(chan int)}}, jsonopts.Default)

	var semanticErr *json.SemanticError
	require.ErrorAs(t, err, &semanticErr)
	assert.Equal(t, "/extras/c", string(semanticErr.JSONPointer))
}
