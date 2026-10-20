package generic

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNormalizedHeaderKey verifies that header keys from the URL are written
// in HTTP form: the first letter of each word capitalized, with a dash before
// a capital that follows a lowercase letter or a digit, so camelCase keys gain
// dashes while acronyms, digits, and other characters stay as written.
func TestNormalizedHeaderKey(t *testing.T) {
	t.Parallel()

	// tests lists header keys with their normalized form.
	tests := []struct {
		key  string
		want string
	}{
		{key: "authorization", want: "Authorization"},
		{key: "Authorization", want: "Authorization"},
		{key: "contentType", want: "Content-Type"},
		{key: "x-custom-header", want: "X-Custom-Header"},
		{key: "X-Request-ID", want: "X-Request-ID"},
		{key: "X-API-Key", want: "X-API-Key"},
		{key: "acceptLanguage", want: "Accept-Language"},
		{key: "acceptlanguage", want: "Acceptlanguage"},
		{key: "xApiKey", want: "X-Api-Key"},
		{key: "x-aB", want: "X-A-B"},
		{key: "x-1st-header", want: "X-1st-Header"},
		{key: "v2Token", want: "V2-Token"},
		{key: "0abc", want: "0abc"},
		{key: "X-Custom_Header", want: "X-Custom_Header"},
		{key: "x--y", want: "X--Y"},
		{key: "-leading", want: "-Leading"},
		{key: "día-ñandú", want: "Día-ñandú"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, normalizedHeaderKey(tt.key))
		})
	}
}

// TestStripCustomQueryValues verifies that @ keys become headers and $ keys
// become extra data, both removed from the query, while other keys and bare
// prefixes stay in it.
func TestStripCustomQueryValues(t *testing.T) {
	t.Parallel()

	query := url.Values{
		"@authorization": {"Bearer token"},
		"$context":       {"inside joke"},
		"template":       {"json"},
		"@":              {"bare header prefix"},
		"$":              {"bare extra prefix"},
	}

	headers, extraData := stripCustomQueryValues(query)

	assert.Equal(t, map[string]string{"Authorization": "Bearer token"}, headers)
	assert.Equal(t, map[string]string{"context": "inside joke"}, extraData)
	assert.Equal(t, url.Values{
		"template": {"json"},
		"@":        {"bare header prefix"},
		"$":        {"bare extra prefix"},
	}, query)
}

// TestAppendCustomQueryValues verifies that headers and extra data are added
// to the query with their @ and $ prefixes.
func TestAppendCustomQueryValues(t *testing.T) {
	t.Parallel()

	query := url.Values{"template": {"json"}}

	appendCustomQueryValues(query, map[string]string{"Authorization": "frend"}, map[string]string{"context": "inside joke"})

	assert.Equal(t, url.Values{
		"template":       {"json"},
		"@Authorization": {"frend"},
		"$context":       {"inside joke"},
	}, query)
}
