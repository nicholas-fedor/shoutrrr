package twilio

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "twilio"

// Static errors for configuration validation.
var (
	ErrAccountSIDMissing = errors.New("account SID missing from config URL")
	ErrAuthTokenMissing  = errors.New("auth token missing from config URL")
	ErrFromNumberMissing = errors.New(
		"from number or messaging service SID missing from config URL",
	)
	ErrToFromNumberSame = errors.New("to and from phone numbers must not be the same")
	ErrToNumbersMissing = errors.New("recipient phone number(s) missing from config URL")
)

// Config for the Twilio SMS notification service.
type Config struct {
	AccountSID string   `desc:"Twilio Account SID"                           required:"" url:"user"`
	AuthToken  string   `desc:"Twilio Auth Token"                            required:"" url:"password"`
	FromNumber string   `desc:"Sender phone number or Messaging Service SID" required:"" url:"host"`
	ToNumbers  []string `desc:"Recipient phone number(s)"                    required:"" url:"path"`
	Title      string   `desc:"Notification title"                                                      default:"" key:"title" optional:""`
}

// Enums returns the fields that should use a corresponding EnumFormatter to Print/Parse their values.
func (*Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{}
}

// GetURL returns a URL representation of its current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates the Config from a URL representation of its field values.
func (c *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, url)
}

// setURL updates the Config from a URL using the provided resolver.
func (c *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	c.AccountSID = url.User.Username()

	password, _ := url.User.Password()
	c.AuthToken = password

	c.FromNumber = normalizePhoneNumber(url.Host)
	c.ToNumbers = parseToNumbers(url.Path)

	for key, vals := range url.Query() {
		err := resolver.Set(key, vals[0])
		if err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	if url.String() != "twilio://dummy@dummy.com" {
		return c.validate()
	}

	return nil
}

// parseToNumbers extracts and normalizes recipient phone numbers from the URL path.
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

// validate checks that all required Config fields are present and consistent.
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

// getURL constructs a URL from the Config's fields using the provided resolver.
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

// phoneReplacer strips common formatting characters from phone numbers.
var phoneReplacer = strings.NewReplacer(
	" ", "",
	"-", "",
	"(", "",
	")", "",
	".", "",
)

// normalizePhoneNumber strips common formatting characters (spaces, dashes,
// parentheses, dots) from a phone number string, leaving only digits and a
// leading '+'. Messaging Service SIDs (starting with "MG") are returned as-is.
func normalizePhoneNumber(number string) string {
	if strings.HasPrefix(number, msgServicePrefix) {
		return number
	}

	return phoneReplacer.Replace(number)
}
