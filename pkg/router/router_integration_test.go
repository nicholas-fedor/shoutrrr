package router_test

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// netKind is how a service reaches the network.
type netKind int

// fixture is a valid service URL for one scheme. Every value in secrets appears in
// url and must never show up in errors or logs.
type fixture struct {
	scheme  string
	url     string
	kind    netKind
	secrets []string
	status  int    // response status for the injected client; defaults to 200
	body    string // response body for the injected client; defaults to "{}"
}

// env holds the injected client, dialer and logger for one contract case.
type env struct {
	fx     *fixture
	client *mocks.MockHTTPClient
	dials  atomic.Int32
	// markedDials counts dials whose context carries [contextMarker].
	markedDials atomic.Int32
	// canceledOps counts marked requests and dials that ended with the caller's cancellation.
	canceledOps atomic.Int32
	logs        *syncBuffer
	logger      *log.Logger
	sendErr     error
	panicMsg    string
	// stop ends when the test ends, which releases held requests and dials.
	stop <-chan struct{}
	// holdMarked makes marked requests and dials wait until their context ends.
	holdMarked bool
}

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

// httpClientFunc adapts a function to [types.HTTPClient].
type httpClientFunc func(*http.Request) (*http.Response, error)

// spaceReader is an endless source of spaces, which JSON decoders skip as
// whitespace, so only the amount read tells an oversized body apart.
type spaceReader struct{}

// countedBody is a response body that records how many bytes were read from it.
type countedBody struct {
	// r is the body's content.
	r io.Reader
	// read is the number of bytes read so far.
	read *atomic.Int64
}

// contextMarker keys a value on the caller's context, so a check can tell whether
// that context reached a request or dial.
type contextMarker struct{}

// failureKind classifies why a contract check failed.
type failureKind int

// failure is a contract check result; the zero value means the check passed.
type failure struct {
	kind   failureKind
	detail string
}

// knownFailure is an allowlist entry: the audit finding and the failure kind it
// is expected to produce.
type knownFailure struct {
	finding string
	kind    failureKind
}

const (
	netHTTP netKind = iota // uses the injected types.HTTPClient
	netTCP                 // uses the injected DialContext
	netNone                // performs no network I/O
)

// Contract checks run against every registered scheme.
const (
	checkInitializeNoIO    = "initialize_no_io"
	checkSendUsesInjection = "send_uses_injection"
	checkReservedParams    = "reserved_params"
	// checkParamsDoNotPersist verifies that params change only the send they are passed to.
	checkParamsDoNotPersist = "params_do_not_persist"
	checkNilHTTPClient      = "nil_http_client"
	// checkTypedNilHTTPClient repeats the nil client check with a nil *http.Client,
	// which is a non-nil types.HTTPClient interface value.
	checkTypedNilHTTPClient = "typed_nil_http_client"
	checkNoSecretLeaks      = "no_secret_leaks"
	// checkNoSecretLeaksDefault repeats the leak check with each service's own
	// default HTTP client, as used by consumers that do not inject one.
	checkNoSecretLeaksDefault = "no_secret_leaks_default_client"
	// checkSendUsesContext verifies that the context passed to the router's
	// SendContext reaches every request and dial a service makes.
	checkSendUsesContext = "send_uses_context"
)

// Failure kinds, one per distinct way a check can fail.
const (
	failNone              failureKind = iota // the check passed
	failInit                                 // the fixture failed to initialize
	failInitIO                               // Initialize performed network I/O
	failPanic                                // Send panicked
	failDefaultTransport                     // Send used http.DefaultTransport
	failNotInjected                          // Send skipped the injected client or dialer
	failUnexpectedIO                         // a service without network access performed I/O
	failRejectedParams                       // reserved params were rejected as unknown keys
	failInjectedAfterNil                     // the injected client was still used after SetHTTPClient(nil)
	failNoFallbackRequest                    // no request went through the default client after SetHTTPClient(nil)
	failSecretLeak                           // a secret appeared in errors or logs
	failParamsPersisted                      // Send with params changed the service configuration
	failNoDefaultRequest                     // no request went through the service's default client
	failContextDropped                       // a request or dial did not carry the caller's context
	failCancelIgnored                        // a request or dial did not end when the caller canceled
)

// cancelAfter is when the context check cancels the caller's context.
const cancelAfter = time.Second

// oversizedBodySize is the size of the response bodies served by
// TestIntegrationResponseBodyLimit, far past [httpbody.MaxSize].
const oversizedBodySize = 8 * httpbody.MaxSize

