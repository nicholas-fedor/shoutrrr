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
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Config holds the configuration for the Ntfy service.
type Config struct {
	// Title is the message title.
	Title string `default:"" desc:"Message title" key:"title"`
	// Host is the ntfy server host, with an optional port.
	Host string `default:"ntfy.sh" desc:"Server hostname and port" url:"host"`
	// Topic is the topic to publish to, written as the path of the service
	// URL.
	Topic string `desc:"Target topic name" required:"" url:"path"`
	// Password is the password for HTTP Basic authentication.
	Password string `desc:"Auth password" optional:"" sensitive:"true" url:"password"`
	// Username is the username for HTTP Basic authentication.
	Username string `desc:"Auth username" optional:"" url:"user"`
	// Token is the access token for Bearer authentication. It takes
	// precedence over Username and Password.
	Token string `desc:"Access token for Bearer auth, takes precedence over username and password" key:"token" optional:"" sensitive:"true"`
	// Scheme is the server's protocol, http or https. A URL or params that set
	// it to http turn TLS off, and GetURL writes that as disabletls.
	//
	// Deprecated: Use [Config.DisableTLS], which the disabletls key sets.
	Scheme string `default:"https" desc:"Deprecated: use disabletls. Server protocol, http or https" key:"scheme"`
	// Click is the URL opened when the notification is tapped.
	Click string `desc:"Website opened when notification is clicked" key:"click" optional:""`
	// Attach is the URL of a file to attach.
	Attach string `desc:"URL of an attachment, see attach via URL" key:"attach" optional:""`
	// Filename is the attachment's file name.
	Filename string `desc:"File name of the attachment" key:"filename" optional:""`
	// Delay schedules delivery for a timestamp or after a duration.
	Delay string `desc:"Timestamp or duration for delayed delivery, see https://docs.ntfy.sh/publish/#scheduled-delivery" key:"delay,at,in" optional:""`
	// Email forwards the notification to this e-mail address.
	Email string `desc:"E-mail address for e-mail notifications" key:"email" optional:""`
	// Icon is the URL of the notification icon.
	Icon string `desc:"URL to use as notification icon" key:"icon" optional:""`
	// Tags lists the message tags. Tags that match an emoji short code show
	// as that emoji.
	Tags []string `desc:"List of tags that may or not map to emojis" key:"tags" optional:""`
	// Actions lists the action buttons, separated by semicolons.
	Actions []string `desc:"Custom user action buttons for notifications, see https://docs.ntfy.sh/publish/#action-buttons" key:"actions" optional:"" sep:";"`
	// Priority is the message priority, from Min to Max. When neither the URL
	// nor the send sets it, the level param chooses it.
	Priority priority `default:"default" desc:"Message priority with 1=min, 3=default and 5=max" key:"priority"`
	// Markdown renders the message as Markdown.
	Markdown bool `default:"no" desc:"Enable Markdown formatting, see https://docs.ntfy.sh/publish/#markdown-formatting" key:"markdown"`
	// Cache keeps the message on the server for clients that connect later.
	Cache bool `default:"yes" desc:"Cache messages" key:"cache"`
	// Firebase forwards the message to Firebase Cloud Messaging for Android
	// clients.
	Firebase bool `default:"yes" desc:"Send to firebase" key:"firebase"`
	// SkipTLSVerify skips verification of the server's TLS certificate.
	SkipTLSVerify bool `default:"no" deprecatedkeys:"disabletlsverification" desc:"Skip TLS certificate verification" key:"skiptlsverify"`
	// DisableTLSVerification is the old name of SkipTLSVerify. When
	// SkipTLSVerify is off, setting it skips verification too.
	//
	// Deprecated: Use [Config.SkipTLSVerify], which the skiptlsverify key sets.
	DisableTLSVerification bool `deprecated:"SkipTLSVerify"`
	// DisableTLS publishes over HTTP whatever Scheme says.
	DisableTLS bool `default:"no" desc:"Send over HTTP instead of HTTPS" key:"disabletls"`
}

