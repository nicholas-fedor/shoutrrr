package pushover

import (
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config for the Pushover notification service.
type Config struct {
	// Token is the application's API token, written as the password of the service URL.
	Token string `desc:"API Token/Key" sensitive:"true" url:"pass"`
	// User is the user or group key to send to, written as the host of the service URL.
	User string `desc:"User Key" sensitive:"true" url:"host"`
	// Title is the message title. Pushover shows the application's name when it is empty.
	Title string `desc:"Notification title, optionally set by the sender" key:"title" optional:""`
	// EncryptionKey is the AES-256 key, as 64 hex characters, for end-to-end
	// encryption of the message and title. Empty sends them unencrypted.
	EncryptionKey string `desc:"256-bit AES key as 64 hex characters for end-to-end encryption" key:"encryptionkey,key" optional:"" sensitive:"true"`
	// Devices lists the devices to send to. Empty sends to all of the user's devices.
	Devices []string `desc:"The devices to send to, or empty for all of the user's devices" key:"devices" optional:""`
	// Priority is the message priority, from -2 (lowest) to 1 (high). Emergency
	// priority 2 needs retry settings, so it is left out of the request.
	Priority int8 `default:"0" desc:"Message priority from -2 (lowest) to 1 (high)" key:"priority"`
	// Timestamp is the Unix time, in seconds, that Pushover shows and orders the
	// message by instead of when it received the message. 0 leaves it to Pushover.
	Timestamp int64 `default:"0" desc:"Unix time in seconds to show instead of when Pushover received the message" key:"timestamp"`
}

// Service identity and URL masking.
const (
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "pushover"

	// redactedPlaceholder replaces secrets when a Pushover URL is masked.
	redactedPlaceholder = "REDACTED"
)

// Enums returns the fields that should use a corresponding EnumFormatter to Print/Parse their values.
//
// Returns:
//   - map[string]types.EnumFormatter: an empty map, as no field is an enum.
func (c *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL returns a URL representation of its current field values.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL representation of its field values.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrUserMissing].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the token as its password and the user
//     key as its host.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       url.UserPassword("Token", c.Token),
		Host:       c.User,
		Scheme:     Scheme,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query, a
//     query prop error, an encryption key error, [ErrInvalidTimestamp] for a
//     negative timestamp, or [ErrUserMissing] or [ErrTokenMissing] when the
//     user or the token is missing.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	password, _ := serviceURL.User.Password()
	c.User = serviceURL.Host
	c.Token = password

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	if _, err := parseEncryptionKey(c.EncryptionKey); err != nil {
		return err
	}

	if c.Timestamp < 0 {
		return ErrInvalidTimestamp
	}

	if len(c.User) < 1 {
		return ErrUserMissing
	}

	if len(c.Token) < 1 {
		return ErrTokenMissing
	}

	return nil
}

// MaskURL redacts the API token in URL userinfo and sensitive query parameters.
//
// Parameters:
//   - parsedURL: The Pushover URL to modify.
func MaskURL(parsedURL *url.URL) {
	if parsedURL == nil {
		return
	}

	if parsedURL.User != nil {
		parsedURL.User = url.UserPassword(parsedURL.User.Username(), redactedPlaceholder)
	}

	queryParams := parsedURL.Query()
	if queryParams.Get("token") != "" {
		queryParams.Set("token", redactedPlaceholder)
	}

	if queryParams.Get("user") != "" {
		queryParams.Set("user", redactedPlaceholder)
	}

	if queryParams.Get("encryptionkey") != "" {
		queryParams.Set("encryptionkey", redactedPlaceholder)
	}

	if queryParams.Get("key") != "" {
		queryParams.Set("key", redactedPlaceholder)
	}

	parsedURL.RawQuery = queryParams.Encode()
}