// fixtures covers every scheme returned by router.SupportedSchemas.
// Configurable hosts use the reserved .invalid TLD.
var fixtures = []fixture{
	{
		scheme:  "bark",
		url:     "bark://:SECRETbarkDEVICEKEY@bark.example.invalid",
		secrets: []string{"SECRETbarkDEVICEKEY"},
		body:    `{"code":200,"message":"success"}`,
	},
	{
		scheme:  "discord",
		url:     "discord://SECRETdiscordTOKEN@123456789012345678",
		secrets: []string{"SECRETdiscordTOKEN"},
		status:  http.StatusNoContent,
	},
	{
		scheme:  "generic",
		url:     "generic://hooks.example.invalid/webhook?@Authorization=Bearer%20SECRETgenericHEADER&token=SECRETgenericQUERY",
		secrets: []string{"SECRETgenericHEADER", "SECRETgenericQUERY"},
	},
	{
		scheme:  "googlechat",
		url:     "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=SECRETgchatKEY&token=SECRETgchatTOKEN",
		secrets: []string{"SECRETgchatKEY", "SECRETgchatTOKEN"},
	},
	{
		scheme:  "gotify",
		url:     "gotify://gotify.example.invalid/ASECRETgotifyTK",
		secrets: []string{"ASECRETgotifyTK"},
		body:    `{"id":1}`,
	},
	{
		scheme:  "hangouts",
		url:     "hangouts://chat.googleapis.com/v1/spaces/FOO/messages?key=SECREThangoutsKEY&token=SECREThangoutsTOKEN",
		secrets: []string{"SECREThangoutsKEY", "SECREThangoutsTOKEN"},
	},
	{
		scheme:  "homeassistant",
		url:     "homeassistant://SECREThassTOKEN@ha.example.invalid",
		secrets: []string{"SECREThassTOKEN"},
		body:    `[]`,
	},
	{
		scheme:  "ifttt",
		url:     "ifttt://SECRETiftttKEY/?events=event",
		secrets: []string{"SECRETiftttKEY"},
		status:  http.StatusNoContent,
	},
	{
		scheme:  "join",
		url:     "join://:SECRETjoinAPIKEY@join/?devices=device",
		secrets: []string{"SECRETjoinAPIKEY"},
	},
	{
		scheme:  "lark",
		url:     "lark://open.larksuite.com/SECRETlarkTOKEN?secret=SECRETlarkSIGN",
		secrets: []string{"SECRETlarkTOKEN", "SECRETlarkSIGN"},
		body:    `{"code":0,"msg":"success"}`,
	},
	{
		scheme:  "logger",
		url:     "logger://",
		kind:    netNone,
		secrets: nil,
	},
	{
		scheme:  "matrix",
		url:     "matrix://contract:SECRETmatrixPASSWORD@matrix.example.invalid",
		secrets: []string{"SECRETmatrixPASSWORD"},
	},
	{
		scheme:  "mattermost",
		url:     "mattermost://contract@mattermost.example.invalid/SECRETmattermostTOKEN",
		secrets: []string{"SECRETmattermostTOKEN"},
	},
	{
		scheme:  "mqtt",
		url:     "mqtt://contract:SECRETmqttPASSWORD@broker.example.invalid:1883/contract/topic",
		kind:    netTCP,
		secrets: []string{"SECRETmqttPASSWORD"},
	},
	{
		scheme:  "mqtts",
		url:     "mqtts://contract:SECRETmqttsPASSWORD@broker.example.invalid:8883/contract/topic",
		kind:    netTCP,
		secrets: []string{"SECRETmqttsPASSWORD"},
	},
	{
		scheme:  "notifiarr",
		url:     "notifiarr://SECRETnotifiarrAPIKEY",
		secrets: []string{"SECRETnotifiarrAPIKEY"},
	},
	{
		scheme:  "ntfy",
		url:     "ntfy://contract:SECRETntfyPASSWORD@ntfy.example.invalid/topic",
		secrets: []string{"SECRETntfyPASSWORD"},
	},
	{
		scheme:  "opsgenie",
		url:     "opsgenie://opsgenie.example.invalid/SECRETopsgenieKEY?responders=user:dummy",
		secrets: []string{"SECRETopsgenieKEY"},
	},
	{
		scheme:  "pagerduty",
		url:     "pagerduty://events.example.invalid/5ec7e75ec7e75ec7e75ec7e75ec7e700",
		secrets: []string{"5ec7e75ec7e75ec7e75ec7e75ec7e700"},
		status:  http.StatusAccepted,
	},
	{
		scheme:  "pushbullet",
		url:     "pushbullet://SECRETpushbulletTOKENxxxxxxxxxxxxx",
		secrets: []string{"SECRETpushbulletTOKENxxxxxxxxxxxxx"},
		body:    `{"type":"note","body":"contract","title":"contract","active":true,"created":0}`,
	},
	{
		scheme:  "pushover",
		url:     "pushover://:SECRETpushoverTOKEN@SECRETpushoverUSER/?devices=device",
		secrets: []string{"SECRETpushoverTOKEN", "SECRETpushoverUSER"},
	},
	{
		scheme:  "rocketchat",
		url:     "rocketchat://rocketchat.example.invalid/SECRETrocketTOKEN/channel",
		secrets: []string{"SECRETrocketTOKEN"},
	},
	{
		scheme:  "signal",
		url:     "signal://signal.example.invalid:8080/+15551234567/+15559876543?token=SECRETsignalTOKEN",
		secrets: []string{"SECRETsignalTOKEN"},
		body:    `{"timestamp":1}`,
	},
	{
		scheme:  "signalgrid",
		url:     "signalgrid://SECRETsignalgridKEY@channel",
		secrets: []string{"SECRETsignalgridKEY"},
	},
	{
		scheme:  "slack",
		url:     "slack://AAAAAAAAA/BBBBBBBBB/SECRETslackTOKENxxxxxxxx",
		secrets: []string{"SECRETslackTOKENxxxxxxxx"},
	},
	{
		scheme:  "smtp",
		url:     "smtp://contract:SECRETsmtpPASSWORD@mail.example.invalid:587/?fromAddress=from@example.invalid&toAddresses=to@example.invalid",
		kind:    netTCP,
		secrets: []string{"SECRETsmtpPASSWORD"},
	},
	{
		scheme: "teams",
		url: "teams://?host=https%3A%2F%2Fprod-00.westus.logic.azure.com%3A443%2Fworkflows%2F00000000000000000000000000000000" +
			"%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D2016-06-01%26sp%3D%252Ftriggers%252Fmanual%252Frun" +
			"%26sv%3D1.0%26sig%3DSECRETteamsSIG",
		secrets: []string{"SECRETteamsSIG"},
	},
	{
		scheme:  "telegram",
		url:     "telegram://123456789:SECRETtelegramTOKENxxxxxxxxxxxxxxxx@telegram?chats=contract",
		secrets: []string{"SECRETtelegramTOKENxxxxxxxxxxxxxxxx"},
		body:    `{"ok":true,"result":{"message_id":1}}`,
	},
	{
		scheme:  "twilio",
		url:     "twilio://ACaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:SECRETtwilioTOKEN@+15551234567/+15559876543",
		secrets: []string{"SECRETtwilioTOKEN"},
		status:  http.StatusCreated,
		body:    `{"sid":"SM123"}`,
	},
	{
		scheme:  "wecom",
		url:     "wecom://SECRETwecomKEY",
		secrets: []string{"SECRETwecomKEY"},
		body:    `{"errcode":0,"errmsg":"ok"}`,
	},
	{
		scheme:  "xmpp",
		url:     "xmpp://contract:SECRETxmppPASSWORD@xmpp.example.invalid/?to=bob@example.invalid",
		kind:    netTCP,
		secrets: []string{"SECRETxmppPASSWORD"},
	},
	{
		scheme:  "xmpps",
		url:     "xmpps://contract:SECRETxmppsPASSWORD@xmpp.example.invalid/?to=bob@example.invalid",
		kind:    netTCP,
		secrets: []string{"SECRETxmppsPASSWORD"},
	},
	{
		scheme:  "zulip",
		url:     "zulip://bot%40example.invalid:SECRETzulipKEY@zulip.example.invalid/?stream=foo&topic=bar",
		secrets: []string{"SECRETzulipKEY"},
	},
}

