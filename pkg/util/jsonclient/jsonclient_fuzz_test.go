package jsonclient

import (
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
)

// fuzzResponse is the typed value FuzzParseResponse decodes response bodies
// into, so bodies with the wrong field types fail as well as malformed ones.
type fuzzResponse struct {
	// Name is a string field.
	Name string `json:"name"`
	// Count is a number field.
	Count int `json:"count"`
	// Items is an array field.
	Items []string `json:"items"`
	// Nested is an object field.
	Nested map[string]any `json:"nested"`
}

// FuzzParseResponse checks that an error status always fails with an [Error]
// that matches [ErrUnexpectedStatus] and keeps the status and the exact body,
// without decoding it, and that any other status decodes exactly what
// [json.Unmarshal] decodes, failing with an [Error] that keeps the status, the
// body, and the decoding error when it fails. The seed corpus covers valid,
// malformed, mistyped, and empty bodies with success, redirect, client error,
// and server error statuses.
func FuzzParseResponse(f *testing.F) {
	for _, seed := range []struct {
		body   string
		status int
	}{
		{`{"name":"a","count":1,"items":["x"],"nested":{"k":true}}`, http.StatusOK},
		{`{}`, http.StatusCreated},
		{``, http.StatusNoContent},
		{`{"count":"one"}`, http.StatusOK},
		{`{"name":`, http.StatusOK},
		{`[1,2,3]`, http.StatusFound},
		{`{"error":"bad request"}`, http.StatusBadRequest},
		{`not json`, http.StatusInternalServerError},
	} {
		f.Add(seed.body, seed.status)
	}

	f.Fuzz(func(t *testing.T, body string, status int) {
		if status < 100 || status > 999 {
			t.Skip()
		}

		res := &http.Response{
			StatusCode: status,
			Status:     strconv.Itoa(status) + " " + http.StatusText(status),
			Body:       io.NopCloser(strings.NewReader(body)),
		}

		var got fuzzResponse

		err := parseResponse(res, &got)

		if status >= HTTPClientErrorThreshold {
			requireJSONError(t, err, status, body)
			require.ErrorIs(t, err, ErrUnexpectedStatus)
			assert.Zero(t, got, "an error status must not decode the body")

			return
		}

		var want fuzzResponse

		wantErr := json.Unmarshal([]byte(body), &want, jsonopts.Default)
		if wantErr == nil {
			require.NoError(t, err)
			assert.Equal(t, want, got)

			return
		}

		requireJSONError(t, err, status, body)
		require.NotErrorIs(t, err, ErrUnexpectedStatus)
		assert.EqualError(t, errors.Unwrap(err), wantErr.Error())
	})
}

// requireJSONError requires err to be an [Error] with the given status and body,
// readable through [ErrorBody].
//
// Parameters:
//   - t: the test.
//   - err: the error to check.
//   - status: the expected status code.
//   - body: the expected response body.
func requireJSONError(t *testing.T, err error, status int, body string) {
	t.Helper()

	jsonErr, ok := errors.AsType[Error](err)
	require.True(t, ok, "error %v is not a jsonclient.Error", err)
	assert.Equal(t, status, jsonErr.StatusCode)
	assert.Equal(t, body, jsonErr.Body)
	assert.Equal(t, body, ErrorBody(err))
}
