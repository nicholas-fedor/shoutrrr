package format

import (
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// maskTestConfig is a config with sensitive fields of several kinds.
type maskTestConfig struct {
	// Token is a sensitive string.
	Token string `sensitive:"true"`
	// Empty is a sensitive string without a value.
	Empty string `sensitive:"true"`
	// Name is not sensitive.
	Name string
	// Count is a sensitive field that cannot hold the placeholder.
	Count int `sensitive:"true"`
}

// valueConfig is a config whose methods have value receivers.
type valueConfig struct {
	// Token is a sensitive string.
	Token string `sensitive:"true"`
}

// stringConfig is a config that is not a struct.
type stringConfig string

// selfMaskingConfig is a config that masks its own URL.
type selfMaskingConfig struct {
	maskTestConfig
}

// Enums returns no enum formatters.
//
// Returns:
//   - map[string]types.EnumFormatter: nil.
func (*maskTestConfig) Enums() map[string]types.EnumFormatter { return nil }

// GetURL returns the fields as a URL.
//
// Returns:
//   - *url.URL: the token as the user, the name as the host, and the other
//     fields in the query.
func (c *maskTestConfig) GetURL() *url.URL {
	return &url.URL{
		Scheme:   "mask",
		User:     url.User(c.Token),
		Host:     c.Name,
		RawQuery: url.Values{"count": {strconv.Itoa(c.Count)}, "empty": {c.Empty}}.Encode(),
	}
}

// SetURL does nothing.
//
// Returns:
//   - error: nil.
func (*maskTestConfig) SetURL(*url.URL) error { return nil }

// MaskedURL returns a fixed URL.
//
// Returns:
//   - *url.URL: the URL mask://self.
func (*selfMaskingConfig) MaskedURL() *url.URL { return &url.URL{Scheme: "mask", Host: "self"} }

// Enums returns no enum formatters.
//
// Returns:
//   - map[string]types.EnumFormatter: nil.
func (valueConfig) Enums() map[string]types.EnumFormatter { return nil }

// GetURL returns the token as the URL user.
//
// Returns:
//   - *url.URL: the URL.
func (c valueConfig) GetURL() *url.URL {
	return &url.URL{Scheme: "value", User: url.User(c.Token), Host: "host"}
}

// SetURL does nothing.
//
// Returns:
//   - error: nil.
func (valueConfig) SetURL(*url.URL) error { return nil }

// Enums returns no enum formatters.
//
// Returns:
//   - map[string]types.EnumFormatter: nil.
func (stringConfig) Enums() map[string]types.EnumFormatter { return nil }

// GetURL returns the string as the URL host.
//
// Returns:
//   - *url.URL: the URL.
func (c stringConfig) GetURL() *url.URL { return &url.URL{Scheme: "string", Host: string(c)} }

// SetURL does nothing.
//
// Returns:
//   - error: nil.
func (stringConfig) SetURL(*url.URL) error { return nil }

// TestMaskedURL verifies that a sensitive string with a value becomes
// [MaskedValue], an empty one stays empty, a sensitive field of another kind
// is cleared, other fields stay, and the config itself is not changed. It also
// covers a config passed by value, one that masks its own URL, and one that is
// not a struct and has no fields to mask.
func TestMaskedURL(t *testing.T) {
	t.Parallel()

	config := &maskTestConfig{Token: "secret", Name: "visible", Count: 7}

	assert.Equal(t, "mask://REDACTED@visible?count=0&empty=", MaskedURL(config).String())
	assert.Equal(t, &maskTestConfig{Token: "secret", Name: "visible", Count: 7}, config)

	assert.Equal(t, "value://REDACTED@host", MaskedURL(valueConfig{Token: "secret"}).String())

	assert.Equal(t, "mask://self", MaskedURL(&selfMaskingConfig{maskTestConfig: *config}).String())

	assert.Equal(t, "string://host", MaskedURL(stringConfig("host")).String())
}
