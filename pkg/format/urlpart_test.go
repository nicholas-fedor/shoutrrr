package format

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseURLPart verifies that each URL part name parses in any case, that
// numbered paths after the first offset from [URLPath], and that an empty or
// unknown name is the query.
func TestParseURLPart(t *testing.T) {
	t.Parallel()

	// tests lists URL part names with the part each should parse to.
	tests := []struct {
		name string
		want URLPart
	}{
		{name: "user", want: URLUser},
		{name: "User", want: URLUser},
		{name: "pass", want: URLPassword},
		{name: "password", want: URLPassword},
		{name: "host", want: URLHost},
		{name: "HOST", want: URLHost},
		{name: "port", want: URLPort},
		{name: "path", want: URLPath},
		{name: "path1", want: URLPath},
		{name: "path2", want: URLPath + 1},
		{name: "path3", want: URLPath + 2},
		{name: "path4", want: URLPath + 3},
		{name: "query", want: URLQuery},
		{name: "", want: URLQuery},
		{name: "path0", want: URLQuery},
		{name: "pathx", want: URLQuery},
		{name: "bogus", want: URLQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ParseURLPart(tt.name))
		})
	}
}

// TestParseURLParts verifies that a comma-separated list parses to its parts
// in order.
func TestParseURLParts(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []URLPart{URLHost, URLPort}, ParseURLParts("host,port"))
	assert.Equal(t, []URLPart{URLUser, URLPassword}, ParseURLParts("user,pass"))
	assert.Equal(t, []URLPart{URLQuery}, ParseURLParts(""))
}

// TestURLPartSuffix verifies the character that follows each URL part.
func TestURLPartSuffix(t *testing.T) {
	t.Parallel()

	assert.Equal(t, '/', URLQuery.Suffix())
	assert.Equal(t, ':', URLUser.Suffix())
	assert.Equal(t, '@', URLPassword.Suffix())
	assert.Equal(t, ':', URLHost.Suffix())
	assert.Equal(t, '/', URLPort.Suffix())
	assert.Equal(t, '/', URLPath.Suffix())
	assert.Equal(t, '/', (URLPath + 1).Suffix())
}

// TestURLPartString verifies the name of each URL part, and the fallback for
// a value without one.
func TestURLPartString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Query", URLQuery.String())
	assert.Equal(t, "Password", URLPassword.String())
	assert.Equal(t, "Path", URLPath.String())
	assert.Equal(t, "URLPart(6)", (URLPath + 1).String())
	assert.Equal(t, "URLPart(-1)", URLPart(-1).String())
}
