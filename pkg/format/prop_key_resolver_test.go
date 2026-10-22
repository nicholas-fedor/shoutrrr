package format

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// titledConfig is a minimal service config that declares the reserved title key.
type titledConfig struct {
	standard.EnumlessConfig

	// Title is the notification title.
	Title string `default:"" key:"title"`
}

// spelledDefaultsConfig declares defaults spelled differently from how the
// formatter prints the values.
type spelledDefaultsConfig struct {
	// Enabled is a bool whose default is spelled in lowercase.
	Enabled bool `default:"yes" key:"enabled"`
	// Quiet is a bool whose default is spelled as false.
	Quiet bool `default:"false" key:"quiet"`
	// Level is an enum whose default is spelled in lowercase.
	Level int `default:"high" key:"level"`
	// Retries is an int whose default has a sign.
	Retries int `default:"+3" key:"retries"`
	// Name is a string whose default is compared as written.
	Name string `default:"Shoutrrr" key:"name"`
	// Broken has a default that does not parse.
	Broken int `default:"NaN" key:"broken"`
	// Plain has no default.
	Plain bool `key:"plain"`
	// Delay has a primary key and an alias.
	Delay string `key:"delay,in"`
}

// resolvingConfig is a service config that resolves its own query keys.
type resolvingConfig struct {
	standard.EnumlessConfig
}

// spelledLevels formats the Level field of spelledDefaultsConfig.
var spelledLevels = CreateEnumFormatter([]string{"Low", "High"})

// TestNewPropKeyResolver verifies that the resolver lists the keys of keyed
// fields, aliases included, in sorted order.
func TestNewPropKeyResolver(t *testing.T) {
	t.Parallel()

	resolver := NewPropKeyResolver(&spelledDefaultsConfig{})

	assert.Equal(t,
		[]string{"broken", "delay", "enabled", "in", "level", "name", "plain", "quiet", "retries"},
		resolver.QueryFields())
	assert.Equal(t, []string{"duration", "signed", "str", "testenum"}, func() []string {
		resolver := NewPropKeyResolver(&testStruct{})

		return resolver.QueryFields()
	}())
}

// TestGetConfigQueryResolver verifies that a config that resolves its own
// keys is used as its resolver, and that any other config gets a
// [PropKeyResolver].
func TestGetConfigQueryResolver(t *testing.T) {
	t.Parallel()

	own := &resolvingConfig{}
	assert.Same(t, own, GetConfigQueryResolver(own))

	resolver, ok := GetConfigQueryResolver(&testStruct{}).(*PropKeyResolver)
	require.True(t, ok)
	assert.Equal(t, []string{"duration", "signed", "str", "testenum"}, resolver.QueryFields())
}

// TestPropKeyResolverBind verifies that a bound resolver reads and writes the
// new config and leaves the original resolver's config alone.
func TestPropKeyResolverBind(t *testing.T) {
	t.Parallel()

	first := &testStruct{Str: "first"}
	second := &testStruct{Str: "second"}

	resolver := NewPropKeyResolver(first)
	bound := resolver.Bind(second)

	value, err := bound.Get("str")
	require.NoError(t, err)
	assert.Equal(t, "second", value)

	require.NoError(t, bound.Set("str", "changed"))
	assert.Equal(t, "changed", second.Str)
	assert.Equal(t, "first", first.Str)
}

// TestPropKeyResolverGet verifies that a value is read by its key in any case
// and that an unknown key fails with [ErrInvalidConfigKey].
func TestPropKeyResolverGet(t *testing.T) {
	t.Parallel()

	config := &testStruct{Str: "value", TestEnum: 1}
	resolver := NewPropKeyResolver(config)

	value, err := resolver.Get("STR")
	require.NoError(t, err)
	assert.Equal(t, "value", value)

	value, err = resolver.Get("testenum")
	require.NoError(t, err)
	assert.Equal(t, "Foo", value)

	_, err = resolver.Get("unknown")
	require.ErrorIs(t, err, ErrInvalidConfigKey)
}

