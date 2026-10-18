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

// TestBuildQueryAliases verifies that a value is written under its primary
// key only.
func TestBuildQueryAliases(t *testing.T) {
	t.Parallel()

	config := &spelledDefaultsConfig{Enabled: true, Level: 1, Retries: 3, Name: "Shoutrrr", Delay: "5s"}
	resolver := NewPropKeyResolver(config)

	assert.Equal(t, "broken=0&delay=5s&plain=No", BuildQuery(&resolver))
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