// allowlist records known contract failures by check and scheme, each tagged with
// its audit finding. Fixing a failure must remove its entry, because
// TestIntegrationContract fails if an allowlisted case passes.
var allowlist = map[string]map[string]knownFailure{
	checkSendUsesContext: {
		// MQTT keeps one connection across sends, so its dials use the connection's
		// lifetime context by design.
		"mqtt":  {"MQTT dials with the connection's lifetime context, which outlives one send", failContextDropped},
		"mqtts": {"MQTT dials with the connection's lifetime context, which outlives one send", failContextDropped},
	},
}

var (
	errTransport        = errors.New("contract: transport failure")
	errDial             = errors.New("contract: dial blocked")
	errDefaultTransport = errors.New("contract: http.DefaultTransport must not be used")
	errLookupBlocked    = errors.New("contract: DNS lookups are blocked")
	errHoldReleased     = errors.New("contract: held operation released when the test ended")
)

var (
	// defaultTransportHits counts uses of http.DefaultTransport across the package.
	defaultTransportHits atomic.Int64
	// lookupHits counts blocked DNS lookups, which reveal requests made through
	// transports other than http.DefaultTransport.
	lookupHits atomic.Int64
)

// TestMain blocks every path to the real network for this package. Requests through
// http.DefaultTransport are counted and refused, and DNS lookups fail, so a service
// that bypasses the injected client or dialer is detected and never leaves the host.
// Every test in the package runs under these fakes, so router tests inject their
// HTTP clients and dialers.
func TestMain(m *testing.M) {
	http.DefaultTransport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		defaultTransportHits.Add(1)

		return nil, errDefaultTransport
	})

	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(context.Context, string, string) (net.Conn, error) {
			lookupHits.Add(1)

			return nil, errLookupBlocked
		},
	}

	// Initialize the net package's lazily created resolver state outside any
	// synctest bubble; a first lookup inside a bubble would bind it to that bubble.
	_, _ = net.DefaultResolver.LookupHost(context.Background(), "contract-warmup.invalid")

	os.Exit(m.Run())
}

// TestIntegrationContract runs each contract check against every registered
// scheme.
//
// Known failures are listed in [allowlist]. They are reported as skips, and an
// allowlisted case that starts passing fails, so each fix must remove its entry.
//
//nolint:paralleltest // Swaps http.DefaultTransport and counts its use per case.
func TestIntegrationContract(t *testing.T) {
	checkFixtureCoverage(t)

	checks := []struct {
		name string
		run  func(t *testing.T, fx *fixture) failure
	}{
		{checkInitializeNoIO, runInitializeNoIO},
		{checkSendUsesInjection, runSendUsesInjection},
		{checkReservedParams, runReservedParams},
		{checkParamsDoNotPersist, runParamsDoNotPersist},
		{checkNilHTTPClient, runNilHTTPClient},
		{checkTypedNilHTTPClient, runTypedNilHTTPClient},
		{checkNoSecretLeaks, runNoSecretLeaks},
		{checkNoSecretLeaksDefault, runNoSecretLeaksDefault},
		{checkSendUsesContext, runSendUsesContext},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			for i := range fixtures {
				fx := &fixtures[i]

				t.Run(fx.scheme, func(t *testing.T) {
					result := check.run(t, fx)
					assertContract(t, check.name, fx.scheme, result)
				})
			}
		})
	}
}

