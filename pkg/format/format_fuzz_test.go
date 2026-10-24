package format

import (
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// fuzzConfig is a service config with a keyed field of each kind a service
// config can declare, each with a valid default.
type fuzzConfig struct {
	// Text is a string with an alias key and a deprecated key.
	Text string `default:"text" deprecatedkeys:"oldtext" key:"text,alias"`
	// Number is a decimal int.
	Number int `default:"0" key:"number"`
	// Hex is an int written in base 16.
	Hex int `base:"16" default:"0x10" key:"hex"`
	// Small is an int8.
	Small int8 `default:"0" key:"small"`
	// Byte is a uint8.
	Byte uint8 `default:"0" key:"byte"`
	// Unsigned is a uint written in base 16.
	Unsigned uint `base:"16" default:"0xff" key:"unsigned"`
	// Flag is a bool.
	Flag bool `default:"no" key:"flag"`
	// Timeout is a duration.
	Timeout time.Duration `default:"10s" key:"timeout"`
	// Level is an enum.
	Level int `default:"None" key:"level"`
	// List is a comma-separated string slice.
	List []string `default:"a,b" key:"list"`
	// Pipes is a string slice separated by pipes.
	Pipes []string `default:"a" key:"pipes" sep:"|"`
	// Labels is a map of strings.
	Labels map[string]string `default:"a:b" key:"labels"`
	// Counts is a map of ints.
	Counts map[string]int `default:"a:1" key:"counts"`
	// Prop is a config prop struct.
	Prop subPropStruct `default:"@x" key:"prop"`
	// Props is a slice of config prop structs.
	Props []subPropStruct `default:"@x" key:"props"`
}

// Enums returns the enum formatter of the Level field.
//
// Returns:
//   - map[string]types.EnumFormatter: the enum formatters by field name.
func (c *fuzzConfig) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{"Level": testEnum}
}

// GetURL returns an empty URL. The fuzz targets do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (c *fuzzConfig) GetURL() *url.URL {
	return &url.URL{}
}

// SetURL ignores the URL. The fuzz targets do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (c *fuzzConfig) SetURL(*url.URL) error {
	return nil
}

// FuzzPropKeyResolver checks that a value set through a key reads back in a form
// that sets the same value again, and that the config survives a round trip
// through [BuildQuery] and [SetConfigPropsFromQuery] into a config with default
// values. The fuzzer picks the key by index and supplies its value, and
// deprecated keys are among the keys. The seed corpus covers each field kind
// and a deprecated key with valid values, including hex prefixes, negative
// numbers, bool spellings, enum names in another case, and separators inside
// map and slice values, as well as invalid ones.
func FuzzPropKeyResolver(f *testing.F) {
	keyResolver := NewPropKeyResolver(&fuzzConfig{})
	keys := keyResolver.QueryFields()
	require.NotEmpty(f, keys, "the fuzz config has no keys")

	for _, seed := range []struct {
		key, value string
	}{
		{"text", "hello world"},
		{"alias", "a=b&c"},
		{"oldtext", "legacy"},
		{"number", "-42"},
		{"hex", "#ff"},
		{"hex", "-1a"},
		{"small", "-128"},
		{"byte", "0x100"},
		{"unsigned", "0xFFFFFFFF"},
		{"flag", "Yes"},
		{"timeout", "1h30m"},
		{"level", "foo"},
		{"list", "a,,b"},
		{"pipes", "a,b|c"},
		{"labels", "x:y:z,w:"},
		{"counts", "a:0x10,b:-3"},
		{"prop", "@value"},
		{"props", "@a,@b"},
	} {
		index := slices.Index(keys, seed.key)
		require.GreaterOrEqual(f, index, 0, "seed key %q is not a config key", seed.key)

		f.Add(uint8(index), seed.value)
	}

	f.Fuzz(func(t *testing.T, index uint8, value string) {
		config := &fuzzConfig{}
		resolver := NewPropKeyResolver(config)
		require.NoError(t, resolver.SetDefaultProps(config))

		key := keys[int(index)%len(keys)]
		if resolver.Set(key, value) != nil {
			return
		}

		got, err := resolver.Get(key)
		require.NoError(t, err)
		require.NoError(t, resolver.Set(key, got), "the value %q read back from %q does not set it again", got, key)

		again, err := resolver.Get(key)
		require.NoError(t, err)
		assert.Equal(t, got, again)

		query, err := url.ParseQuery(BuildQuery(&resolver))
		require.NoError(t, err)

		rebuilt := &fuzzConfig{}
		rebuiltResolver := NewPropKeyResolver(rebuilt)
		require.NoError(t, rebuiltResolver.SetDefaultProps(rebuilt))

		rest, err := SetConfigPropsFromQuery(&rebuiltResolver, query)
		require.NoError(t, err)
		assert.Empty(t, rest)

		for _, key := range keys {
			want, err := resolver.Get(key)
			require.NoError(t, err)

			got, err := rebuiltResolver.Get(key)
			require.NoError(t, err)
			assert.Equal(t, want, got, "key %q after the query round trip", key)
		}
	})
}
