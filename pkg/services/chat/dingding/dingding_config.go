package dingding

import (
	"net/url"
	"text/template"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// type DingdingServiceKind string

// const (
// 	DingdingServiceKindCustomBot  DingdingServiceKind = "custombot"  // "自定义机器人"
// 	DingdingServiceKindWorkNotice DingdingServiceKind = "worknotice" // "工作通知"
// )

// Config for the dingding service.
type Config struct {
	// not really enumless, but not important
	standard.EnumlessConfig

	Kind string `json:"kind" required:"true" desc:"The type of Dingding service to use, either 'custombot'(default) or 'worknotice'"`

	AccessToken string `json:"access_token" required:"true" secret:"false" desc:"For the Dingding custom bot, found in the bot's url, should be 64 characters of hexadecimal. For work notice, this is app's 'Client ID (原 AppKey 和 SuiteKey)'"`
	Secret      string `json:"secret,omitempty" required:"false" secret:"true" desc:"For the Dingding custom bot, this is the bot's secret, should starts with 'SEC'. For work notice, this is the app's 'Client Secret (原 AppSecret 和 SuiteSecret)'."`
	Keyword     string `json:"keyword,omitempty" required:"false" secret:"false"`
	APIEndpoint string `json:"apiendpoint,omitempty" required:"false" secret:"false" desc:"Dingding API endpoint, only used for work notice. Default is 'api.dingtalk.com', set to 'api.dingtalk.io' for global dingtalk."`

	// for message rendering
	UserIDs  string `json:"user_ids,omitempty" required:"false" secret:"false" desc:"Comma-separated list of user IDs to send the work notice to. Only used for 'worknotice' kind."`
	Title    string `json:"title,omitempty" required:"false" secret:"false" desc:"Notification title, optionally set by the sender" key:"title"`
	Template string `json:"template,omitempty" required:"false" secret:"false" desc:"Custom message payload JSON template, using Go's text/template syntax. If not set, a default template will be used." key:"template"`

	// internal
	// since standard.Templater do not support FuncMap now, we use go implement
	// tmpl *standard.Templater
	tmpl *template.Template
}

// Scheme is the identifying part of this service's configuration URL.
const Scheme = "dingding"

// Clone creates a copy of the Config.
func (c *Config) Clone() *Config {
	newTmpl, err := makeTemplate(c.Kind, c.Template)
	if err != nil {
		// we cannot handle this
		// should never happen since the template is static and known to be valid
		panic("failed to clone dingding config: " + err.Error())
	}

	return &Config{
		Kind:        c.Kind,
		AccessToken: c.AccessToken,
		Secret:      c.Secret,
		Keyword:     c.Keyword,
		APIEndpoint: c.APIEndpoint,
		UserIDs:     c.UserIDs,
		Title:       c.Title,
		Template:    c.Template,

		tmpl: newTmpl,
	}
}

// GetURL returns a URL representation of its current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// SetURL updates a ServiceConfig from a URL representation of its field values.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (c *Config) getURL(_ types.ConfigQueryResolver) *url.URL {
	query := &url.Values{}
	if c.UserIDs != "" {
		query.Set("userids", c.UserIDs)
	}
	if c.Template != "" {
		query.Set("template", c.Template)
	}
	if c.Title != "" {
		query.Set("title", c.Title)
	}
	if c.Secret != "" {
		query.Set("secret", c.Secret)
	}
	if c.Keyword != "" {
		query.Set("keyword", c.Keyword)
	}
	if c.Kind != "" {
		query.Set("kind", c.Kind)
	}
	if c.APIEndpoint != "" {
		query.Set("apiendpoint", c.APIEndpoint)
	}

	return &url.URL{
		Host:     c.AccessToken,
		RawQuery: query.Encode(),
		Scheme:   Scheme,
	}
}

// setURL updates the Config from a URL using the provided resolver.
func (c *Config) setURL(_ types.ConfigQueryResolver, serviceURL *url.URL) error {

	c.AccessToken = serviceURL.Host

	if serviceURL.Query().Has("userids") {
		c.UserIDs = serviceURL.Query().Get("userids")
	}
	if serviceURL.Query().Has("template") {
		c.Template = serviceURL.Query().Get("template")
	}
	if serviceURL.Query().Has("title") {
		c.Title = serviceURL.Query().Get("title")
	}
	if serviceURL.Query().Has("secret") {
		c.Secret = serviceURL.Query().Get("secret")
	}
	if serviceURL.Query().Has("keyword") {
		c.Keyword = serviceURL.Query().Get("keyword")
	}
	c.Kind = "custombot" // default kind
	if serviceURL.Query().Has("kind") {
		c.Kind = serviceURL.Query().Get("kind")
	}
	if serviceURL.Query().Has("apiendpoint") {
		c.APIEndpoint = serviceURL.Query().Get("apiendpoint")
	}

	if c.Kind != "custombot" && c.Kind != "worknotice" {
		return ErrInvalidKind
	}

	switch c.Kind {
	case "custombot":
		// for custombot, auth using secret, keyword, or ip whitelisting
		// so may be all empty
	case "worknotice":
		if c.Secret == "" {
			return ErrMissingCred
		}
		if c.UserIDs == "" {
			return ErrMissingUserIDs
		}
	}

	return nil
}

// CreateConfigFromURL creates a new Config from a URL for use within the dingding service.
func CreateConfigFromURL(serviceURL *url.URL) (*Config, error) {
	config := Config{}
	err := config.setURL(nil, serviceURL)
	if err != nil {
		return nil, err
	}

	tmpl, err := makeTemplate(config.Kind, config.Template)
	if err != nil {
		return nil, err
	}

	config.tmpl = tmpl

	return &config, err
}

const dummyAccessToken = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// isDummyURL checks if the given URL is the dummy URL used for documentation generation.
// It compares URL components instead of string comparison to avoid issues with
// URL formatting differences.
func isDummyURL(u *url.URL) bool {
	return u.Host == dummyAccessToken
}