// TestIntegrationIPv6Hosts verifies that every service that connects to its
// URL's host accepts an IPv6 address in brackets, with or without a port,
// keeps the brackets and port in its config URL so the config survives a round
// trip, and rejects an unbracketed IPv6 address, which url.Parse splits at its
// last colon.
func TestIntegrationIPv6Hosts(t *testing.T) {
	t.Parallel()

	// ipv6Schemes lists the schemes whose URL host is the server they connect to.
	ipv6Schemes := []string{
		"homeassistant", "mqtt", "mqtts", "opsgenie", "pagerduty", "rocketchat", "signal", "smtp", "xmpp", "xmpps",
	}

	for _, scheme := range ipv6Schemes {
		fixtureURL := fixtureURLFor(t, scheme)

		for _, host := range []string{"[2001:db8::1]", "[2001:db8::1]:8443"} {
			t.Run(scheme+" "+host, func(t *testing.T) {
				t.Parallel()

				serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
				require.NoError(t, err)

				service, err := serviceRouter.Locate(withURLHost(t, fixtureURL, host))
				closeService(t, service)
				require.NoError(t, err)

				first := format.GetServiceConfig(service).GetURL()
				assert.Equal(t, "2001:db8::1", first.Hostname())
				assert.True(t, strings.HasPrefix(first.Host, "[2001:db8::1]"), "host %q lost its brackets", first.Host)

				if strings.HasSuffix(host, ":8443") {
					assert.Equal(t, "8443", first.Port())
				}

				restored, err := serviceRouter.Locate(first.String())
				closeService(t, restored)
				require.NoError(t, err)
				assert.Equal(t, first.Host, format.GetServiceConfig(restored).GetURL().Host)
			})
		}

		t.Run(scheme+" unbracketed", func(t *testing.T) {
			t.Parallel()

			serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
			require.NoError(t, err)

			service, err := serviceRouter.Locate(withURLHost(t, fixtureURL, "2001:db8::1"))
			closeService(t, service)
			require.ErrorIs(t, err, util.ErrInvalidHost)
		})
	}
}

// TestIntegrationMissingHost verifies how each service treats a URL without a
// host: the services with a documented default host use it, keep it in their
// config URL, and survive a round trip, while the services without one reject
// the URL.
func TestIntegrationMissingHost(t *testing.T) {
	t.Parallel()

	// defaultHosts lists the schemes that apply a default host, with that host.
	defaultHosts := map[string]string{
		"mqtt": "localhost", "mqtts": "localhost", "ntfy": "ntfy.sh", "signal": "localhost",
	}

	for scheme, wantHost := range defaultHosts {
		t.Run(scheme+" default", func(t *testing.T) {
			t.Parallel()

			serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
			require.NoError(t, err)

			service, err := serviceRouter.Locate(withURLHost(t, fixtureURLFor(t, scheme), ""))
			closeService(t, service)
			require.NoError(t, err)

			first := format.GetServiceConfig(service).GetURL()
			assert.Equal(t, wantHost, first.Hostname())

			restored, err := serviceRouter.Locate(first.String())
			closeService(t, restored)
			require.NoError(t, err)
			assert.Equal(t, first.Host, format.GetServiceConfig(restored).GetURL().Host)
		})
	}

	// requiredHostSchemes lists the schemes whose URL must name a host.
	requiredHostSchemes := []string{"generic", "gotify", "mattermost", "notifiarr", "rocketchat", "smtp"}

	for _, scheme := range requiredHostSchemes {
		t.Run(scheme+" rejected", func(t *testing.T) {
			t.Parallel()

			serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
			require.NoError(t, err)

			service, err := serviceRouter.Locate(withURLHost(t, fixtureURLFor(t, scheme), ""))
			closeService(t, service)
			require.Error(t, err)
		})
	}
}

// TestIntegrationMalformedQuery verifies that every service that reads its URL
// query rejects a query pair with a malformed escape or a literal semicolon,
// which url.URL.Query would drop without an error, including in the webhook URL
// of a custom URL. ntfy reads a literal semicolon as part of the value, which
// keeps its semicolon-separated actions working, so it accepts the semicolon.
func TestIntegrationMalformedQuery(t *testing.T) {
	t.Parallel()

	// queryless lists the schemes that do not read their URL query.
	queryless := []string{"logger", "rocketchat"}

	// semicolonValueSchemes lists the schemes that read a literal semicolon as
	// part of the value.
	semicolonValueSchemes := []string{"ntfy"}

	// pairs lists the malformed query pairs by name.
	pairs := map[string]string{"escape": "title=%zz", "semicolon": "title=a;b"}

	for pairName, pair := range pairs {
		// rawURLs lists the service URLs to test by scheme: each fixture with the
		// pair added to its query, and the custom URLs.
		rawURLs := map[string]string{
			"generic custom":   "generic+https://hooks.example.invalid/webhook?" + pair,
			"notifiarr custom": "notifiarr+https://notifiarr.example.invalid/api?" + pair,
		}

		for _, fx := range fixtures {
			if slices.Contains(queryless, fx.scheme) {
				continue
			}

			serviceURL, err := url.Parse(fx.url)
			require.NoError(t, err)

			if serviceURL.RawQuery != "" {
				serviceURL.RawQuery += "&"
			}

			serviceURL.RawQuery += pair
			rawURLs[fx.scheme] = serviceURL.String()
		}

		for scheme, rawURL := range rawURLs {
			t.Run(scheme+" "+pairName, func(t *testing.T) {
				t.Parallel()

				serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
				require.NoError(t, err)

				service, err := serviceRouter.Locate(rawURL)
				closeService(t, service)

				if pairName == "semicolon" && slices.Contains(semicolonValueSchemes, scheme) {
					require.NoError(t, err)

					return
				}

				require.ErrorIs(t, err, util.ErrInvalidQuery)
			})
		}
	}
}

