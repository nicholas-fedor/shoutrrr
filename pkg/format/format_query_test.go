package format

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// errResolver is the error a mock resolver returns.
var errResolver = errors.New("resolver failed")

// newTestStructResolver returns a test config at its defaults and a resolver
// bound to it.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - *testStruct: the config.
//   - *PropKeyResolver: the resolver.
func newTestStructResolver(t *testing.T) (*testStruct, *PropKeyResolver) {
	t.Helper()

	config := &testStruct{}
	resolver := NewPropKeyResolver(config)
	require.NoError(t, resolver.SetDefaultProps(config))

	return config, &resolver
}

// TestApplyQuery verifies that ApplyQuery sets the config props from a URL
// query whose keys match in any case. One key in two spellings is rejected
// before any value is applied. Errors name the first failing key in sorted
// order, so they do not depend on map order. Unknown keys in two spellings are
// left to the resolver, and a deprecated key loses to its replacement.
func TestApplyQuery(t *testing.T) {
	t.Parallel()

	t.Run("keys in any case", func(t *testing.T) {
		t.Parallel()

		config, resolver := newTestStructResolver(t)

		require.NoError(t, ApplyQuery(resolver, url.Values{"Str": {"a"}, "SIGNED": {"2"}}))
		assert.Equal(t, "a", config.Str)
		assert.Equal(t, 2, config.Signed)
	})

	t.Run("one key in two spellings", func(t *testing.T) {
		t.Parallel()

		config, resolver := newTestStructResolver(t)
		before := *config

		err := ApplyQuery(resolver, url.Values{"str": {"a"}, "Str": {"b"}, "signed": {"3"}})
		require.ErrorIs(t, err, ErrConflictingKeys)
		require.ErrorContains(t, err, `"Str" and "str"`)
		assert.Equal(t, before, *config, "no value is applied")
	})

	t.Run("first invalid key in sorted order", func(t *testing.T) {
		t.Parallel()

		for range 20 {
			_, resolver := newTestStructResolver(t)

			err := ApplyQuery(resolver, url.Values{"zz": {"1"}, "aa": {"1"}, "mm": {"1"}})
			require.ErrorIs(t, err, ErrInvalidConfigKey)
			assert.ErrorContains(t, err, `setting query parameter "aa"`)
		}
	})

	t.Run("unknown key in two spellings", func(t *testing.T) {
		t.Parallel()

		_, resolver := newTestStructResolver(t)

		err := ApplyQuery(resolver, url.Values{"foo": {"1"}, "FOO": {"1"}})
		require.ErrorIs(t, err, ErrInvalidConfigKey)
		require.NotErrorIs(t, err, ErrConflictingKeys)
	})

	t.Run("deprecated key and its replacement", func(t *testing.T) {
		t.Parallel()

		for _, tt := range []struct {
			query url.Values
			want  bool
		}{
			{query: url.Values{"insecure": {"yes"}, "skipverify": {"no"}}, want: false},
			{query: url.Values{"insecure": {"no"}, "skipverify": {"yes"}}, want: true},
		} {
			config := &deprecatedConfig{}
			resolver := NewPropKeyResolver(config)
			require.NoError(t, resolver.SetDefaultProps(config))

			require.NoError(t, ApplyQuery(&resolver, tt.query))
			assert.Equal(t, tt.want, config.SkipVerify, tt.query)
		}
	})
}

// TestBuildQuery verifies that the query holds only the keyed values that
// differ from their defaults.
func TestBuildQuery(t *testing.T) {
	t.Parallel()

	config, resolver := newTestStructResolver(t)
	assert.Empty(t, BuildQuery(resolver))

	config.Str = "test"
	config.Duration = 30 * time.Second
	config.TestEnum = 2

	assert.Equal(t, "duration=30s&str=test&testenum=Bar", BuildQuery(resolver))
}

