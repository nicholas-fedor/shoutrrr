package router_test

import (
	"io"
	"log"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// fuzzSecretMarker marks the userinfo values FuzzLocate treats as secrets. No
// error or log text contains it, so finding a marked value there is a leak, and
// not a value that happens to match fixed text.
const fuzzSecretMarker = "SECRET"

// skipEmptySliceValue is the known failure FuzzConfigRoundTrip skips for URLs
// with an empty query value: an empty value sets a slice key to one empty item,
// which GetURL leaves out as the default. Fixing it must remove the skip.
const skipEmptySliceValue = "an empty query value sets a slice key to one empty item, which GetURL leaves out"

// skipUnbracketedIPv6Host is the known failure FuzzConfigRoundTrip skips for
// hosts with several colons and no brackets, such as ::1: services split them
// into a hostname and port that net.JoinHostPort rebuilds as a host that does
// not parse, such as [:]:1. Fixing it must remove the skip.
const skipUnbracketedIPv6Host = "a host with several colons and no brackets comes back as a host that does not parse"

// skipDocsDummyURL is the reason FuzzConfigRoundTrip skips the documentation
// generator's <scheme>://dummy@dummy.com URL, which services accept without
// validation only so the generator can render their config fields, and whose
// config is not meant to be restored from its URL.
const skipDocsDummyURL = "the documentation dummy URL bypasses validation and is not a real configuration"

// roundTripAllowlist records the schemes whose configs FuzzConfigRoundTrip skips,
// each with the known failure. Fixing a failure must remove its entry.
var roundTripAllowlist = map[string]string{
	"gotify":     "Gotify accepts a URL without a host, and GetURL then writes the token as the host",
	"ntfy":       "ntfy accepts a topic that starts with a slash, which GetURL writes as the path's own slash",
	"rocketchat": "Rocket.Chat accepts a URL without a host, but rejects the URL GetURL writes for it",
	"signal":     "Signal keeps the port in the host when the URL has no hostname, so GetURL writes a host such as [:0]:0",
}

// FuzzLocate checks that locating any service URL never panics, performs no
// network I/O, and keeps a username or password marked with [fuzzSecretMarker]
// out of the error and the logs. The fuzzer picks the scheme by index into
// [router.SupportedSchemas] and supplies the rest of the URL. The seed corpus is
// the contract fixture of every scheme, with a custom URL whose conversion fails
// and URLs with malformed escapes and ports.
func FuzzLocate(f *testing.F) {
	schemes := router.SupportedSchemas()
	addRouterSeeds(f, schemes)

	f.Fuzz(func(t *testing.T, index uint8, rest string) {
		rawURL := schemes[int(index)%len(schemes)] + rest

		logs := &syncBuffer{}
		serviceRouter, err := router.NewWithOptions(log.New(logs, "", 0), types.SenderOptions{})
		require.NoError(t, err)

		transportHits, lookups := defaultTransportHits.Load(), lookupHits.Load()

		service, err := serviceRouter.Locate(rawURL)
		closeService(t, service)

		assert.Equal(t, transportHits, defaultTransportHits.Load(), "Locate used http.DefaultTransport")
		assert.Equal(t, lookups, lookupHits.Load(), "Locate looked up a host")

		var errText string
		if err != nil {
			errText = err.Error()
		}

		for _, secret := range userinfoSecrets(rawURL) {
			assert.NotContains(t, errText, secret, "the error quotes the userinfo")
			assert.NotContains(t, logs.String(), secret, "the logs quote the userinfo")
		}
	})
}

// FuzzConfigRoundTrip checks that a service located from any URL can be located
// again from its config's URL, and that the second config produces the same URL,
// so a config survives being saved and restored through its URL. The fuzzer
// picks the scheme by index into [router.SupportedSchemas] and supplies the rest
// of the URL. The seed corpus matches [FuzzLocate]'s.
func FuzzConfigRoundTrip(f *testing.F) {
	schemes := router.SupportedSchemas()
	addRouterSeeds(f, schemes)

	f.Fuzz(func(t *testing.T, index uint8, rest string) {
		scheme := schemes[int(index)%len(schemes)]
		if reason, known := roundTripAllowlist[scheme]; known {
			t.Skip(reason)
		}

		rawURL := scheme + rest
		if isDocsDummyURL(rawURL) {
			t.Skip(skipDocsDummyURL)
		}

		if hasEmptyQueryValue(rawURL) {
			t.Skip(skipEmptySliceValue)
		}

		if hasUnbracketedIPv6Host(rawURL) {
			t.Skip(skipUnbracketedIPv6Host)
		}

		serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
		require.NoError(t, err)

		service, err := serviceRouter.Locate(rawURL)
		closeService(t, service)

		if err != nil || !hasConfig(service) {
			return
		}

		first := format.GetServiceConfig(service).GetURL()
		require.NotNil(t, first, "the config of %q has no URL", rawURL)

		restored, err := serviceRouter.Locate(first.String())
		closeService(t, restored)
		require.NoError(t, err, "the config URL %q of %q does not locate", first, rawURL)

		second := format.GetServiceConfig(restored).GetURL()
		require.NotNil(t, second, "the config restored from %q has no URL", first)
		assertSameURL(t, first, second)
	})
}

// addRouterSeeds adds the seed corpus shared by the router fuzz targets: each
// contract fixture split into its scheme index and the rest of its URL, and a
// few malformed URLs.
//
// Parameters:
//   - f: the fuzz target.
//   - schemes: the sorted supported schemes the fuzz targets index into.
func addRouterSeeds(f *testing.F, schemes []string) {
	f.Helper()

	add := func(scheme, rest string) {
		index := slices.Index(schemes, scheme)
		require.GreaterOrEqual(f, index, 0, "scheme %q is not supported", scheme)

		f.Add(uint8(index), rest)
	}

	for _, fx := range fixtures {
		add(fx.scheme, strings.TrimPrefix(fx.url, fx.scheme))
	}

	add("generic", "+https://user:SECRETgenericPASSWORD@example.invalid/?disabletls=maybe")
	add("discord", "://SECRETdiscordTOKEN@123456/%zz")
	add("smtp", "://user:SECRETsmtpPASSWORD@mail.example.invalid:port/")
	add("mqtt", "://:SECRETmqttPASSWORD@broker.example.invalid/topic")
	add("mqtt", "://broker.example.invalid//home/alerts")
	add("wecom", "://key?key=other-key_2&mentioned_list=@all")
	add("generic", "://example.invalid/hook?@X-API-Key=key&@x-1st=first&@acceptLanguage=tlh")
	add("hangouts", ":///v1/spaces/FOO/messages?key=k&token=t")
	add("googlechat", "://dummy@dummy.com")
}

// userinfoSecrets returns the username and password of rawURL that carry
// [fuzzSecretMarker] and occur nowhere else in the URL, so finding one in an
// error or a log means the userinfo leaked.
//
// Parameters:
//   - rawURL: the service URL.
//
// Returns:
//   - []string: the userinfo values to look for, or nil when the URL does not
//     parse or has none.
func userinfoSecrets(rawURL string) []string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User == nil {
		return nil
	}

	query, _ := url.QueryUnescape(parsed.RawQuery)
	elsewhere := []string{
		parsed.Scheme, parsed.Opaque, parsed.Host, parsed.Path, parsed.RawPath,
		parsed.RawQuery, query, parsed.Fragment, parsed.RawFragment,
	}

	password, _ := parsed.User.Password()

	var secrets []string

	for _, value := range []string{parsed.User.Username(), password} {
		if !strings.Contains(value, fuzzSecretMarker) ||
			slices.ContainsFunc(elsewhere, func(part string) bool { return strings.Contains(part, value) }) {
			continue
		}

		secrets = append(secrets, value)
	}

	return secrets
}