// TestIntegrationResponseBodyLimit verifies that no HTTP service reads more of
// a response body than [httpbody.MaxSize] plus the one byte that detects an
// oversized body. Every response carries the fixture's body followed by spaces
// up to [oversizedBodySize]. The check runs with the fixture's status and with
// an error status, because some services read the body only on an error. A
// send that gets the error status must still fail.
func TestIntegrationResponseBodyLimit(t *testing.T) {
	t.Parallel()

	for _, fx := range fixtures {
		if fx.kind != netHTTP {
			continue
		}

		for _, status := range []int{cmp.Or(fx.status, http.StatusOK), http.StatusInternalServerError} {
			t.Run(fmt.Sprintf("%s %d", fx.scheme, status), func(t *testing.T) {
				t.Parallel()

				var (
					mu      sync.Mutex
					counts  []*atomic.Int64
					sendErr error
				)

				client := httpClientFunc(func(req *http.Request) (*http.Response, error) {
					if req.Body != nil {
						_, _ = io.Copy(io.Discard, req.Body)
						_ = req.Body.Close()
					}

					read := &atomic.Int64{}

					mu.Lock()

					counts = append(counts, read)

					mu.Unlock()

					content := io.MultiReader(
						strings.NewReader(cmp.Or(fx.body, "{}")),
						io.LimitReader(spaceReader{}, oversizedBodySize),
					)

					return &http.Response{
						StatusCode: status,
						Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(&countedBody{r: content, read: read}),
						Request:    req,
					}, nil
				})

				// Services that retry an error status wait between attempts, so the
				// send runs on synctest's fake clock.
				synctest.Test(t, func(t *testing.T) {
					serviceRouter, err := router.NewWithOptions(
						log.New(io.Discard, "", 0),
						types.SenderOptions{HTTPClient: client},
					)
					require.NoError(t, err)

					service, err := serviceRouter.Locate(fx.url)
					require.NoError(t, err)

					defer closeService(t, service)

					sendErr = service.Send("contract message", nil)
				})

				// A capped read must not turn a failed request into a success.
				if status >= http.StatusBadRequest {
					require.Error(t, sendErr, "the send succeeded despite status %d", status)
				}

				mu.Lock()
				defer mu.Unlock()

				require.NotEmpty(t, counts, "the service made no request")

				for i, read := range counts {
					assert.LessOrEqual(t, read.Load(), int64(httpbody.MaxSize)+1, "response %d", i)
				}
			})
		}
	}
}

// TestIntegrationMaskedConfig verifies that rendering a service's config with
// [format.ConsoleTreeRenderer.MaskSensitive], as the CLI's verify command does,
// shows none of the fixture's secrets, and that each secret is still shown
// without masking. A secret that survives masking is held by a field without a
// `sensitive:"true"` tag.
func TestIntegrationMaskedConfig(t *testing.T) {
	t.Parallel()

	for _, fx := range fixtures {
		if len(fx.secrets) == 0 {
			continue
		}

		t.Run(fx.scheme, func(t *testing.T) {
			t.Parallel()

			serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
			require.NoError(t, err)

			service, err := serviceRouter.Locate(fx.url)
			require.NoError(t, err)
			closeService(t, service)

			node := format.GetConfigFormat(format.GetServiceConfig(service))
			masked := format.ConsoleTreeRenderer{WithValues: true, MaskSensitive: true}.RenderTree(node, "")
			plain := format.ConsoleTreeRenderer{WithValues: true, MaskSensitive: false}.RenderTree(node, "")

			for _, secret := range fx.secrets {
				assert.NotContains(t, masked, secret, "the masked config shows a secret")

				// Secrets held outside the config tree, such as generic's custom
				// headers, never appear in it.
				if fx.scheme != "generic" {
					assert.Contains(t, plain, secret, "the unmasked config hides a secret")
				}
			}
		})
	}
}

// fixtureURLFor returns the contract fixture URL for scheme.
//
// Parameters:
//   - t: the test.
//   - scheme: the service scheme.
//
// Returns:
//   - string: the fixture's service URL.
func fixtureURLFor(t *testing.T, scheme string) string {
	t.Helper()

	for _, fx := range fixtures {
		if fx.scheme == scheme {
			return fx.url
		}
	}

	t.Fatalf("no fixture for scheme %q", scheme)

	return ""
}

// withURLHost returns rawURL with its host and port replaced by host.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - host: the new URL authority, such as [2001:db8::1]:8443.
//
// Returns:
//   - string: the service URL with the new host.
func withURLHost(t *testing.T, rawURL, host string) string {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	serviceURL.Host = host

	return serviceURL.String()
}

// checkFixtureCoverage fails when a registered scheme has no fixture, so new
// services are covered by the contract from the start.
func checkFixtureCoverage(t *testing.T) {
	t.Helper()

	covered := make(map[string]bool, len(fixtures))
	for _, fx := range fixtures {
		covered[fx.scheme] = true
	}

	for _, scheme := range router.SupportedSchemas() {
		if !covered[scheme] {
			t.Errorf("scheme %q has no contract fixture; add one to fixtures", scheme)
		}
	}
}

