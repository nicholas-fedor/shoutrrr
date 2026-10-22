package jsonopts_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jsonv1 "encoding/json"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// payload is a request body with the field kinds whose encoding the options keep.
type payload struct {
	Labels  map[string]string `json:"labels"`
	Tags    []string          `json:"tags"`
	Message string            `json:"message"`
}

// response is a provider response decoded with the options.
type response struct {
	ErrorCode int    `json:"error_code"`
	Message   string `json:"message"`
}

// TestDefaultMarshal verifies that marshaling with [jsonopts.Default] produces
// the same bytes as encoding/json, for sorted map keys, nil slices and maps,
// and invalid UTF-8.
func TestDefaultMarshal(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value any
		want  string
	}{
		"map keys sorted": {
			value: map[string]int{"b": 2, "a": 1, "c": 3},
			want:  `{"a":1,"b":2,"c":3}`,
		},
		"nil slice and map as null": {
			value: payload{Message: "hi"},
			want:  `{"labels":null,"tags":null,"message":"hi"}`,
		},
		"empty slice and map kept": {
			value: payload{Labels: map[string]string{}, Tags: []string{}},
			want:  `{"labels":{},"tags":[],"message":""}`,
		},
		"invalid UTF-8 replaced": {
			value: payload{Message: "a\xffb"},
			want:  `{"labels":null,"tags":null,"message":"a�b"}`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(tc.value, jsonopts.Default)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))

			legacy, err := jsonv1.Marshal(tc.value)
			require.NoError(t, err)
			assert.JSONEq(t, string(legacy), string(got))
		})
	}
}

// TestDefaultUnmarshal verifies that unmarshaling with [jsonopts.Default]
// accepts the input encoding/json accepts and decodes the same value, for
// member names in another case, repeated names, and invalid UTF-8.
func TestDefaultUnmarshal(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  response
	}{
		"exact names": {
			input: `{"error_code":400,"message":"bad"}`,
			want:  response{ErrorCode: 400, Message: "bad"},
		},
		"names in another case": {
			input: `{"Error_Code":400,"MESSAGE":"bad"}`,
			want:  response{ErrorCode: 400, Message: "bad"},
		},
		"delimiters stay significant": {
			input: `{"errorcode":400,"message":"bad"}`,
			want:  response{ErrorCode: 0, Message: "bad"},
		},
		"repeated name keeps the last": {
			input: `{"message":"first","message":"last"}`,
			want:  response{ErrorCode: 0, Message: "last"},
		},
		"invalid UTF-8 replaced": {
			input: "{\"message\":\"a\xffb\"}",
			want:  response{ErrorCode: 0, Message: "a�b"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var got response
			require.NoError(t, json.Unmarshal([]byte(tc.input), &got, jsonopts.Default))
			assert.Equal(t, tc.want, got)

			var legacy response
			require.NoError(t, jsonv1.Unmarshal([]byte(tc.input), &legacy))
			assert.Equal(t, legacy, got)
		})
	}
}
