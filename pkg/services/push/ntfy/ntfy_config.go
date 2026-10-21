package ntfy

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Config holds the configuration for the Ntfy service.
type Config struct {
	Title                  string   `default:""                                                                                              desc:"Message title"                                                                     key:"title"`
	Host                   string   `default:"ntfy.sh"                                                                                       desc:"Server hostname and port"                                                          url:"host"`
	Topic                  string   `desc:"Target topic name"                                                                                required:""                                                                              url:"path"`
	Password               string   `desc:"Auth password"                                                                                    optional:""                                                                              url:"password"`
	Username               string   `desc:"Auth username"                                                                                    optional:""                                                                              url:"user"`
	Token                  string   `desc:"Access token for Bearer auth, takes precedence over username and password"                        key:"token"                                                                              optional:""                  sensitive:"true"`
	Scheme                 string   `default:"https"                                                                                         desc:"Server protocol, http or https"                                                    key:"scheme"`
	Tags                   []string `desc:"List of tags that may or not map to emojis"                                                       key:"tags"                                                                               optional:""`
	Priority               priority `default:"default"                                                                                       desc:"Message priority with 1=min, 3=default and 5=max"                                  key:"priority"`
	Actions                []string `desc:"Custom user action buttons for notifications, see https://docs.ntfy.sh/publish/#action-buttons"   key:"actions"                                                                            optional:""                  sep:";"`
	Click                  string   `desc:"Website opened when notification is clicked"                                                      key:"click"                                                                              optional:""`
	Attach                 string   `desc:"URL of an attachment, see attach via URL"                                                         key:"attach"                                                                             optional:""`
	Filename               string   `desc:"File name of the attachment"                                                                      key:"filename"                                                                           optional:""`
	Delay                  string   `desc:"Timestamp or duration for delayed delivery, see https://docs.ntfy.sh/publish/#scheduled-delivery" key:"delay,at,in"                                                                        optional:""`
	Email                  string   `desc:"E-mail address for e-mail notifications"                                                          key:"email"                                                                              optional:""`
	Icon                   string   `desc:"URL to use as notification icon"                                                                  key:"icon"                                                                               optional:""`
	Markdown               bool     `default:"no"                                                                                            desc:"Enable Markdown formatting, see https://docs.ntfy.sh/publish/#markdown-formatting" key:"markdown"`
	Cache                  bool     `default:"yes"                                                                                           desc:"Cache messages"                                                                    key:"cache"`
	Firebase               bool     `default:"yes"                                                                                           desc:"Send to firebase"                                                                  key:"firebase"`
	DisableTLSVerification bool     `default:"no"                                                                                            desc:"Disable TLS certificate verification"                                              key:"disabletlsverification"`
	DisableTLS             bool     `default:"no"                                                                                            desc:"Disable TLS entirely (force HTTP scheme)"                                          key:"disabletls"`
}

// Scheme is the identifying part of this service's configuration URL.
const (
	Scheme = "ntfy"
)

// topicNamePattern matches the topic names ntfy accepts.
var topicNamePattern = regexp.MustCompile(`^[-_A-Za-z0-9]{1,64}$`)

// Errors for an invalid topic in the config URL.
var (
	// ErrTopicRequired indicates that the topic is missing from the config URL.
	ErrTopicRequired = errors.New("topic is required")
	// ErrInvalidTopic indicates a topic name that ntfy does not accept, or a
	// path with an empty segment.
	ErrInvalidTopic = errors.New("invalid topic: use up to 64 letters, digits, underscores, and dashes")
)

// Enums returns the fields that use an EnumFormatter for their values.
func (*Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"Priority": Priority.Enum,
	}
}

// GetAPIURL constructs the API URL for the Ntfy service based on the configuration.
// It uses HTTP when TLS is disabled and the configured scheme otherwise.
//
// Returns:
//   - string: the publish URL for the topic.
func (c *Config) GetAPIURL() string {
	path := c.Topic
	if !strings.HasPrefix(c.Topic, "/") {
		path = "/" + path
	}

	scheme := c.Scheme
	if c.DisableTLS {
		scheme = "http"
	}

	apiURL := url.URL{
		Scheme: scheme,
		Host:   c.Host,
		Path:   path,
	}

	return apiURL.String()
}

// GetURL returns a URL representation of the Config's current field values.
func (c *Config) GetURL() *url.URL {
	resolver := format.NewPropKeyResolver(c)

	return c.getURL(&resolver)
}

// QueryFields returns the list of query parameter names for the Config struct.
func (c *Config) QueryFields() []string {
	return format.GetConfigQueryResolver(c).QueryFields()
}

// SetURL updates the Config from a URL representation of its field values.
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// getURL constructs a URL from the Config's fields using the provided resolver.
func (c *Config) getURL(resolver types.ConfigQueryResolver) *url.URL {
	path := c.Topic
	if !strings.HasPrefix(c.Topic, "/") {
		path = "/" + path
	}

	result := &url.URL{
		Host:       c.Host,
		Scheme:     Scheme,
		ForceQuery: true,
		Path:       path,
		RawQuery:   format.BuildQuery(resolver),
	}
	if c.Username != "" {
		if c.Password != "" {
			result.User = url.UserPassword(c.Username, c.Password)
		} else {
			result.User = url.User(c.Username)
		}
	}

	return result
}

// setURL updates the Config from a URL using the provided resolver.
//
// The topic is the URL path without its first slash. Its last segment is the
// topic name, which must follow ntfy's rule, and any segments before it form a
// base path, such as for a server behind a reverse proxy.
//
// Parameters:
//   - resolver: the resolver that sets the query parameters.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: a query parameter error, [ErrTopicRequired] for a missing topic,
//     or [ErrInvalidTopic] for an invalid topic name or an empty path segment.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	if serviceURL.User != nil {
		password, _ := serviceURL.User.Password()
		c.Password = password
		c.Username = serviceURL.User.Username()
	} else {
		c.Password = ""
		c.Username = ""
	}

	c.Host = serviceURL.Host
	c.Topic = strings.TrimPrefix(serviceURL.Path, "/")
	c.Token = ""

	serviceURL.RawQuery = strings.ReplaceAll(serviceURL.RawQuery, ";", "%3b")
	for key, vals := range serviceURL.Query() {
		if err := resolver.Set(key, vals[0]); err != nil {
			return fmt.Errorf("setting query parameter %q: %w", key, err)
		}
	}

	if serviceURL.String() != "ntfy://dummy@dummy.com" {
		if c.Topic == "" {
			return ErrTopicRequired
		}

		if !isValidTopicPath(c.Topic) {
			return ErrInvalidTopic
		}
	}

	return nil
}

// isValidTopicPath reports whether topicPath is a topic name, optionally after
// a base path, with no empty segment. The topic name follows ntfy's rule of up
// to 64 letters, digits, underscores, and dashes.
//
// Parameters:
//   - topicPath: the URL path without its first slash.
//
// Returns:
//   - bool: true when every segment is non-empty and the last one is a valid
//     topic name.
func isValidTopicPath(topicPath string) bool {
	segments := strings.Split(topicPath, "/")
	if slices.Contains(segments, "") {
		return false
	}

	return topicNamePattern.MatchString(segments[len(segments)-1])
}