// assertContract reports a check result, honoring the allowlist. A known failure
// is skipped only when it fails for its declared kind; any other failure is reported.
func assertContract(t *testing.T, check, scheme string, result failure) {
	t.Helper()

	known, isKnown := allowlist[check][scheme]

	switch {
	case result.kind != failNone && isKnown && result.kind == known.kind:
		t.Skipf("known failure (%s): %s", known.finding, result.detail)
	case result.kind != failNone && isKnown:
		t.Errorf("failed for an unexpected reason (allowlisted for %s): %s", known.finding, result.detail)
	case result.kind != failNone:
		t.Error(result.detail)
	case isKnown:
		t.Errorf("%s now passes for %q; remove its allowlist entry (%s)", check, scheme, known.finding)
	}
}

// fail builds a failure with a formatted detail message.
func fail(kind failureKind, msg string, args ...any) failure {
	return failure{kind: kind, detail: fmt.Sprintf(msg, args...)}
}

// runInitializeNoIO verifies that building a service performs no network I/O.
func runInitializeNoIO(t *testing.T, fx *fixture) failure {
	t.Helper()

	var (
		env           *env
		defaultBefore int64
		lookupBefore  int64
		initErr       error
	)

	// Count I/O only after synctest.Test returns, when every goroutine started
	// during Initialize has exited.
	synctest.Test(t, func(t *testing.T) {
		env = newEnv(t, fx, false)
		defaultBefore, lookupBefore = defaultTransportHits.Load(), lookupHits.Load()

		_, initErr = env.locate(t)
	})

	hits := int64(env.ioCount()) +
		defaultTransportHits.Load() - defaultBefore +
		lookupHits.Load() - lookupBefore
	if hits > 0 {
		return fail(failInitIO, "performed %d network operation(s) during Initialize", hits)
	}

	if initErr != nil {
		t.Fatalf("fixture %q failed to initialize: %v", fx.url, initErr)
	}

	return failure{}
}

// runSendUsesInjection verifies that Send goes through the injected client or dialer.
func runSendUsesInjection(t *testing.T, fx *fixture) failure {
	t.Helper()

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)

		service, err := env.locate(t)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		httpBefore, dialBefore := env.httpCalls(), env.dials.Load()
		before := defaultTransportHits.Load()

		if result = env.send(service, nil); result.kind != failNone {
			return
		}

		httpUsed := env.httpCalls() > httpBefore
		dialUsed := env.dials.Load() > dialBefore
		defaultUsed := defaultTransportHits.Load() > before

		switch {
		case fx.kind == netNone && (httpUsed || dialUsed || defaultUsed):
			result = fail(failUnexpectedIO, "Send performed network I/O for a service without network access")
		case fx.kind == netNone:
		case defaultUsed:
			result = fail(failDefaultTransport, "Send used http.DefaultTransport instead of the injected client")
		case fx.kind == netTCP && !dialUsed, fx.kind == netHTTP && !httpUsed:
			result = fail(failNotInjected, "Send did not use the injected client or dialer")
		}
	})

	return result
}

// runSendUsesContext verifies that the context passed to the router's SendContext
// reaches every request and dial the service makes, and that canceling it ends
// them. Marked requests and dials wait until their context ends, and the caller
// cancels while they wait.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failContextDropped, failCancelIgnored, failNotInjected, or failInit
//     failure, or the zero value.
func runSendUsesContext(t *testing.T, fx *fixture) failure {
	t.Helper()

	if fx.kind == netNone {
		return failure{}
	}

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)
		env.holdMarked = true

		serviceRouter, err := router.NewWithOptions(
			env.logger,
			types.SenderOptions{HTTPClient: env.client, DialContext: env.dial},
			fx.url,
		)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		t.Cleanup(func() { _ = serviceRouter.Close() })

		ctx, cancel := context.WithCancel(context.WithValue(t.Context(), contextMarker{}, fx.scheme))
		defer cancel()

		time.AfterFunc(cancelAfter, cancel)

		_ = serviceRouter.SendContext(ctx, "contract message", nil)

		// The router returns when the caller cancels, so wait for the send itself to finish.
		synctest.Wait()

		requests, marked := 0, 0

		for i := range env.client.Calls {
			req, ok := env.client.Calls[i].Arguments.Get(0).(*http.Request)
			if !ok {
				continue
			}

			requests++

			if req.Context().Value(contextMarker{}) == fx.scheme {
				marked++
			}
		}

		dials, markedDials := int(env.dials.Load()), int(env.markedDials.Load())
		canceled := int(env.canceledOps.Load())

		switch {
		case requests+dials == 0:
			result = fail(failNotInjected, "SendContext did not use the injected client or dialer")
		case marked < requests || markedDials < dials:
			result = fail(
				failContextDropped,
				"%d of %d request(s) and %d of %d dial(s) carried the caller's context",
				marked, requests, markedDials, dials,
			)
		case canceled < marked+markedDials:
			result = fail(
				failCancelIgnored,
				"%d of %d request(s) and dial(s) ended when the caller canceled",
				canceled, marked+markedDials,
			)
		}
	})

	return result
}

// runReservedParams verifies that the reserved title, message and level params
// never fail a send as unknown config keys.
func runReservedParams(t *testing.T, fx *fixture) failure {
	t.Helper()

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)

		service, err := env.locate(t)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		params := types.Params{}
		params.SetTitle("contract title")
		params.SetMessage("contract message")
		params.SetLevel(types.Warning)

		if result = env.send(service, &params); result.kind != failNone {
			return
		}

		if errors.Is(env.sendErr, format.ErrInvalidConfigKey) ||
			(env.sendErr != nil && strings.Contains(env.sendErr.Error(), format.ErrInvalidConfigKey.Error())) {
			result = fail(failRejectedParams, "rejected reserved params: %v", env.sendErr)
		}
	})

	return result
}

