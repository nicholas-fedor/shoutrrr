package twilio

import (
	"net/url"
	"slices"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config for the Twilio SMS notification service.
type Config struct {
	// AccountSID is the Twilio Account SID. It is the user of the service URL.
	AccountSID string `desc:"Twilio Account SID" required:"" url:"user"`
	// AuthToken is the Twilio Auth Token. It is the password of the service
	// URL.
	AuthToken string `desc:"Twilio Auth Token" required:"" sensitive:"true" url:"password"`
	// FromNumber is the sender, taken from the host of the service URL. A
	// value starting with "MG" is a Messaging Service SID. Any other value is
	// a phone number.
	FromNumber string `desc:"Sender phone number or Messaging Service SID" required:"" url:"host"`
	// Title is prepended to the message on its own line when it is not empty.
	Title string `default:"" desc:"Notification title" key:"title" optional:""`
	// ToNumbers holds the recipient phone numbers, one per segment of the
	// service URL path.
	ToNumbers []string `desc:"Recipient phone number(s)" required:"" url:"path"`
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "twilio"

// phoneReplacer strips common formatting characters from phone numbers.
var phoneReplacer = strings.NewReplacer(
	" ", "",
	"-", "",
	"(", "",
	")", "",
	".", "",
)

// Enums returns the fields that should use a corresponding EnumFormatter to Print/Parse their values.
//
// Returns:
//   - map[string]types.EnumFormatter: an empty map, since Twilio has no enum
//     fields.
func (*Config) Enums() map[string]types.EnumFormatter {
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
//   - error: the error from [Config.setURL].
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
//   - *url.URL: the service URL, with the recipients as path segments.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	path := "/" + strings.Join(c.ToNumbers, "/")

	return &url.URL{
		Scheme:     Scheme,
		User:       url.UserPassword(c.AccountSID, c.AuthToken),
		Host:       c.FromNumber,
		Path:       path,
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
//     query prop error, or a validation error from [Config.validate].
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.AccountSID = serviceURL.User.Username()

	password, _ := serviceURL.User.Password()
	c.AuthToken = password

	c.FromNumber = normalizePhoneNumber(serviceURL.Host)
	c.ToNumbers = parseToNumbers(serviceURL.Path)

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.validate()
}

// validate checks that all required Config fields are present and consistent.
//
// Returns:
//   - error: the error for the first missing field, or [ErrToFromNumberSame]
//     when a recipient is the sender's phone number. It is nil for a valid
//     config.
func (c *Config) validate() error {
	if c.AccountSID == "" {
		return ErrAccountSIDMissing
	}

	if c.AuthToken == "" {
		return ErrAuthTokenMissing
	}

	if c.FromNumber == "" {
		return ErrFromNumberMissing
	}

	if len(c.ToNumbers) == 0 {
		return ErrToNumbersMissing
	}

	// Twilio rejects calls/messages where To == From.
	if !strings.HasPrefix(c.FromNumber, msgServicePrefix) {
		if slices.Contains(c.ToNumbers, c.FromNumber) {
			return ErrToFromNumberSame
		}
	}

	return nil
}

// normalizePhoneNumber strips common formatting characters (spaces, dashes,
// parentheses, dots) from a phone number string. Other characters are kept.
// Messaging Service SIDs (starting with "MG") are returned as-is.
//
// Parameters:
//   - number: the phone number or Messaging Service SID.
//
// Returns:
//   - string: the number without formatting characters, or the unchanged SID.
func normalizePhoneNumber(number string) string {
	if strings.HasPrefix(number, msgServicePrefix) {
		return number
	}

	return phoneReplacer.Replace(number)
}

// parseToNumbers extracts and normalizes recipient phone numbers from the URL path.
//
// Parameters:
//   - path: the URL path, with one recipient per segment.
//
// Returns:
//   - []string: the normalized numbers without empty segments, or nil for an
//     empty path.
func parseToNumbers(path string) []string {
	rawPath := strings.TrimPrefix(path, "/")
	if rawPath == "" {
		return nil
	}

	parts := strings.Split(rawPath, "/")
	numbers := make([]string, 0, len(parts))

	for _, p := range parts {
		n := normalizePhoneNumber(strings.TrimSpace(p))
		if n != "" {
			numbers = append(numbers, n)
		}
	}

	return numbers
}