// Identifiers and defaults for this service's configuration URL.
const (
	// Scheme is the identifying part of this service's configuration URL.
	Scheme = "ntfy"
	// DefaultHost is the public ntfy server a URL without a host uses.
	DefaultHost = "ntfy.sh"
)

// Protocols that the deprecated scheme accepts.
const (
	// schemeHTTP publishes over plain HTTP.
	schemeHTTP = "http"
	// schemeHTTPS publishes over HTTPS, the default.
	schemeHTTPS = "https"
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
	// ErrInvalidScheme indicates a scheme other than http or https.
	ErrInvalidScheme = errors.New("invalid scheme: use http or https, or disabletls instead")
)

// Enums returns the fields that use an EnumFormatter for their values.
//
// Returns:
//   - map[string]types.EnumFormatter: the priority formatter by field name.
func (*Config) Enums() map[string]types.EnumFormatter {
	return map[string]types.EnumFormatter{
		"Priority": Priority.Enum,
	}
}

// GetAPIURL constructs the API URL for the Ntfy service based on the configuration.
// It uses HTTP when TLS is disabled or the deprecated scheme is http, and HTTPS
// otherwise.
//
// Returns:
//   - string: the publish URL for the topic.
func (c *Config) GetAPIURL() string {
	path := c.Topic
	if !strings.HasPrefix(c.Topic, "/") {
		path = "/" + path
	}

	scheme := schemeHTTPS
	if c.DisableTLS || strings.EqualFold(c.Scheme, schemeHTTP) {
		scheme = schemeHTTP
	}

	apiURL := url.URL{
		Scheme: scheme,
		Host:   c.serverHost(),
		Path:   path,
	}

	return apiURL.String()
}

// GetURL returns a URL representation of the Config's current field values.
// A value set through a deprecated field is written under its replacement's
// key, and an http scheme is written as disabletls.
//
// Returns:
//   - *url.URL: the service URL.
func (c *Config) GetURL() *url.URL {
	synced := *c

	_, _ = format.SyncDeprecatedFields(&synced) //nolint:errcheck // The tags are valid, and a failed sync changes nothing.
	synced.normalizeScheme()
	resolver := format.NewPropKeyResolver(&synced)

	return synced.getURL(&resolver)
}

// QueryFields returns the list of query parameter names for the Config struct.
//
// Returns:
//   - []string: the query keys.
func (c *Config) QueryFields() []string {
	return format.GetConfigQueryResolver(c).QueryFields()
}

// SetURL updates the Config from a URL representation of its field values.
//
// Parameters:
//   - serviceURL: the service URL.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrTopicRequired] or
//     [ErrInvalidTopic].
func (c *Config) SetURL(serviceURL *url.URL) error {
	resolver := format.NewPropKeyResolver(c)

	return c.setURL(&resolver, serviceURL)
}

// applyScheme checks the deprecated Scheme and turns TLS off when it is http,
// so DisableTLS alone decides the protocol. An empty Scheme counts as https.
//
// Returns:
//   - error: [ErrInvalidScheme] for a scheme other than http or https.
func (c *Config) applyScheme() error {
	switch strings.ToLower(c.Scheme) {
	case schemeHTTP:
		c.DisableTLS = true
	case schemeHTTPS, "":
	default:
		return fmt.Errorf("%w: %q", ErrInvalidScheme, c.Scheme)
	}

	return nil
}

// getURL constructs a URL from the Config's fields using the provided resolver.
//
// Parameters:
//   - resolver: the resolver that builds the query from the config props.
//
// Returns:
//   - *url.URL: the service URL, with the topic as its path.
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

// normalizeScheme writes an http scheme as DisableTLS and resets the scheme to
// its default, so a config URL carries disabletls instead of the deprecated
// scheme key. An invalid scheme is kept, so reading the URL reports it.
func (c *Config) normalizeScheme() {
	switch strings.ToLower(c.Scheme) {
	case schemeHTTP:
		c.DisableTLS = true
		c.Scheme = schemeHTTPS
	case schemeHTTPS, "":
		c.Scheme = schemeHTTPS
	default:
	}
}