// runParamsDoNotPersist verifies that params passed to Send apply to that send
// only: the service's exported Config is the same before and after the send.
// Services without an exported Config field are not checked.
func runParamsDoNotPersist(t *testing.T, fx *fixture) failure {
	t.Helper()

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)

		service, err := env.locate(t)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		before, err := configSnapshot(service)
		if err != nil {
			t.Fatalf("snapshotting config: %v", err)
		}

		if before == nil {
			return
		}

		params := types.Params{}
		params.SetTitle("contract title")

		if result = env.send(service, &params); result.kind != failNone {
			return
		}

		after, err := configSnapshot(service)
		if err != nil {
			t.Fatalf("snapshotting config: %v", err)
		}

		if !bytes.Equal(before, after) {
			result = fail(failParamsPersisted, "Send with params changed the service config")
		}
	})

	return result
}

// configSnapshot encodes the configuration a service exposes in its exported Config
// field. The JSON encoding is a deep copy, so it also captures slices and maps a
// send might change in place. Params set only exported fields, which it covers.
//
// Parameters:
//   - service: the service to inspect.
//
// Returns:
//   - []byte: the encoded configuration, or nil when there is none to check.
//   - error: the encoding failure.
func configSnapshot(service types.Service) ([]byte, error) {
	value := reflect.ValueOf(service)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return nil, nil
	}

	field := value.FieldByName("Config")
	if !field.IsValid() || field.Kind() != reflect.Pointer || field.IsNil() {
		return nil, nil
	}

	encoded, err := json.Marshal(field.Interface())
	if err != nil {
		return nil, fmt.Errorf("encoding config: %w", err)
	}

	return encoded, nil
}

// runNilHTTPClient verifies that SetHTTPClient(nil) restores a usable default
// client: Send must not panic, must stop using the injected client, and must make
// its request through the default client. That request is observed either on
// http.DefaultTransport or as a blocked DNS lookup from a dedicated transport.
func runNilHTTPClient(t *testing.T, fx *fixture) failure {
	t.Helper()

	return runResetHTTPClient(t, fx, nil)
}

// runTypedNilHTTPClient verifies that a nil *http.Client restores the default
// client the same way an untyped nil does.
func runTypedNilHTTPClient(t *testing.T, fx *fixture) failure {
	t.Helper()

	return runResetHTTPClient(t, fx, (*http.Client)(nil))
}

// runResetHTTPClient injects a client, resets it with SetHTTPClient(reset), and
// checks the send as described in [runNilHTTPClient].
func runResetHTTPClient(t *testing.T, fx *fixture, reset types.HTTPClient) failure {
	t.Helper()

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)

		service, err := env.locate(t)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		setter, ok := service.(types.HTTPClientSetter)
		if !ok {
			return
		}

		setter.SetHTTPClient(reset)

		httpBefore := env.httpCalls()
		defaultBefore, lookupBefore := defaultTransportHits.Load(), lookupHits.Load()

		if result = env.send(service, nil); result.kind != failNone {
			return
		}

		fallbackUsed := defaultTransportHits.Load() > defaultBefore || lookupHits.Load() > lookupBefore

		switch {
		case env.httpCalls() > httpBefore:
			result = fail(failInjectedAfterNil, "Send still used the injected client after SetHTTPClient(nil)")
		case fx.kind == netHTTP && !fallbackUsed:
			result = fail(failNoFallbackRequest, "Send made no request through the default client after SetHTTPClient(nil)")
		}
	})

	return result
}

// runNoSecretLeaks verifies that secrets from the service URL never appear in
// errors or logs when the transport fails. The injected client fails the way
// net/http does, with a *url.Error that carries the full request URL.
func runNoSecretLeaks(t *testing.T, fx *fixture) failure {
	t.Helper()

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, true)

		service, err := env.locate(t)
		result = env.checkLeaks(service, err)
	})

	return result
}

// runNoSecretLeaksDefault verifies that secrets never appear in errors or logs when
// a service uses its own default HTTP client. Requests through that client fail at
// the blocked http.DefaultTransport or DNS resolver, and net/http reports them
// with a *url.Error that carries the full request URL. An HTTP fixture must make
// that request, observed as in [runNilHTTPClient], so the check cannot pass
// without exercising the default client.
func runNoSecretLeaksDefault(t *testing.T, fx *fixture) failure {
	t.Helper()

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)

		service, err := env.locateWithDefaultClient(t)
		defaultBefore, lookupBefore := defaultTransportHits.Load(), lookupHits.Load()

		if result = env.checkLeaks(service, err); result.kind != failNone || err != nil {
			return
		}

		requested := defaultTransportHits.Load() > defaultBefore || lookupHits.Load() > lookupBefore
		if fx.kind == netHTTP && !requested {
			result = fail(failNoDefaultRequest, "Send made no request through the default client")
		}
	})

	return result
}

// newEnv creates the injected dependencies for one case. The HTTP client answers
// every request with the fixture's response, or fails like net/http with a
// *url.Error carrying the full request URL when failTransport is set.
func newEnv(t *testing.T, fx *fixture, failTransport bool) *env {
	t.Helper()

	logs := &syncBuffer{}

	e := &env{
		fx:     fx,
		client: mocks.NewMockHTTPClient(t),
		logs:   logs,
		logger: log.New(logs, "", 0),
		stop:   t.Context().Done(),
	}

	answer := respond(fx, failTransport)

	e.client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		if err := e.holdIfMarked(req.Context()); err != nil {
			return nil, err
		}

		return answer(req)
	}).Maybe()

	return e
}