// TestPropKeyResolverSet verifies that a value is set by its key in any case,
// that an invalid value fails with the parse error, and that an unknown key
// fails with [ErrInvalidConfigKey], listing the valid keys.
func TestPropKeyResolverSet(t *testing.T) {
	t.Parallel()

	config := &testStruct{}
	resolver := NewPropKeyResolver(config)

	require.NoError(t, resolver.Set("Signed", "7"))
	assert.Equal(t, 7, config.Signed)

	require.ErrorIs(t, resolver.Set("signed", "seven"), ErrParseIntFailed)
	assert.Equal(t, 7, config.Signed)

	err := resolver.Set("unknown", "x")
	require.ErrorIs(t, err, ErrInvalidConfigKey)
	require.ErrorContains(t, err, "valid keys: [duration signed str testenum]")
}

// TestPropKeyResolverIsDefault verifies that a value is compared with the
// default as the formatter prints it.
func TestPropKeyResolverIsDefault(t *testing.T) {
	t.Parallel()

	resolver := NewPropKeyResolver(&spelledDefaultsConfig{})

	// tests lists keys and values with whether each value is the default.
	tests := []struct {
		name  string
		key   string
		value string
		want  bool
	}{
		{name: "bool default in lowercase", key: "enabled", value: "Yes", want: true},
		{name: "bool other value", key: "enabled", value: "No", want: false},
		{name: "bool default spelled as false", key: "quiet", value: "No", want: true},
		{name: "enum default in lowercase", key: "level", value: "High", want: true},
		{name: "enum other value", key: "level", value: "Low", want: false},
		{name: "int default with a sign", key: "retries", value: "3", want: true},
		{name: "string default as written", key: "name", value: "Shoutrrr", want: true},
		{name: "string default in another case", key: "name", value: "shoutrrr", want: false},
		{name: "default that does not parse, as written", key: "broken", value: "NaN", want: true},
		{name: "default that does not parse, other value", key: "broken", value: "0", want: false},
		{name: "no default", key: "plain", value: "No", want: false},
		{name: "no default, empty value", key: "plain", value: "", want: true},
		{name: "unknown key", key: "unknown", value: "", want: false},
		{name: "key in another case", key: "Enabled", value: "Yes", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, resolver.IsDefault(tt.key, tt.value))
		})
	}

	var unbound PropKeyResolver

	unbound.keyFields = resolver.keyFields
	assert.False(t, unbound.IsDefault("retries", "3"))
}

// TestPropKeyResolverKeyIsPrimary verifies that only the first key of a field
// is its primary key, in any case.
func TestPropKeyResolverKeyIsPrimary(t *testing.T) {
	t.Parallel()

	resolver := NewPropKeyResolver(&spelledDefaultsConfig{})

	assert.True(t, resolver.KeyIsPrimary("delay"))
	assert.True(t, resolver.KeyIsPrimary("Delay"))
	assert.False(t, resolver.KeyIsPrimary("in"))
	assert.False(t, resolver.KeyIsPrimary("IN"))
	assert.False(t, resolver.KeyIsPrimary("unknown"))
}

// TestPropKeyResolverSetDefaultProps verifies that the defaults are set on the
// given config, or on the resolver's own config when none is given, and that
// a default that does not parse fails.
func TestPropKeyResolverSetDefaultProps(t *testing.T) {
	t.Parallel()

	config := &testStruct{Str: "changed", Signed: 3}
	resolver := NewPropKeyResolver(config)

	other := &testStruct{}
	require.NoError(t, resolver.SetDefaultProps(other))
	assert.Equal(t, testStruct{Str: "notempty", Duration: 10 * time.Second}, *other)
	assert.Equal(t, "changed", config.Str)

	require.NoError(t, resolver.SetDefaultProps(nil))
	assert.Equal(t, "notempty", config.Str)
	assert.Zero(t, config.Signed)

	bad := &testStructBadDefault{}
	badResolver := NewPropKeyResolver(bad)
	require.ErrorIs(t, badResolver.SetDefaultProps(bad), ErrParseIntFailed)
}

// TestPropKeyResolverSetDefaultPropsEmptyLists verifies that the empty default
// of a keyed slice or map clears it, rather than leaving one empty item in a
// slice or failing for a map.
func TestPropKeyResolverSetDefaultPropsEmptyLists(t *testing.T) {
	t.Parallel()

	config := &testStructListDefaults{
		Strs:  []string{"one"},
		Props: []subPropStruct{{Value: "two"}},
		Pairs: map[string]string{"a": "1"},
	}

	resolver := NewPropKeyResolver(config)
	require.NoError(t, resolver.SetDefaultProps(nil))
	assert.Nil(t, config.Strs)
	assert.Nil(t, config.Props)
	assert.Nil(t, config.Pairs)
}

