package format

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// recordingConfig is a config that records the keys its URL sets. Priority is
// level-mapped, and the other fields are not.
type recordingConfig struct {
	URLKeys

	// Title is a string with a default.
	Title string `default:"Note" key:"title"`
	// Name is a string without a default.
	Name string `key:"name"`
	// Tag is another string without a default.
	Tag string `key:"tag"`
	// Priority is a level-mapped int with a primary key and an alias.
	Priority int `default:"0" key:"priority,prio" levelmapped:"true"`
	// SkipVerify is a bool with a deprecated key.
	SkipVerify bool `default:"No" deprecatedkeys:"insecure" key:"skipverify"`
}

// readRecordingConfig reads a config from a service URL through SetURL.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *recordingConfig: the config read from the URL.
func readRecordingConfig(t *testing.T, rawURL string) *recordingConfig {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	config := &recordingConfig{}
	require.NoError(t, config.SetURL(serviceURL))

	return config
}

// TestURLKeysKeepsDefault verifies that a key the URL set, in any case or
// through an alias or a deprecated key, does not keep its default. A key the
// URL leaves out does.
func TestURLKeysKeepsDefault(t *testing.T) {
	t.Parallel()

	// resolver checks the configs, as a service's resolver checks its sends.
	resolver := NewPropKeyResolver(&recordingConfig{})

	// tests lists service URLs with the key to check and whether it keeps its
	// default.
	tests := []struct {
		name   string
		rawURL string
		key    string
		want   bool
	}{
		{name: "left out", rawURL: "rec://host", key: "priority", want: true},
		{name: "set to the default", rawURL: "rec://host?priority=0", key: "priority", want: false},
		{name: "in another case", rawURL: "rec://host?PRIORITY=0", key: "priority", want: false},
		{name: "through an alias", rawURL: "rec://host?prio=0", key: "priority", want: false},
		{name: "through a deprecated key", rawURL: "rec://host?insecure=no", key: "skipverify", want: false},
		{name: "another key set", rawURL: "rec://host?title=Note", key: "priority", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := readRecordingConfig(t, tt.rawURL)

			assert.Equal(t, tt.want, resolver.KeepsDefault(config, nil, tt.key))
		})
	}
}

// TestURLKeysFollowTheConfig verifies that the record belongs to the config,
// not to the resolver that read the URL. A copy of the config shares the
// record, a config built in code has none, and the next URL read starts it
// anew.
func TestURLKeysFollowTheConfig(t *testing.T) {
	t.Parallel()

	config := &recordingConfig{}
	resolver := NewPropKeyResolver(config)
	require.NoError(t, ApplyQuery(&resolver, url.Values{"priority": {"0"}}))

	sendCopy := *config

	assert.False(t, resolver.KeepsDefault(config, nil, "priority"))
	assert.False(t, resolver.KeepsDefault(&sendCopy, nil, "priority"), "a copy of the config shares its record")
	assert.True(t, resolver.KeepsDefault(&recordingConfig{}, nil, "priority"),
		"a config built in code keeps its default after the resolver read a URL")

	require.NoError(t, ApplyQuery(&resolver, url.Values{"title": {"Note"}}))
	assert.True(t, resolver.KeepsDefault(config, nil, "priority"), "a new URL read starts the record anew")

	_, err := SetConfigPropsFromQuery(&resolver, url.Values{"prio": {"0"}, "custom": {"x"}})
	require.NoError(t, err)
	assert.True(t, resolver.KeepsDefault(config, nil, "title"), "a query with custom keys starts the record anew too")
	assert.False(t, resolver.KeepsDefault(config, nil, "priority"), "and records the config keys it sets")
}

// TestURLKeysRejectedQuery verifies that a query rejected before it sets a
// key, for one key in two spellings or a value that does not parse, leaves
// the record of the last URL as it is.
func TestURLKeysRejectedQuery(t *testing.T) {
	t.Parallel()

	// tests lists queries that are rejected before any key is set.
	tests := []struct {
		query   url.Values
		wantErr error
		name    string
	}{
		{name: "key in two spellings", query: url.Values{"title": {"x"}, "TITLE": {"y"}}, wantErr: ErrConflictingKeys},
		{name: "value that does not parse", query: url.Values{"priority": {"high"}}, wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := readRecordingConfig(t, "rec://host?priority=0")
			resolver := NewPropKeyResolver(config)

			err := ApplyQuery(&resolver, tt.query)
			require.Error(t, err)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			assert.False(t, resolver.KeepsDefault(config, nil, "priority"))
		})
	}
}