// isDocsDummyURL reports whether rawURL is the documentation generator's
// dummy URL, which services recognize by comparing the parsed URL's string
// with <scheme>://dummy@dummy.com.
//
// Parameters:
//   - rawURL: the service URL.
//
// Returns:
//   - bool: true when the URL parses to the dummy URL of its scheme.
func isDocsDummyURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)

	return err == nil && parsed.String() == parsed.Scheme+"://dummy@dummy.com"
}

// hasEmptyQueryValue reports whether rawURL has a query parameter without a
// value.
//
// Parameters:
//   - rawURL: the service URL.
//
// Returns:
//   - bool: true when the URL parses and a query parameter has an empty value.
func hasEmptyQueryValue(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	for _, values := range parsed.Query() {
		if slices.Contains(values, "") {
			return true
		}
	}

	return false
}

// hasUnbracketedIPv6Host reports whether rawURL has a host with several colons
// that is not enclosed in brackets.
//
// Parameters:
//   - rawURL: the service URL.
//
// Returns:
//   - bool: true when the URL parses and its host is an unbracketed IPv6 form.
func hasUnbracketedIPv6Host(rawURL string) bool {
	parsed, err := url.Parse(rawURL)

	return err == nil && !strings.HasPrefix(parsed.Host, "[") && strings.Count(parsed.Host, ":") > 1
}

// hasConfig reports whether service has the Config field that
// [format.GetServiceConfig] reads.
//
// Parameters:
//   - service: the service.
//
// Returns:
//   - bool: true when the service struct has a Config field.
func hasConfig(service types.Service) bool {
	_, found := reflect.Indirect(reflect.ValueOf(service)).Type().FieldByName("Config")

	return found
}

// closeService closes service when it holds resources between sends.
//
// Parameters:
//   - t: the test.
//   - service: the service, which may be nil.
func closeService(t *testing.T, service types.Service) {
	t.Helper()

	if closer, ok := service.(io.Closer); ok {
		assert.NoError(t, closer.Close())
	}
}

// assertSameURL asserts that two config URLs match in every part, comparing
// their query values rather than the order of the query string.
//
// Parameters:
//   - t: the test.
//   - want: the first config URL.
//   - got: the config URL after the round trip.
func assertSameURL(t *testing.T, want, got *url.URL) {
	t.Helper()

	assert.Equal(t, want.Scheme, got.Scheme)
	assert.Equal(t, want.User.String(), got.User.String())
	assert.Equal(t, want.Host, got.Host)
	assert.Equal(t, want.Opaque, got.Opaque)
	assert.Equal(t, want.Path, got.Path)
	assert.Equal(t, want.Fragment, got.Fragment)

	wantQuery, err := url.ParseQuery(want.RawQuery)
	require.NoError(t, err)

	gotQuery, err := url.ParseQuery(got.RawQuery)
	require.NoError(t, err)

	assert.Equal(t, wantQuery, gotQuery)
}
