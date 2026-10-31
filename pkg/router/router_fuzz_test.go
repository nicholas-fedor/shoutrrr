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

// roundTripAllowlist records the schemes whose configs FuzzConfigRoundTrip skips,
// each with the known failure. Fixing a failure must remove its entry.
var roundTripAllowlist = map[string]string{}

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
