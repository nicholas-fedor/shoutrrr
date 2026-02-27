package twilio

import (
	"errors"
	"fmt"
	"net/url"
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
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates the Config from a URL representation of its field values.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	config.AccountSID = url.User.Username()

	password, _ := url.User.Password()
	config.AuthToken = password

	config.FromNumber = normalizePhoneNumber(url.Host)
	config.ToNumbers = parseToNumbers(url.Path)

	for key, vals := range url.Query() {
		err := resolver.Set(key, vals[0])
		if err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	if url.String() != "twilio://dummy@dummy.com" {
		return config.validate()
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
func (config *Config) validate() error {
	if config.AccountSID == "" {
		return ErrAccountSIDMissing
	}

	if config.AuthToken == "" {
		return ErrAuthTokenMissing
	}

	if config.FromNumber == "" {
		return ErrFromNumberMissing
	}

	if len(config.ToNumbers) == 0 {
		return ErrToNumbersMissing
	}

	// Twilio rejects calls/messages where To == From.
	if !strings.HasPrefix(config.FromNumber, msgServicePrefix) {
		for _, to := range config.ToNumbers {
			if to == config.FromNumber {
				return ErrToFromNumberSame
			}
		}
	}

	return nil
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	path := "/" + strings.Join(config.ToNumbers, "/")

	return &url.URL{
		Scheme:     Scheme,
		User:       url.UserPassword(config.AccountSID, config.AuthToken),
		Host:       config.FromNumber,
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