// TestURLKeysWrittenBack verifies that the URL a config writes keeps a
// level-mapped key its URL set, even at its default, and leaves out any other
// key at its default. The config read back from that URL then records the
// level-mapped key too.
func TestURLKeysWrittenBack(t *testing.T) {
	t.Parallel()

	// resolver checks the configs, as a service's resolver checks its sends.
	resolver := NewPropKeyResolver(&recordingConfig{})

	// tests lists service URLs with the query the config read from each
	// should write.
	tests := []struct {
		name      string
		rawURL    string
		wantQuery string
	}{
		{name: "level-mapped key at its default", rawURL: "rec://host?priority=0&title=Note", wantQuery: "priority=0"},
		{name: "level-mapped key through an alias", rawURL: "rec://host?prio=0", wantQuery: "priority=0"},
		{name: "other keys at their defaults", rawURL: "rec://host?title=Note&skipverify=no&name=", wantQuery: ""},
		{name: "values other than the defaults", rawURL: "rec://host?priority=2&title=Alert", wantQuery: "priority=2&title=Alert"},
		{name: "no keys", rawURL: "rec://host", wantQuery: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := readRecordingConfig(t, tt.rawURL)
			serviceURL := config.GetURL()
			assert.Equal(t, tt.wantQuery, serviceURL.RawQuery)

			readBack := readRecordingConfig(t, serviceURL.String())
			assert.Equal(t, resolver.KeepsDefault(config, nil, "priority"), resolver.KeepsDefault(readBack, nil, "priority"))
		})
	}

	assert.Empty(t, (&recordingConfig{Title: "Note"}).GetURL().RawQuery, "a config built in code at its defaults")
}

// TestURLKeysOnlyFromURLQueries verifies that only reading a URL query records
// keys. Set alone, as code that fills a config from elsewhere calls it,
// records nothing, and a copy of a config reads a URL into its own record.
func TestURLKeysOnlyFromURLQueries(t *testing.T) {
	t.Parallel()

	original := &recordingConfig{Title: "Note"}
	resolver := NewPropKeyResolver(original)

	require.NoError(t, resolver.Set("priority", "0"))
	assert.True(t, resolver.KeepsDefault(original, nil, "priority"), "Set alone records nothing")

	require.NoError(t, ApplyQuery(&resolver, url.Values{"priority": {"0"}}))

	readCopy := *original
	copyResolver := resolver.Bind(&readCopy)
	require.NoError(t, ApplyQuery(&copyResolver, url.Values{"title": {"Note"}}))

	assert.False(t, resolver.KeepsDefault(original, nil, "priority"))
	assert.True(t, resolver.KeepsDefault(original, nil, "title"))
	assert.True(t, resolver.KeepsDefault(&readCopy, nil, "priority"))
	assert.False(t, resolver.KeepsDefault(&readCopy, nil, "title"))
}

// TestURLKeysOf verifies that the record is reached through a config given
// by pointer, and that a config given by value, one without a record, or a
// resolver bound to no config has none to reach.
func TestURLKeysOf(t *testing.T) {
	t.Parallel()

	config := &recordingConfig{}
	resolver := NewPropKeyResolver(config)

	assert.Same(t, &config.URLKeys, urlKeysOf(config))
	assert.Same(t, &config.URLKeys, resolver.boundURLKeys())
	assert.Nil(t, urlKeysOf(recordingConfig{}))
	assert.Nil(t, urlKeysOf(&spelledDefaultsConfig{}))
	assert.Nil(t, urlKeysOf(nil))
	assert.Nil(t, (&PropKeyResolver{}).boundURLKeys())
}

// Enums reports no enum fields.
//
// Returns:
//   - map[string]types.EnumFormatter: an empty map.
func (c *recordingConfig) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL writes the config's query on a fixed host.
//
// Returns:
//   - *url.URL: the service URL.
func (c *recordingConfig) GetURL() *url.URL {
	resolver := NewPropKeyResolver(c)

	return &url.URL{Scheme: "rec", Host: "host", RawQuery: BuildQuery(&resolver)}
}

// SetURL applies the defaults and then the URL's query.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from applying the defaults or the query, or nil.
func (c *recordingConfig) SetURL(serviceURL *url.URL) error {
	resolver := NewPropKeyResolver(c)
	if err := resolver.SetDefaultProps(c); err != nil {
		return err
	}

	return ApplyQuery(&resolver, serviceURL.Query())
}