// TestBuildQueryZeroValueDefaults verifies that keyed fields without a default
// are left out of the query at their zero values, which SetDefaultProps sets,
// and are written once they hold another value.
func TestBuildQueryZeroValueDefaults(t *testing.T) {
	t.Parallel()

	config := &testStructNoDefaults{Duration: time.Second, Signed: -1, Unsigned: 1, Level: 2, Flag: true}
	resolver := NewPropKeyResolver(config)
	require.NoError(t, resolver.SetDefaultProps(nil))

	assert.Empty(t, BuildQuery(&resolver))

	config.Signed = 3
	config.Level = 1
	config.Flag = true

	assert.Equal(t, "flag=Yes&level=Foo&signed=3", BuildQuery(&resolver))
}

// TestBuildQueryAliases verifies that a value is written under its primary
// key only. The other fields hold their defaults, apart from Broken, whose
// default does not parse.
func TestBuildQueryAliases(t *testing.T) {
	t.Parallel()

	config := &spelledDefaultsConfig{Enabled: true, Level: 1, Retries: 3, Name: "Shoutrrr", Delay: "5s"}
	resolver := NewPropKeyResolver(config)

	assert.Equal(t, "broken=0&delay=5s", BuildQuery(&resolver))
}

// TestBuildQueryWithCustomFields verifies that a custom query value whose key
// is a config key is kept under the escaped key, and that other custom values
// are kept as they are.
func TestBuildQueryWithCustomFields(t *testing.T) {
	t.Parallel()

	config, resolver := newTestStructResolver(t)
	config.Str = "service"

	query := BuildQueryWithCustomFields(resolver, url.Values{"str": {"custom"}, "extra": {"kept"}})

	assert.Equal(t, url.Values{"__str": {"custom"}, "str": {"service"}, "extra": {"kept"}}, query)
}

// TestBuildQueryOtherResolver verifies that a resolver other than
// [PropKeyResolver] writes every field it lists, skipping fields it cannot
// read.
func TestBuildQueryOtherResolver(t *testing.T) {
	t.Parallel()

	resolver := mocks.NewMockConfigQueryResolver(t)
	resolver.EXPECT().QueryFields().Return([]string{"a", "b", "c"})
	resolver.EXPECT().Get("a").Return("1", nil)
	resolver.EXPECT().Get("b").Return("", errResolver)
	resolver.EXPECT().Get("c").Return("", nil)

	assert.Equal(t, "a=1&c=", BuildQuery(resolver))
}

// TestSetConfigPropsFromQuery verifies that query values for config keys are
// set and removed from the query, that escaped custom values come back under
// their key, and that other values are kept.
func TestSetConfigPropsFromQuery(t *testing.T) {
	t.Parallel()

	config, resolver := newTestStructResolver(t)

	query, err := SetConfigPropsFromQuery(resolver, url.Values{
		"str": {"service"}, "signed": {"5"}, "__str": {"custom"}, "extra": {"kept"},
	})

	require.NoError(t, err)
	assert.Equal(t, "service", config.Str)
	assert.Equal(t, 5, config.Signed)
	assert.Equal(t, url.Values{"str": {"custom"}, "extra": {"kept"}}, query)
}

// TestSetConfigPropsFromQueryErrors verifies that the error of the first
// invalid value in key order is reported while the other values are still
// set, and that a nil query sets nothing.
func TestSetConfigPropsFromQueryErrors(t *testing.T) {
	t.Parallel()

	config, resolver := newTestStructResolver(t)

	query, err := SetConfigPropsFromQuery(resolver, url.Values{"duration": {"long"}, "signed": {"many"}, "str": {"set"}})

	require.ErrorIs(t, err, ErrParseDurationFailed)
	require.NotErrorIs(t, err, ErrParseIntFailed)
	assert.Equal(t, "set", config.Str)
	assert.Empty(t, query)

	query, err = SetConfigPropsFromQuery(resolver, nil)
	require.NoError(t, err)
	assert.Equal(t, url.Values{}, query)
}

// TestEscapeKey verifies that escaping adds the key prefix and unescaping
// removes it.
func TestEscapeKey(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "__str", EscapeKey("str"))
	assert.Equal(t, "str", UnescapeKey("__str"))
	assert.Equal(t, "str", UnescapeKey("str"))
}
