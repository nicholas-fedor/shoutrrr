package smtp

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "smtp"

// Static errors for configuration validation.
var (
	ErrFromAddressMissing = errors.New("fromAddress missing from config URL")
	ErrToAddressMissing   = errors.New("toAddress missing from config URL")
)

// Config is the configuration needed to send e-mail notifications over SMTP.
type Config struct {
	Host        string    `desc:"SMTP server hostname or IP address"                     url:"Host"`
	Username    string    `desc:"SMTP server username"                                   url:"User" default:""`
	Password    string    `desc:"SMTP server password or hash (for OAuth2)"              url:"Pass" default:""`
	Port        uint16    `desc:"SMTP server port, common ones are 25, 465, 587 or 2525" url:"Port" default:"25"`
	FromAddress string    `desc:"E-mail address that the mail are sent from"                                                        key:"fromaddress,from"`
	FromName    string    `desc:"Name of the sender"                                                                                key:"fromname"             optional:"yes"`
	ToAddresses []string  `desc:"List of recipient e-mails"                                                                         key:"toaddresses,to"`
	Subject     string    `desc:"The subject of the sent mail"                                      default:"Shoutrrr Notification" key:"subject,title"`
	Auth        authType  `desc:"SMTP authentication method"                                        default:"Unknown"               key:"auth"`
	Encryption  encMethod `desc:"Encryption method"                                                 default:"Auto"                  key:"encryption"`
	UseStartTLS bool      `desc:"Whether to use StartTLS encryption"                                default:"Yes"                   key:"usestarttls,starttls"`
	UseHTML     bool      `desc:"Whether the message being sent is in HTML"                         default:"No"                    key:"usehtml"`
	ClientHost  string    `desc:"SMTP client hostname"                                              default:"localhost"             key:"clienthost"`
}

// GetURL returns a URL representation of its current field values.
func (config *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(config)

	return config.getURL(&resolver)
}

// SetURL updates a ServiceConfig from a URL representation of its field values.
func (config *Config) SetURL(url *url.URL) error {
	resolver := format.NewPropKeyResolver(config)

	return config.setURL(&resolver, url)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (config *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	return &url.URL{
		User:       util.URLUserPassword(config.Username, config.Password),
		Host:       fmt.Sprintf("%s:%d", config.Host, config.Port),
		Path:       "/",
		Scheme:     Scheme,
		ForceQuery: true,
		RawQuery:   format.BuildQuery(resolver),
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (config *Config) setURL(resolver types.ConfigQueryResolver, url *url.URL) error {
	password, _ := url.User.Password()
	config.Username = url.User.Username()
	config.Password = password
	config.Host = url.Hostname()

	if port, err := strconv.ParseUint(url.Port(), 10, 16); err == nil {
		config.Port = uint16(port)
	}

	for key, vals := range url.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q to %q: %w", key, vals[0], err)
		}
	}

	if url.String() != "smtp://dummy@dummy.com" {
		if len(config.FromAddress) < 1 {
			return ErrFromAddressMissing
		}

		if len(config.ToAddresses) < 1 {
			return ErrToAddressMissing
		}
	}

	return nil
}

// Clone returns a copy of the config.
func (config *Config) Clone() Config {
	clone := *config
	clone.ToAddresses = make([]string, len(config.ToAddresses))
	copy(clone.ToAddresses, config.ToAddresses)

	return clone
}

// FixEmailTags replaces parsed spaces (+) in e-mail addresses with '+'.
func (config *Config) FixEmailTags() {
	config.FromAddress = strings.ReplaceAll(config.FromAddress, " ", "+")
	for i, adr := range config.ToAddresses {
		config.ToAddresses[i] = strings.ReplaceAll(adr, " ", "+")
	}
}

// Enums returns the fields that should use a corresponding EnumFormatter to Print/Parse their values.
func (config *Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"Auth":       AuthTypes.Enum,
		"Encryption": EncMethods.Enum,
	}
}
