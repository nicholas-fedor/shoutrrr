package wecom

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequestBodyJSON verifies the JSON field names of the webhook payload and
// that empty mention lists are omitted.
func TestRequestBodyJSON(t *testing.T) {
	t.Parallel()

	// tests lists payloads with the JSON each should marshal to.
	tests := []struct {
		name string
		body RequestBody
		want string
	}{
		{
			name: "without mentions",
			body: RequestBody{MsgType: "text", Text: TextContent{Content: "hello"}},
			want: `{"msgtype":"text","text":{"content":"hello"}}`,
		},
		{
			name: "with mentions",
			body: RequestBody{MsgType: "text", Text: TextContent{
				Content:             "hello",
				MentionedList:       []string{"@all"},
				MentionedMobileList: []string{"13800001111"},
			}},
			want: `{"msgtype":"text","text":{"content":"hello","mentioned_list":["@all"],"mentioned_mobile_list":["13800001111"]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.body)
			require.NoError(t, err)

			assert.JSONEq(t, tt.want, string(data))
		})
	}
}

// TestResponseJSON verifies that the webhook API response decodes into its
// error code and message.
func TestResponseJSON(t *testing.T) {
	t.Parallel()

	var response Response
	require.NoError(t, json.Unmarshal([]byte(`{"errcode":93000,"errmsg":"invalid webhook url"}`), &response))

	assert.Equal(t, Response{ErrCode: 93000, ErrMsg: "invalid webhook url"}, response)
}
