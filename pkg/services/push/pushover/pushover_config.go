package pushover

import (
	"fmt"
	"net/url"
	"time"

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
	// Priority is the message priority, from -2 (lowest) to 2 (emergency).
	//
	// Pushover repeats an emergency notification every Retry until the user
	// acknowledges it or Expire passes. It stops after 50 repeats, even when
	// Expire has not passed.
	Priority int8 `default:"0" desc:"Message priority from -2 (lowest) to 2 (emergency)" key:"priority"`
	// Timestamp is the Unix time, in seconds, that Pushover shows and orders the
	// message by instead of when it received the message. 0 leaves it to Pushover.
	Timestamp int64 `default:"0" desc:"Unix time in seconds to show instead of when Pushover received the message" key:"timestamp"`
	// Retry is how often Pushover repeats an emergency notification.
	//
	// It is whole seconds, at least 30 seconds. Zero uses 15 minutes.
	Retry time.Duration `desc:"Time between repeats of an emergency notification, at least 30s (default 15m)" key:"retry" optional:""`
	// Expire is how long Pushover keeps repeating an emergency notification.
	//
	// It is whole seconds, at most 3 hours. Zero uses 1 hour.
	Expire time.Duration `desc:"How long an emergency notification repeats, at most 3h (default 1h)" key:"expire" optional:""`
}

// Priorities and emergency settings the Message API accepts.
const (
	// priorityLowest is the lowest priority, which sends no notification.
	priorityLowest int8 = -2
	// priorityEmergency is the emergency priority, which repeats the
	// notification until the user acknowledges it.
	priorityEmergency int8 = 2
	// defaultRetry is the time between repeats of an emergency notification
	// when Retry is zero.
	defaultRetry = 15 * time.Minute
	// defaultExpire is how long an emergency notification repeats when Expire
	// is zero.
	defaultExpire = time.Hour
	// minRetry is the shortest time between repeats that the API accepts.
	minRetry = 30 * time.Second
	// maxExpire is the longest time a notification may repeat that the API
	// accepts.
	maxExpire = 3 * time.Hour
)

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

// prepare drops the empty devices and checks the config.
//
// The checks cover the encryption key, the timestamp, the priority and its
// emergency settings, the user, and the token. Reading a service URL and each
// send run it, so a config built in code sends to the devices its URL would,
// and fails before a request as its URL would.
//
// Returns:
//   - error: an encryption key error or [ErrInvalidTimestamp] for a negative
//     timestamp. An error wrapping [ErrInvalidPriority], [ErrInvalidRetry], or
//     [ErrInvalidExpire] for a value the Message API rejects. [ErrUserMissing]
//     or [ErrTokenMissing] when the user or the token is missing. An error
//     wrapping [util.ErrInvalidHost] for a user key that a URL host cannot
//     hold. Otherwise nil.
func (c *Config) prepare() error {
	c.Devices = util.NonEmpty(c.Devices)

	if _, err := parseEncryptionKey(c.EncryptionKey); err != nil {
		return err
	}

	if c.Timestamp < 0 {
		return ErrInvalidTimestamp
	}

	if c.Priority < priorityLowest || c.Priority > priorityEmergency {
		return fmt.Errorf("%w: %d", ErrInvalidPriority, c.Priority)
	}

	if c.Retry != 0 && (c.Retry < minRetry || c.Retry%time.Second != 0) {
		return fmt.Errorf("%w: %s", ErrInvalidRetry, c.Retry)
	}

	if c.Expire != 0 && (c.Expire < 0 || c.Expire > maxExpire || c.Expire%time.Second != 0) {
		return fmt.Errorf("%w: %s", ErrInvalidExpire, c.Expire)
	}

	if c.User == "" {
		return ErrUserMissing
	}

	if err := util.ValidateHostToken(c.User); err != nil {
		return fmt.Errorf("user key: %w", err)
	}

	if c.Token == "" {
		return ErrTokenMissing
	}

	return nil
}

// setURL updates the Config from a URL using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that sets the config props from the query.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: an error wrapping [util.ErrInvalidQuery] for a malformed query, a
//     query prop error, or the error from the config's checks, such as
//     [ErrUserMissing].
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

	return c.prepare()
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