// TestPropKeyResolverUpdateConfigFromParams verifies that params set the
// matching keys on the given config or the resolver's own, that the first
// invalid param is reported while the others still apply, and that nil params
// change nothing.
func TestPropKeyResolverUpdateConfigFromParams(t *testing.T) {
	t.Parallel()

	config := &testStruct{}
	resolver := NewPropKeyResolver(config)

	require.NoError(t, resolver.UpdateConfigFromParams(nil, &types.Params{"str": "newValue"}))
	assert.Equal(t, "newValue", config.Str)

	err := resolver.UpdateConfigFromParams(nil, &types.Params{"signed": "1", "b": "c", "str": "val"})
	require.ErrorIs(t, err, ErrInvalidConfigKey)
	assert.Equal(t, 1, config.Signed)
	assert.Equal(t, "val", config.Str)

	other := &testStruct{}
	require.NoError(t, resolver.UpdateConfigFromParams(other, &types.Params{"str": "other"}))
	assert.Equal(t, "other", other.Str)
	assert.Equal(t, "val", config.Str)

	require.NoError(t, resolver.UpdateConfigFromParams(nil, nil))
}

// TestPropKeyResolverReservedParams verifies that the reserved title,
// message, and level params are skipped when the config does not declare
// them, applied when it does, and that other unknown params are still
// reported.
func TestPropKeyResolverReservedParams(t *testing.T) {
	t.Parallel()

	config := &testStruct{}
	resolver := NewPropKeyResolver(config)

	require.NoError(t, resolver.UpdateConfigFromParams(nil, &types.Params{
		types.TitleKey: "title", types.MessageKey: "message", "Level": "warning", "str": "val",
	}))
	assert.Equal(t, "val", config.Str)

	titled := &titledConfig{}
	titledResolver := NewPropKeyResolver(titled)
	require.NoError(t, titledResolver.UpdateConfigFromParams(nil, &types.Params{types.TitleKey: "applied"}))
	assert.Equal(t, "applied", titled.Title)

	err := resolver.UpdateConfigFromParams(nil, &types.Params{types.TitleKey: "title", "titel": "typo"})
	require.ErrorIs(t, err, ErrInvalidConfigKey)
	require.ErrorContains(t, err, "titel")
}

// GetURL returns an empty URL. The resolver tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (c *titledConfig) GetURL() *url.URL { return &url.URL{} }

// SetURL ignores the URL. The resolver tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (c *titledConfig) SetURL(*url.URL) error { return nil }

// Enums returns the enum formatter of the Level field.
//
// Returns:
//   - map[string]types.EnumFormatter: the enum formatters by field name.
func (c *spelledDefaultsConfig) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{"Level": spelledLevels}
}

// GetURL returns an empty URL. The resolver tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (c *spelledDefaultsConfig) GetURL() *url.URL { return &url.URL{} }

// SetURL ignores the URL. The resolver tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (c *spelledDefaultsConfig) SetURL(*url.URL) error { return nil }

// Get reads no keys.
//
// Parameters:
//   - string: unused.
//
// Returns:
//   - string: always "".
//   - error: always [ErrInvalidConfigKey].
func (c *resolvingConfig) Get(string) (string, error) { return "", ErrInvalidConfigKey }

// GetURL returns an empty URL. The resolver tests do not use it.
//
// Returns:
//   - *url.URL: an empty URL.
func (c *resolvingConfig) GetURL() *url.URL { return &url.URL{} }

// QueryFields lists no keys.
//
// Returns:
//   - []string: always nil.
func (c *resolvingConfig) QueryFields() []string { return nil }

// Set writes no keys.
//
// Parameters:
//   - string: unused.
//   - string: unused.
//
// Returns:
//   - error: always [ErrInvalidConfigKey].
func (c *resolvingConfig) Set(string, string) error { return ErrInvalidConfigKey }

// SetURL ignores the URL. The resolver tests do not use it.
//
// Parameters:
//   - *url.URL: unused.
//
// Returns:
//   - error: always nil.
func (c *resolvingConfig) SetURL(*url.URL) error { return nil }