// serverHost returns the host requests go to. A host without a hostname, such
// as an empty host or a port alone, uses [DefaultHost] with that port, as a
// URL without a host does.
//
// Returns:
//   - string: the config's host, or [DefaultHost] with the host's port.
func (c *Config) serverHost() string {
	host, port, err := util.SplitURLHost(&url.URL{Host: c.Host})
	if err != nil || host != "" {
		return c.Host
	}

	return util.JoinURLHost(DefaultHost, port)
}

// setURL updates the Config from a URL using the provided resolver.
//
// A URL without a host uses [DefaultHost], keeping any port. The topic is the
// URL path without its first slash. Its last segment is the
// topic name, which must follow ntfy's rule, and any segments before it form a
// base path, such as for a server behind a reverse proxy.
//
// Parameters:
//   - resolver: the resolver that sets the query parameters.
//   - serviceURL: the service URL.
//
// Returns:
//   - error: a host error such as [util.ErrInvalidHost], a malformed query
//     ([util.ErrInvalidQuery]) or query parameter error, [ErrInvalidScheme],
//     [ErrTopicRequired] for a missing topic, or [ErrInvalidTopic] for an
//     invalid topic name or an empty path segment.
func (c *Config) setURL(resolver types.ConfigQueryResolver, serviceURL *url.URL) error {
	if serviceURL.User != nil {
		password, _ := serviceURL.User.Password()
		c.Password = password
		c.Username = serviceURL.User.Username()
	} else {
		c.Password = ""
		c.Username = ""
	}

	host, port, err := util.SplitURLHost(serviceURL)
	if err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	c.Host = util.JoinURLHost(host, port)
	c.Host = c.serverHost()
	c.Topic = strings.TrimPrefix(serviceURL.Path, "/")
	c.Token = ""

	// A literal semicolon in the query is a value, such as between actions.
	// The query is read from a copy, so the caller's URL is not changed.
	queryURL := *serviceURL
	queryURL.RawQuery = strings.ReplaceAll(queryURL.RawQuery, ";", "%3b")

	query, err := util.ParseURLQuery(&queryURL)
	if err != nil {
		return err
	}

	if err := format.ApplyQuery(resolver, query); err != nil {
		return err
	}

	disableTLS := c.DisableTLS

	if err := c.applyScheme(); err != nil {
		return err
	}

	// disabletls replaces scheme, so it wins when the query sets both. The
	// scheme resets, so a send that applies the stored scheme keeps the choice.
	if hasQueryKey(query, "disabletls") {
		c.DisableTLS = disableTLS
		c.Scheme = schemeHTTPS
	}

	return c.validate()
}

// validate checks the host syntax and the topic. Reading a service URL and
// each send run it, so a config built in code fails before a request.
//
// Returns:
//   - error: a host error such as [util.ErrInvalidHost] for a host that is not
//     a hostname or a bracketed IPv6 address, [ErrTopicRequired] for a
//     missing topic, [ErrInvalidTopic] for an invalid topic name or an empty
//     path segment, or nil.
func (c *Config) validate() error {
	if _, _, err := util.SplitURLHost(&url.URL{Host: c.Host}); err != nil {
		return fmt.Errorf("parsing host: %w", err)
	}

	if c.Topic == "" {
		return ErrTopicRequired
	}

	if !isValidTopicPath(c.Topic) {
		return ErrInvalidTopic
	}

	return nil
}

// hasQueryKey reports whether query sets key, matched case-insensitively as
// the config keys are.
//
// Parameters:
//   - query: the URL query.
//   - key: the key in lower case.
//
// Returns:
//   - bool: whether the query holds the key.
func hasQueryKey(query url.Values, key string) bool {
	for queryKey := range query {
		if strings.EqualFold(queryKey, key) {
			return true
		}
	}

	return false
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