// respond returns the injected client's behavior for a fixture.
func respond(fx *fixture, failTransport bool) func(*http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		if req.Body != nil {
			_, _ = io.Copy(io.Discard, req.Body)
			_ = req.Body.Close()
		}

		if failTransport {
			return nil, &url.Error{Op: req.Method, URL: req.URL.String(), Err: errTransport}
		}

		status := fx.status
		if status == 0 {
			status = http.StatusOK
		}

		body := fx.body
		if body == "" {
			body = "{}"
		}

		return &http.Response{
			StatusCode: status,
			Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	}
}

// checkLeaks sends through service, when it initialized, and reports a secret from
// the fixture that appears in the resulting error or the captured logs.
//
// Parameters:
//   - service: the located service, or nil when initialization failed.
//   - initErr: the initialization error, if any.
//
// Returns:
//   - failure: a failSecretLeak or failPanic failure, or the zero value.
func (e *env) checkLeaks(service types.Service, initErr error) failure {
	err := initErr
	if err == nil {
		if result := e.send(service, nil); result.kind != failNone {
			return result
		}

		err = e.sendErr
	}

	output := e.logs.String()
	if err != nil {
		output += "\n" + err.Error()
	}

	for _, secret := range e.fx.secrets {
		if strings.Contains(output, secret) {
			return fail(failSecretLeak, "secret %q leaked into errors or logs", secret)
		}
	}

	return failure{}
}

// dial is the injected DialContext. It records and refuses every dial, and counts
// the dials whose context carries [contextMarker] for this fixture.
//
// Parameters:
//   - ctx: the dial's context.
//
// Returns:
//   - net.Conn: always nil.
//   - error: errDial, or the error from [env.holdIfMarked] when the dial was held.
func (e *env) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	e.dials.Add(1)

	if ctx.Value(contextMarker{}) == e.fx.scheme {
		e.markedDials.Add(1)
	}

	if err := e.holdIfMarked(ctx); err != nil {
		return nil, err
	}

	return nil, errDial
}

// holdIfMarked waits until ctx ends when holdMarked is set and ctx carries
// [contextMarker] for this fixture, and counts the wait in canceledOps when ctx
// ended by cancellation. A wait that outlasts the test ends with the test and is
// not counted.
//
// Parameters:
//   - ctx: the request or dial context.
//
// Returns:
//   - error: nil when the operation was not held, ctx's error when ctx ended, or
//     errHoldReleased when the test ended first.
func (e *env) holdIfMarked(ctx context.Context) error {
	if !e.holdMarked || ctx.Value(contextMarker{}) != e.fx.scheme {
		return nil
	}

	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.Canceled) {
			e.canceledOps.Add(1)
		}

		return ctx.Err()
	case <-e.stop:
		return errHoldReleased
	}
}

// httpCalls returns how many requests reached the injected client.
func (e *env) httpCalls() int { return len(e.client.Calls) }

func (e *env) ioCount() int { return e.httpCalls() + int(e.dials.Load()) }

// locate builds the service through the router, so injection follows the same
// path consumers use.
func (e *env) locate(t *testing.T) (types.Service, error) {
	t.Helper()

	return e.locateWith(t, types.SenderOptions{HTTPClient: e.client, DialContext: e.dial})
}

// locateWith builds the service through the router with the given options. A
// service that holds a connection is closed when the test ends, as consumers do.
func (e *env) locateWith(t *testing.T, opts types.SenderOptions) (types.Service, error) {
	t.Helper()

	serviceRouter, err := router.NewWithOptions(e.logger, opts)
	if err != nil {
		t.Fatalf("creating router: %v", err)
	}

	service, err := serviceRouter.Locate(e.fx.url)
	if err != nil {
		return nil, fmt.Errorf("locating service: %w", err)
	}

	if closer, ok := service.(io.Closer); ok {
		t.Cleanup(func() { _ = closer.Close() })
	}

	return service, nil
}

// locateWithDefaultClient builds the service through the router without injecting
// an HTTP client, so the service uses its own default client.
func (e *env) locateWithDefaultClient(t *testing.T) (types.Service, error) {
	t.Helper()

	return e.locateWith(t, types.SenderOptions{DialContext: e.dial})
}

// send sends a message and returns a failPanic failure if Send panicked.
func (e *env) send(service types.Service, params *types.Params) failure {
	e.trySend(service, params)

	if e.panicMsg != "" {
		return fail(failPanic, "%s", e.panicMsg)
	}

	return failure{}
}

func (e *env) trySend(service types.Service, params *types.Params) {
	e.panicMsg = ""

	defer func() {
		if r := recover(); r != nil {
			e.panicMsg = fmt.Sprintf("panicked: %v", r)
		}
	}()

	e.sendErr = service.Send("contract message", params)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// Do calls f with the request.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: f's response.
//   - error: f's error.
func (f httpClientFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

// Read fills p with spaces.
//
// Parameters:
//   - p: the buffer to fill.
//
// Returns:
//   - int: the length of p.
//   - error: always nil.
func (spaceReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}

	return len(p), nil
}

// Read reads from the body's content and counts the bytes.
//
// Parameters:
//   - p: the buffer to read into.
//
// Returns:
//   - int: the number of bytes read.
//   - error: the content's error, such as [io.EOF] at its end.
func (b *countedBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.read.Add(int64(n))

	return n, err
}
