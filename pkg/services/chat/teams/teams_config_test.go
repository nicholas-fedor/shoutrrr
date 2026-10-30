package teams

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingResolver is a config resolver whose Set always returns err.
type failingResolver struct {
	// err is the error that Set returns.
	err error
}

// testWorkflowURL is a valid Logic Apps workflow webhook URL whose query
// carries a signature that must never appear in errors.
const testWorkflowURL = "https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke" +
	"?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX"

// testServiceURL is the Teams service URL that carries testWorkflowURL as its host.
var testServiceURL = "teams://?host=" + url.QueryEscape(testWorkflowURL)

// errResolverSet is the error that failingResolver returns from Set.
var errResolverSet = errors.New("resolver set failed")

// Get returns an empty value.
//
// Parameters:
//   - key: the config key, which is ignored.
//
// Returns:
//   - string: always empty.
//   - error: always nil.
func (failingResolver) Get(string) (string, error) {
	return "", nil
}

// QueryFields returns the title key, so the title query value reaches Set.
//
// Returns:
//   - []string: the title key.
func (failingResolver) QueryFields() []string {
	return []string{"title"}
}

// Set returns the resolver's error.
//
// Parameters:
//   - key: the config key, which is ignored.
//   - value: the value, which is ignored.
//
// Returns:
//   - error: the resolver's error.
func (r failingResolver) Set(string, string) error {
	return r.err
}

// mustParseURL parses rawURL and fails the test if it is invalid.
//
// Parameters:
//   - t: the test.
//   - rawURL: the URL to parse.
//
// Returns:
//   - *url.URL: the parsed URL.
func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	return parsed
}

// TestConfigSetURL verifies the config fields that SetURL reads from a
// service URL, including that unknown and empty query values are skipped.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the config that SetURL should produce.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{
			name:   "host only",
			rawURL: testServiceURL,
			want:   Config{Host: testWorkflowURL},
		},
		{
			name:   "host, title, and color",
			rawURL: testServiceURL + "&title=Test&color=red",
			want:   Config{Host: testWorkflowURL, Title: "Test", Color: "red"},
		},
		{
			name:   "unknown workflow query params are skipped",
			rawURL: testServiceURL + "&api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX",
			want:   Config{Host: testWorkflowURL},
		},
		{
			name:   "empty values are skipped",
			rawURL: testServiceURL + "&title=&color=",
			want:   Config{Host: testWorkflowURL},
		},
		{
			name:   "no query",
			rawURL: "teams://",
			want:   Config{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(mustParseURL(t, tt.rawURL)))
			assert.Equal(t, tt.want, *config)
		})
	}
}

// TestConfigSetURLResetsFields verifies that SetURL clears fields that the
// new service URL does not set.
func TestConfigSetURLResetsFields(t *testing.T) {
	t.Parallel()

	config := &Config{Host: "https://old.logic.azure.com/workflows/old", Title: "Old", Color: "red"}
	require.NoError(t, config.SetURL(mustParseURL(t, testServiceURL)))

	assert.Equal(t, Config{Host: testWorkflowURL}, *config)
}

// TestConfigSetURLResolverError verifies that setURL returns the resolver's
// error for a config key, naming the key.
func TestConfigSetURLResolverError(t *testing.T) {
	t.Parallel()

	config := &Config{}
	err := config.setURL(failingResolver{err: errResolverSet}, mustParseURL(t, "teams://?title=Test"))

	require.ErrorIs(t, err, errResolverSet)
	assert.ErrorContains(t, err, `"title"`)
}

// TestConfigGetURL verifies that GetURL builds a teams URL whose query
// carries every field, and that SetURL reads it back to the same config.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	config := &Config{Host: testWorkflowURL, Title: "Test", Color: "red"}

	serviceURL := config.GetURL()
	assert.Equal(t, Scheme, serviceURL.Scheme)
	assert.Empty(t, serviceURL.Host)

	query := serviceURL.Query()
	assert.Equal(t, testWorkflowURL, query.Get("host"))
	assert.Equal(t, "Test", query.Get("title"))
	assert.Equal(t, "red", query.Get("color"))

	roundTrip := &Config{}
	require.NoError(t, roundTrip.SetURL(mustParseURL(t, serviceURL.String())))
	assert.Equal(t, *config, *roundTrip)
}
