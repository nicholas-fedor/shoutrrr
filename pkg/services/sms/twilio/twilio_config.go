package twilio

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config for the Twilio SMS notification service.
type Config struct {
	// AccountSID is the Twilio Account SID. It is the user of the service URL.
	AccountSID string `desc:"Twilio Account SID" required:"" url:"user"`
	// AuthToken is the Twilio Auth Token, or the API key secret when APIKey
	// is set. It is the password of the service URL.
	AuthToken string `desc:"Twilio Auth Token, or the API key secret with apikey" required:"" sensitive:"true" url:"password"`
	// APIKey is the SID of a Twilio API key, SK followed by 32 hexadecimal
	// characters. When it is set, requests authenticate with the API key and
	// AuthToken as its secret, and the account SID still names the account.
	APIKey string `default:"" desc:"API key SID, which authenticates with the auth token as its secret" key:"apikey"`
	// FromNumber is the sender, taken from the host of the service URL. A
	// Messaging Service SID, MG followed by 32 hexadecimal characters, is sent
	// as the Messaging Service. Any other value, such as a phone number, a
	// short code, or an alphanumeric sender ID, is sent as the sender.
	FromNumber string `desc:"Sender phone number, alphanumeric sender ID, or Messaging Service SID" required:"" url:"host"`
	// Title is prepended to the message on its own line when it is not empty.
	Title string `default:"" desc:"Notification title" key:"title" optional:""`
	// ToNumbers holds the recipient phone numbers, one per segment of the
	// service URL path.
	ToNumbers []string `desc:"Recipient phone number(s)" required:"" url:"path"`
}

