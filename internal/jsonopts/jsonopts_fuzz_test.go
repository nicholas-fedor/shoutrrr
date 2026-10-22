package jsonopts_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jsonv1 "encoding/json"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// FuzzDefaultMarshal checks that a request body marshaled with
// [jsonopts.Default] holds the same JSON value as encoding/json produces, so
// only the escaping of characters can differ. The seed corpus covers plain
// text, HTML characters, invalid UTF-8, control characters, and the line and
// paragraph separators.
func FuzzDefaultMarshal(f *testing.F) {
	for _, seed := range []string{
		"",
		"hello",
		"<b>a & b</b>",
		"a\xffb",
		"tab\tnew\nline\x00",
		"\u2028\u2029",
	} {
		f.Add(seed, seed)
	}

	f.Fuzz(func(t *testing.T, message, key string) {
		value := payload{
			Labels:  map[string]string{key: message, "z" + key: key},
			Tags:    []string{message},
			Message: message,
		}

		got, err := json.Marshal(value, jsonopts.Default)
		require.NoError(t, err)

		legacy, err := jsonv1.Marshal(value)
		require.NoError(t, err)

		var gotValue, legacyValue any
		require.NoError(t, jsonv1.Unmarshal(got, &gotValue))
		require.NoError(t, jsonv1.Unmarshal(legacy, &legacyValue))
		assert.Equal(t, legacyValue, gotValue)
	})
}

// FuzzDefaultUnmarshal checks that unmarshaling with [jsonopts.Default]
// accepts exactly the input encoding/json accepts and decodes the same value,
// both into a struct and into an untyped value. Values are compared only
// when both decodes succeed, because encoding/json keeps decoding the members
// after a type error and v2 stops at it. The seed corpus covers exact
// and differently cased member names, repeated names, invalid UTF-8, a lone
// surrogate escape, arrays, and null. The regression seeds under testdata/fuzz
// repeat a name with a different value type and mistype a member before a valid one.
func FuzzDefaultUnmarshal(f *testing.F) {
	for _, seed := range []string{
		`{"error_code":400,"message":"bad"}`,
		`{"Error_Code":1,"MESSAGE":"x","message":"y"}`,
		"{\"message\":\"a\xffb\"}",
		`{"message":"\ud800"}`,
		`[1,2,{"a":null}]`,
		`null`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		var got, legacy response

		err := json.Unmarshal([]byte(input), &got, jsonopts.Default)
		legacyErr := jsonv1.Unmarshal([]byte(input), &legacy)
		require.Equal(t, legacyErr == nil, err == nil, "struct: v2 error %v, encoding/json error %v", err, legacyErr)

		if err == nil {
			assert.Equal(t, legacy, got)
		}

		var gotValue, legacyValue any

		err = json.Unmarshal([]byte(input), &gotValue, jsonopts.Default)
		legacyErr = jsonv1.Unmarshal([]byte(input), &legacyValue)
		require.Equal(t, legacyErr == nil, err == nil, "untyped: v2 error %v, encoding/json error %v", err, legacyErr)

		if err == nil {
			assert.Equal(t, legacyValue, gotValue)
		}
	})
}
