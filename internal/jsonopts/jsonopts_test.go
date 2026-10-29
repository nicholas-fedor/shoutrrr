package jsonopts_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// payload is a request body with the field kinds whose encoding the options decide.
type payload struct {
	Labels  map[string]string `json:"labels"`
	Tags    []string          `json:"tags"`
	Message string            `json:"message"`
}

// response is a provider response decoded with the options.
type response struct {
	Extra     map[string]any `json:"extra"`
	Message   string         `json:"message"`
	ErrorCode int            `json:"error_code"`
}

// TestDefaultMarshal verifies the bytes that marshaling with
// [jsonopts.Default] produces, for sorted map keys, nil and empty slices and
// maps, and invalid UTF-8.
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
		"nil slice and map as empty": {
			value: payload{Message: "hi"},
			want:  `{"labels":{},"tags":[],"message":"hi"}`,
		},
		"empty slice and map kept": {
			value: payload{Labels: map[string]string{}, Tags: []string{}},
			want:  `{"labels":{},"tags":[],"message":""}`,
		},
		"invalid UTF-8 replaced": {
			value: payload{Message: "a\xffb"},
			want:  `{"labels":{},"tags":[],"message":"a�b"}`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(tc.value, jsonopts.Default)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}

// TestDefaultUnmarshal verifies the values that unmarshaling with
// [jsonopts.Default] decodes. Member names match fields exactly, so a name in
// another case is ignored. Invalid UTF-8 is replaced.
func TestDefaultUnmarshal(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  response
	}{
		"exact names": {
			input: `{"error_code":400,"message":"bad"}`,
			want:  response{Extra: nil, Message: "bad", ErrorCode: 400},
		},
		"names in another case ignored": {
			input: `{"Error_Code":400,"MESSAGE":"bad"}`,
			want:  response{Extra: nil, Message: "", ErrorCode: 0},
		},
		"invalid UTF-8 replaced": {
			input: "{\"message\":\"a\xffb\"}",
			want:  response{Extra: nil, Message: "a\ufffdb", ErrorCode: 0},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var got response
			require.NoError(t, json.Unmarshal([]byte(tc.input), &got, jsonopts.Default))
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestDefaultUnmarshalRepeatedName verifies that [jsonopts.Default] fails a
// decode whose input repeats a member name.
func TestDefaultUnmarshalRepeatedName(t *testing.T) {
	t.Parallel()

	var got response
	require.Error(t, json.Unmarshal([]byte(`{"message":"first","message":"last"}`), &got, jsonopts.Default))
}

// TestUserInputUnmarshal verifies the values that unmarshaling with
// [jsonopts.UserInput] decodes, which follow the encoding/json rules. A
// member name matches a field in any case, but underscores stay significant.
// A repeated name keeps the last value even of another type. Invalid UTF-8 is
// replaced.
func TestUserInputUnmarshal(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input string
		want  response
	}{
		"exact names": {
			input: `{"error_code":400,"message":"bad"}`,
			want:  response{Extra: nil, Message: "bad", ErrorCode: 400},
		},
		"names in another case": {
			input: `{"Error_Code":400,"MESSAGE":"bad"}`,
			want:  response{Extra: nil, Message: "bad", ErrorCode: 400},
		},
		"underscore left out": {
			input: `{"errorcode":400}`,
			want:  response{Extra: nil, Message: "", ErrorCode: 0},
		},
		"dash in place of the underscore": {
			input: `{"error-code":400}`,
			want:  response{Extra: nil, Message: "", ErrorCode: 0},
		},
		"repeated name keeps the last": {
			input: `{"message":"first","message":"last"}`,
			want:  response{Extra: nil, Message: "last", ErrorCode: 0},
		},
		"repeated name of another type keeps the last": {
			input: `{"extra":{"a":1,"a":"x"}}`,
			want:  response{Extra: map[string]any{"a": "x"}, Message: "", ErrorCode: 0},
		},
		"invalid UTF-8 replaced": {
			input: "{\"message\":\"a\xffb\"}",
			want:  response{Extra: nil, Message: "a\ufffdb", ErrorCode: 0},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var got response
			require.NoError(t, json.Unmarshal([]byte(tc.input), &got, jsonopts.UserInput))
			assert.Equal(t, tc.want, got)
		})
	}
}