// Service identity and SID formats.
const (
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "twilio"
	// accountSIDPrefix starts every account SID.
	accountSIDPrefix = "AC"
	// apiKeySIDPrefix starts every API key SID.
	apiKeySIDPrefix = "SK"
	// msgServicePrefix starts every Messaging Service SID.
	msgServicePrefix = "MG"
	// sidLength is the length of a SID, a two-letter prefix and 32
	// hexadecimal characters.
	sidLength = 34
	// maxBodyLength is the longest message body Twilio accepts, in characters.
	maxBodyLength = 1600
)

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
	// Each recipient is one escaped segment, as the path is read.
	path, rawPath := util.JoinEscapedPath(c.ToNumbers)

	return &url.URL{
		Scheme:     Scheme,
		User:       url.UserPassword(c.AccountSID, c.AuthToken),
		Host:       c.FromNumber,
		Path:       path,
		RawPath:    rawPath,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// messageBody returns the body of a message, which puts the title on its own
// line before the message. It checks the body against what Twilio accepts.
//
// Parameters:
//   - message: the notification message.
//
// Returns:
//   - string: the message body.
//   - error: [ErrMessageMissing] for an empty body, or [ErrMessageTooLong] for
//     a body over 1600 characters.
func (c *Config) messageBody(message string) (string, error) {
	body := util.TitledMessage(c.Title, message)
	if body == "" {
		return "", ErrMessageMissing
	}

	// A body of at most 1600 bytes holds at most 1600 characters.
	if len(body) > maxBodyLength {
		if count := utf8.RuneCountInString(body); count > maxBodyLength {
			return "", fmt.Errorf("%w: %d characters", ErrMessageTooLong, count)
		}
	}

	return body, nil
}

// prepare strips the formatting from the sender and the recipients and drops
// the empty recipients, then checks that all required Config fields are
// present and consistent.
//
// Reading a service URL and each send run it, so a config built in code texts
// the recipients its URL would, and fails before a request as its URL would.
//
// Returns:
//   - error: the error for the first missing field, or [ErrToFromNumberSame]
//     when a recipient is the sender's phone number. An account SID in another
//     format gives [ErrInvalidAccountSID], and an API key in another format
//     gives [ErrInvalidAPIKey]. A sender that a URL host cannot hold gives an
//     error wrapping [util.ErrInvalidHost]. It is nil for a valid config.
func (c *Config) prepare() error {
	c.FromNumber = normalizeSender(c.FromNumber)
	c.ToNumbers = normalizeRecipients(c.ToNumbers)

	if c.AccountSID == "" {
		return ErrAccountSIDMissing
	}

	// The account SID is a segment of the request path, so its format also
	// keeps the request on the account's endpoint.
	if !isSID(c.AccountSID, accountSIDPrefix) {
		return ErrInvalidAccountSID
	}

	if c.APIKey != "" && !isSID(c.APIKey, apiKeySIDPrefix) {
		return ErrInvalidAPIKey
	}

	if c.AuthToken == "" {
		return ErrAuthTokenMissing
	}

	if c.FromNumber == "" {
		return ErrFromNumberMissing
	}

	if err := util.ValidateHostToken(c.FromNumber); err != nil {
		return fmt.Errorf("sender: %w", err)
	}

	if len(c.ToNumbers) == 0 {
		return ErrToNumbersMissing
	}

	// Twilio rejects a message whose recipient is its sender.
	if !c.usesMessagingService() && slices.Contains(c.ToNumbers, c.FromNumber) {
		return ErrToFromNumberSame
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
//     [ErrToNumbersMissing].
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	c.AccountSID = serviceURL.User.Username()

	password, _ := serviceURL.User.Password()
	c.AuthToken = password

	c.FromNumber = serviceURL.Host
	c.ToNumbers = parseToNumbers(serviceURL.EscapedPath())

	query, err := util.ParseURLQuery(serviceURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	return c.prepare()
}

// usesMessagingService reports whether the sender is a Messaging Service SID.
//
// Returns:
//   - bool: true when the sender is a Messaging Service SID.
func (c *Config) usesMessagingService() bool {
	return isSID(c.FromNumber, msgServicePrefix)
}

// isSID reports whether value is a Twilio SID with the given prefix, the prefix
// followed by 32 hexadecimal characters.
//
// Parameters:
//   - value: the value to check.
//   - prefix: the two-letter prefix of the SID type, such as AC.
//
// Returns:
//   - bool: true when value has the SID format.
func isSID(value, prefix string) bool {
	return len(value) == sidLength && strings.HasPrefix(value, prefix) && util.IsHex(value[len(prefix):])
}

// normalizeSender strips common formatting characters (spaces, dashes,
// parentheses, dots) from a sender phone number or short code. A sender that
// holds a letter, such as an alphanumeric sender ID or a Messaging Service
// SID, is returned as written.
//
// Parameters:
//   - sender: the sender.
//
// Returns:
//   - string: the sender without formatting characters, or the unchanged
//     sender when it holds a letter.
func normalizeSender(sender string) string {
	if strings.ContainsFunc(sender, unicode.IsLetter) {
		return sender
	}

	return phoneReplacer.Replace(sender)
}

// normalizeRecipients strips spaces around each recipient number and common
// formatting characters within it, and drops the numbers left empty. It never
// writes to numbers, since a send's config shares the stored config's slice.
//
// Parameters:
//   - numbers: the recipient numbers.
//
// Returns:
//   - []string: numbers itself when no number changes, or a new slice.
func normalizeRecipients(numbers []string) []string {
	var result []string

	for i, number := range numbers {
		normalized := phoneReplacer.Replace(strings.TrimSpace(number))

		if result == nil {
			if normalized == number && normalized != "" {
				continue
			}

			result = make([]string, i, len(numbers))
			copy(result, numbers[:i])
		}

		if normalized != "" {
			result = append(result, normalized)
		}
	}

	if result == nil {
		return numbers
	}

	return result
}

// parseToNumbers splits the escaped URL path into recipient numbers, one per
// segment. An escaped slash stays in its segment, so it never splits one
// number into two.
//
// Parameters:
//   - escapedPath: the escaped URL path, with one recipient per segment.
//
// Returns:
//   - []string: the decoded segments, or nil for an empty path.
func parseToNumbers(escapedPath string) []string {
	trimmed := strings.TrimPrefix(escapedPath, "/")
	if trimmed == "" {
		return nil
	}

	return util.SplitEscapedPath(trimmed)
}
