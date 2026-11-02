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
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"regexp"
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
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	mockTypes "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
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
	// serverKey is the query key that holds the base URL of the server the
	// service sends to, for a service that does not send to its URL host.
	serverKey string
}

// env holds the injected client, dialer and logger for one contract case.
type env struct {
	fx     *fixture
	client *mockTypes.MockHTTPClient
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

// logFunc adapts a function that receives a warning's message to
// [logging.Logger], as its own slog handler.
type logFunc func(msg string)

// sendOutcome is the result of a send that may panic.
type sendOutcome struct {
	// err is the send's error.
	err error
	// panicMsg describes the panic, or is empty when Send returned.
	panicMsg string
}

// skipVerifyURLs are the service URLs for a server whose certificate cannot be
// verified.
type skipVerifyURLs struct {
	// base points at the server without skipping verification.
	base string
	// skip is base with skiptlsverify=yes.
	skip string
}

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
	// checkNilPointerHTTPClient verifies that an injected client that is a nil
	// pointer of another type fails the send with types.ErrNilHTTPClient.
	checkNilPointerHTTPClient = "nil_pointer_http_client"
	// checkClientFieldOrder verifies that a service's exported client field and
	// SetHTTPClient follow last-write-wins.
	checkClientFieldOrder = "client_field_order"
	checkNoSecretLeaks    = "no_secret_leaks"
	// checkNoSecretLeaksDefault repeats the leak check with each service's own
	// default HTTP client, as used by consumers that do not inject one.
	checkNoSecretLeaksDefault = "no_secret_leaks_default_client"
	// checkSendUsesContext verifies that the context passed to the router's
	// SendContext reaches every request and dial a service makes.
	checkSendUsesContext = "send_uses_context"
	// checkStandardKeyNames verifies that TLS options use the standard key names.
	checkStandardKeyNames = "standard_key_names"
	// checkKeyDescriptions verifies that every config key has a description.
	checkKeyDescriptions = "key_descriptions"
	// checkURLParts verifies that every url tag on a config field names a part
	// of a URL.
	checkURLParts = "url_parts"
	// checkDeprecatedFields verifies that deprecated config fields have valid
	// tags and reach the config URL under their replacement's key.
	checkDeprecatedFields = "deprecated_fields"
	// checkSendBudget verifies that a send without a deadline is bounded by the
	// service's own send budget.
	checkSendBudget = "send_budget"
	// checkSendKeepsDeadline verifies that a caller's deadline reaches every
	// request and dial of a send unchanged.
	checkSendKeepsDeadline = "send_keeps_deadline"
	// checkPlaintextWarning verifies that a service with a disabletls option
	// warns once when its sends go out without TLS.
	checkPlaintextWarning = "plaintext_warning"
	// checkLeveledLogs verifies that a service logs only leveled records at
	// Debug, Info, or Warn, without "Warning:" in their messages.
	checkLeveledLogs = "leveled_logs"
	// checkConflictingKeys verifies that a URL giving one config key in two
	// spellings is rejected.
	checkConflictingKeys = "conflicting_keys"
	// checkUnknownParams verifies that a send param that is neither a config
	// key nor reserved fails the send.
	checkUnknownParams = "unknown_params"
	// checkCheckedConfig verifies that a send whose config fails the checks of
	// reading its URL fails without a request or dial.
	checkCheckedConfig = "checked_config"
)

// Checks that run against the services that let users skip TLS certificate
// verification over HTTP.
const (
	// checkSkipTLSVerifyURL verifies that skiptlsverify in the URL reaches a
	// server with an unverifiable certificate, and that nothing does without it.
	checkSkipTLSVerifyURL = "skip_tls_verify_url"
	// checkSkipTLSVerifyParams verifies that skiptlsverify in the send params
	// reaches a server with an unverifiable certificate.
	checkSkipTLSVerifyParams = "skip_tls_verify_params"
	// checkInsecureWarning verifies that the shared warning is logged once for
	// the insecure default client and never for an injected client.
	checkInsecureWarning = "insecure_warning"
)

// Failure kinds, one per distinct way a check can fail.
const (
	failNone                 failureKind = iota // the check passed
	failInit                                    // the fixture failed to initialize
	failInitIO                                  // Initialize performed network I/O
	failPanic                                   // Send panicked
	failDefaultTransport                        // Send used http.DefaultTransport
	failNotInjected                             // Send skipped the injected client or dialer
	failUnexpectedIO                            // a service without network access performed I/O
	failRejectedParams                          // reserved params were rejected as unknown keys
	failInjectedAfterNil                        // the injected client was still used after SetHTTPClient(nil)
	failNoFallbackRequest                       // no request went through the default client after SetHTTPClient(nil)
	failSecretLeak                              // a secret appeared in errors or logs
	failParamsPersisted                         // Send with params changed the service configuration
	failNoDefaultRequest                        // no request went through the service's default client
	failContextDropped                          // a request or dial did not carry the caller's context
	failCancelIgnored                           // a request or dial did not end when the caller canceled
	failNonStandardKey                          // a TLS option uses a key name other than the standard one
	failMissingDescription                      // a config key has no description
	failInvalidURLPart                          // a url tag names no part of a URL
	failDeprecatedField                         // a deprecated field has an invalid tag or is lost in the config URL
	failBudget                                  // a request or dial lacked the service's send budget as its deadline
	failUnverifiedAccepted                      // a request reached a server whose certificate was not verified
	failSkipIgnored                             // skiptlsverify did not let a request reach the server
	failWarningCount                            // the insecure warning was not logged exactly once
	failWarnedInjected                          // the insecure warning was logged for an injected client
	failClientFieldOrder                        // a send used the wrong one of the client field and the injected client
	failDeadlineChanged                         // a request or dial did not carry the caller's deadline
	failNilClientUnreported                     // a send through a nil injected client did not report types.ErrNilHTTPClient
	failUnleveledLog                            // a service logged text without a level, or "Warning:" in a message
	failSendFailed                              // a send failed though its transport works
	failConflictAccepted                        // a URL with one config key in two spellings was accepted
	failUnknownParamAccepted                    // a send with an unknown param did not fail as an unknown config key
	failUncheckedConfig                         // a send with an invalid config succeeded or made a request or dial
)

// uncheckedDial is the reason for a checked_config allowlist entry whose send
// dials with a config that reading a URL would reject.
const uncheckedDial = "Checks its URL only when reading it, so a send with an invalid config still dials"

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
		body:    `{"type":"note","body":"contract message","title":"Shoutrrr notification","active":true,"created":0}`,
	},
	{
		scheme:  "pushover",
		url:     "pushover://:SECRETpushoverTOKEN@SECRETpushoverUSER/?devices=device",
		secrets: []string{"SECRETpushoverTOKEN", "SECRETpushoverUSER"},
		body:    `{"status":1}`,
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
		url:     "signalgrid://SECRETsignalgridKEY@SECRETsignalgridCHANNEL",
		secrets: []string{"SECRETsignalgridKEY", "SECRETsignalgridCHANNEL"},
	},
	{
		scheme:  "slack",
		url:     "slack://AAAAAAAAA/BBBBBBBBB/SECRETslackTOKENxxxxxxxx",
		secrets: []string{"SECRETslackTOKENxxxxxxxx"},
		body:    "ok",
	},
	{
		scheme:  "smtp",
		url:     "smtp://contract:SECRETsmtpPASSWORD@mail.example.invalid:587/?fromAddress=from@example.invalid&toAddresses=to@example.invalid",
		kind:    netTCP,
		secrets: []string{"SECRETsmtpPASSWORD"},
	},
	{
		// OAuth2 with a refresh token makes a token request through the
		// injected HTTP client before it dials the SMTP server.
		scheme: "smtp",
		url: "smtp://contract@mail.example.invalid:587/?fromAddress=from@example.invalid&toAddresses=to@example.invalid" +
			"&auth=OAuth2&clientid=contract-client&clientsecret=SECRETsmtpCLIENTSECRET&refreshtoken=SECRETsmtpREFRESHTOKEN" +
			"&tokenurl=https%3A%2F%2Foauth.example.invalid%2Ftoken",
		kind:    netTCP,
		secrets: []string{"SECRETsmtpCLIENTSECRET", "SECRETsmtpREFRESHTOKEN"},
		body:    `{"access_token":"contract-access-token","token_type":"Bearer","expires_in":3600}`,
	},
	{
		scheme: "teams",
		url: "teams://?host=https%3A%2F%2Fprod-00.westus.logic.azure.com%3A443%2Fworkflows%2F00000000000000000000000000000000" +
			"%2Ftriggers%2Fmanual%2Fpaths%2Finvoke%3Fapi-version%3D2016-06-01%26sp%3D%252Ftriggers%252Fmanual%252Frun" +
			"%26sv%3D1.0%26sig%3DSECRETteamsSIG",
		secrets: []string{"SECRETteamsSIG"},
	},
	{
		scheme:    "telegram",
		url:       "telegram://123456789:SECRETtelegramTOKENxxxxxxxxxxxxxxxx@telegram?chats=contract",
		secrets:   []string{"SECRETtelegramTOKENxxxxxxxxxxxxxxxx"},
		body:      `{"ok":true,"result":{"message_id":1}}`,
		serverKey: "apiurl",
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
	checkSendBudget: {
		"mqtt":  {"MQTT dials with the connection's lifetime context, which has no deadline", failBudget},
		"mqtts": {"MQTT dials with the connection's lifetime context, which has no deadline", failBudget},
		// Zulip's limits lookup is optional, so it is capped below the budget and
		// a slow lookup leaves the message request the rest of the budget.
		"zulip": {"Zulip caps its optional limits lookup at 10s by design", failBudget},
	},
	checkConflictingKeys: {
		// A custom query key reaches the webhook as written, and only the exact
		// config key sets a prop, so another spelling is a webhook parameter.
		"generic":   {"Generic passes a key in another case through to the webhook as a custom query parameter", failConflictAccepted},
		"notifiarr": {"Notifiarr passes a key in another case through to the webhook as a custom query parameter", failConflictAccepted},
	},
	checkLeveledLogs: {
		// The fixture's password URL needs a login exchange, which one canned
		// response cannot answer, so the check sees only failed Matrix sends.
		"matrix": {"Matrix logs in first, which the fixture's single response cannot answer", failSendFailed},
	},
	checkSendKeepsDeadline: {
		// MQTT keeps one connection across sends, so its dials use the connection's
		// lifetime context by design.
		"mqtt":  {"MQTT dials with the connection's lifetime context, which has no deadline", failDeadlineChanged},
		"mqtts": {"MQTT dials with the connection's lifetime context, which has no deadline", failDeadlineChanged},
		// Zulip's limits lookup is optional, so it is capped below the caller's
		// deadline and a slow lookup leaves the message request the rest.
		"zulip": {"Zulip caps its optional limits lookup at 10s by design", failDeadlineChanged},
	},
	checkUnknownParams: {
		// Generic sends every param that is not a config key as a field of the
		// webhook body, and the logger uses its params as template data.
		"generic": {"Generic sends params that are not config keys in the webhook body by design", failUnknownParamAccepted},
		"logger":  {"The logger uses its params as data for its message template by design", failUnknownParamAccepted},
	},
	checkCheckedConfig: {
		// These services check a config when they read its URL but not when they
		// send, so a config built in code reaches the network unchecked.
		"mqtt":  {uncheckedDial, failUncheckedConfig},
		"mqtts": {uncheckedDial, failUncheckedConfig},
		"smtp":  {uncheckedDial, failUncheckedConfig},
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

var (
	// standardKeyPattern matches the config keys that name a TLS option.
	standardKeyPattern = regexp.MustCompile(`tls|verify|insecure|scheme`)
	// standardKeys are the key names a TLS option may use. Deprecated keys are
	// listed apart from Keys by the format package, so they are not checked.
	standardKeys = []string{"skiptlsverify", "disabletls", "usestarttls", "starttls", "requirestarttls"}
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
		{checkNilPointerHTTPClient, runNilPointerHTTPClient},
		{checkClientFieldOrder, runClientFieldOrder},
		{checkNoSecretLeaks, runNoSecretLeaks},
		{checkNoSecretLeaksDefault, runNoSecretLeaksDefault},
		{checkSendUsesContext, runSendUsesContext},
		{checkStandardKeyNames, runStandardKeyNames},
		{checkKeyDescriptions, runKeyDescriptions},
		{checkURLParts, runURLParts},
		{checkDeprecatedFields, runDeprecatedFields},
		{checkSendBudget, runSendBudget},
		{checkSendKeepsDeadline, runSendKeepsDeadline},
		{checkPlaintextWarning, runPlaintextWarning},
		{checkLeveledLogs, runLeveledLogs},
		{checkConflictingKeys, runConflictingKeys},
		{checkUnknownParams, runUnknownParams},
		{checkCheckedConfig, runCheckedConfig},
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

// TestIntegrationPlaceholderURL verifies that services read a URL by its
// values and give no particular URL its own handling. Each scheme must give
// the placeholder URL <scheme>://dummy@dummy.com the same result as a URL of
// the same shape with another user and host, and when both are accepted, the
// same config URL with those values swapped.
func TestIntegrationPlaceholderURL(t *testing.T) {
	t.Parallel()

	// swap turns the placeholder's user and host into the other URL's.
	swap := strings.NewReplacer("dummy.com", "dummx.com", "dummy", "dumm1")

	for _, scheme := range router.SupportedSchemas() {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
			require.NoError(t, err)

			placeholder, placeholderErr := serviceRouter.Locate(scheme + "://dummy@dummy.com")
			closeService(t, placeholder)

			other, otherErr := serviceRouter.Locate(scheme + "://dumm1@dummx.com")
			closeService(t, other)

			require.Equal(t, otherErr == nil, placeholderErr == nil,
				"placeholder error %v, other URL error %v", placeholderErr, otherErr)

			if placeholderErr == nil {
				// Both URLs are parsed from their strings, so a config URL built
				// with an opaque part compares the same as its parsed form.
				want, err := url.Parse(format.GetServiceConfig(other).GetURL().String())
				require.NoError(t, err)

				got, err := url.Parse(swap.Replace(format.GetServiceConfig(placeholder).GetURL().String()))
				require.NoError(t, err)

				assertSameURL(t, want, got)
			}
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

// TestIntegrationEmptyFragment verifies that every scheme, and the custom
// URLs, reject a service URL that ends in a bare "#". A parsed URL cannot tell
// it from a URL without one, so the router rejects it before the service sees
// the URL.
func TestIntegrationEmptyFragment(t *testing.T) {
	t.Parallel()

	// rawURLs lists the service URLs to test by scheme: each fixture and the
	// custom URLs, all with a bare "#" appended.
	rawURLs := map[string]string{
		"generic custom":   "generic+https://hooks.example.invalid/webhook#",
		"notifiarr custom": "notifiarr+https://notifiarr.example.invalid/api#",
	}

	for _, fx := range fixtures {
		require.NotContains(t, fx.url, "#", "the fixture URL must have no fragment of its own")

		rawURLs[fx.scheme] = fx.url + "#"
	}

	for scheme, rawURL := range rawURLs {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			serviceRouter, err := router.NewWithOptions(log.New(io.Discard, "", 0), types.SenderOptions{})
			require.NoError(t, err)

			service, err := serviceRouter.Locate(rawURL)
			closeService(t, service)

			require.ErrorIs(t, err, router.ErrEmptyFragment)
		})
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

// TestIntegrationMaskedURL verifies that [format.MaskedURL], which the CLI uses
// to print URLs, holds none of a fixture's secrets while the unmasked config
// URL holds each of them, and that the masked URL parses with the scheme of
// the config URL.
func TestIntegrationMaskedURL(t *testing.T) {
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

			config := format.GetServiceConfig(service)
			masked := format.MaskedURL(config).String()
			plainURL := config.GetURL()
			plain := plainURL.String()

			for _, secret := range fx.secrets {
				assert.NotContains(t, masked, secret, "the masked URL holds a secret")
				assert.Contains(t, plain, url.QueryEscape(secret), "the config URL drops a secret")
			}

			parsed, err := url.Parse(masked)
			require.NoError(t, err)
			assert.Equal(t, plainURL.Scheme, parsed.Scheme)
		})
	}
}

// TestIntegrationItemTimeout verifies that a target that does not answer
// fails when its share of the send's time runs out, that the next target is
// still sent, and that the router reports that target's error as the service
// did, not as a timeout of the whole send.
func TestIntegrationItemTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var chats []string

		client := httpClientFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)

			if strings.Contains(string(body), `"chat_id":"hung"`) {
				chats = append(chats, "hung")

				<-req.Context().Done()

				return nil, req.Context().Err()
			}

			chats = append(chats, "next")

			return respond(fixtureFor(t, "telegram"), false)(req)
		})

		serviceRouter, err := router.NewWithOptions(nil, types.SenderOptions{HTTPClient: client},
			"telegram://12345:SECRETtoken@telegram/?chats=hung,next")
		require.NoError(t, err)

		errs := serviceRouter.Send("message", nil)

		require.Len(t, errs, 1)
		require.NotErrorIs(t, errs[0], router.ErrServiceTimeout)
		require.ErrorIs(t, errs[0], context.DeadlineExceeded)
		assert.Contains(t, errs[0].Error(), `sending to chat "hung"`)
		assert.Equal(t, []string{"hung", "next"}, chats)
	})
}

// TestIntegrationServiceLogger verifies that the leveled records of the
// router and its services carry the service's ID. The logger reaches the
// services through the router's constructor or through SetLogger. It is a
// plain logger or a slog logger. The generic service logs the plaintext
// warning for an http custom URL, and the response status at Debug.
func TestIntegrationServiceLogger(t *testing.T) {
	t.Parallel()

	const customURL = "generic+http://example.invalid/hook"

	ok := httpClientFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{Status: "200 OK", StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{}}, nil
	})
	opts := types.SenderOptions{HTTPClient: ok}

	t.Run("plain logger", func(t *testing.T) {
		t.Parallel()

		logs := &syncBuffer{}

		serviceRouter, err := router.NewWithOptions(log.New(logs, "", 0), opts, customURL)
		require.NoError(t, err)

		for _, err := range serviceRouter.Send("message", nil) {
			require.NoError(t, err)
		}

		assert.Equal(t,
			"DEBUG got custom URL service=generic\n"+
				"DEBUG converted custom URL service=generic\n"+
				"WARN sending without TLS service=generic\n"+
				`DEBUG response received service=generic status="200 OK"`+"\n",
			logs.String())
	})

	t.Run("slog logger", func(t *testing.T) {
		t.Parallel()

		logs := &syncBuffer{}
		logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelDebug,
			ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
				if len(groups) == 0 && attr.Key == slog.TimeKey {
					return slog.Attr{}
				}

				return attr
			},
		}))

		serviceRouter, err := router.NewWithOptions(types.NewSlogLogger(logger), opts, customURL)
		require.NoError(t, err)

		for _, err := range serviceRouter.Send("message", nil) {
			require.NoError(t, err)
		}

		assert.Equal(t,
			`level=DEBUG msg="got custom URL" service=generic`+"\n"+
				`level=DEBUG msg="converted custom URL" service=generic`+"\n"+
				`level=WARN msg="sending without TLS" service=generic`+"\n"+
				`level=DEBUG msg="response received" service=generic status="200 OK"`+"\n",
			logs.String())
	})

	t.Run("warning source", func(t *testing.T) {
		t.Parallel()

		logs := &syncBuffer{}
		logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{AddSource: true, Level: slog.LevelWarn}))

		serviceRouter, err := router.NewWithOptions(types.NewSlogLogger(logger), opts, customURL)
		require.NoError(t, err)

		for _, err := range serviceRouter.Send("message", nil) {
			require.NoError(t, err)
		}

		// The source is the service's call to the shared warning, not the
		// library's logging code.
		assert.Regexp(t, `level=WARN source=\S+/specialized/generic/generic\.go:\d+ msg="sending without TLS"`, logs.String())
	})

	t.Run("set logger", func(t *testing.T) {
		t.Parallel()

		logs := &syncBuffer{}

		serviceRouter, err := router.NewWithOptions(nil, opts, customURL)
		require.NoError(t, err)

		serviceRouter.SetLogger(log.New(logs, "", 0))

		for _, err := range serviceRouter.Send("message", nil) {
			require.NoError(t, err)
		}

		assert.Equal(t,
			"WARN sending without TLS service=generic\n"+`DEBUG response received service=generic status="200 OK"`+"\n",
			logs.String())
	})
}

// TestIntegrationSkipTLSVerify verifies how each HTTP service that lets users
// skip TLS certificate verification treats a server whose certificate cannot
// be verified. [skipVerifyFixtures] picks the services by their skiptlsverify
// key, so a service that adds the key is checked without an edit here.
//
//   - Without skiptlsverify, no request reaches the server.
//   - With skiptlsverify in the URL or in the send params, requests reach it.
//   - The shared warning is logged once for the insecure default client, and
//     never when the caller injected a client.
//
// Known failures are listed in [allowlist], as in [TestIntegrationContract].
func TestIntegrationSkipTLSVerify(t *testing.T) {
	t.Parallel()

	var warning string

	(&logging.Once{}).Insecure(t.Context(), logFunc(func(msg string) { warning = msg }))
	require.NotEmpty(t, warning)

	for _, fx := range skipVerifyFixtures(t) {
		t.Run(fx.scheme, func(t *testing.T) {
			t.Parallel()

			var hits atomic.Int32

			answer := respond(fx, false)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				hits.Add(1)

				resp, _ := answer(req)
				defer resp.Body.Close()

				w.WriteHeader(resp.StatusCode)
				_, _ = io.Copy(w, resp.Body)
			}))
			t.Cleanup(server.Close)

			tlsURLs := skipVerifyURLs{
				base: withURLHost(t, fx.url, server.Listener.Addr().String()),
			}

			// A service that does not send to its URL host names its server in
			// a query key instead.
			if fx.serverKey != "" {
				tlsURLs.base = withURLQuery(t, fx.url, fx.serverKey, server.URL)
			}

			tlsURLs.skip = withURLQuery(t, tlsURLs.base, "skiptlsverify", "yes")

			checks := []struct {
				name string
				run  func(t *testing.T, fx *fixture, urls skipVerifyURLs, hits *atomic.Int32, warning string) failure
			}{
				{checkSkipTLSVerifyURL, runSkipTLSVerifyURL},
				{checkSkipTLSVerifyParams, runSkipTLSVerifyParams},
				{checkInsecureWarning, runInsecureWarning},
			}

			for _, check := range checks {
				t.Run(check.name, func(t *testing.T) {
					hits.Store(0)
					assertContract(t, check.name, fx.scheme, check.run(t, fx, tlsURLs, &hits, warning))
				})
			}
		})
	}
}

// skipVerifyFixtures returns the first contract fixture of each scheme whose
// config has a skiptlsverify key and whose service sends over HTTP. XMPP,
// SMTP, and MQTT implement types.DialContextSetter because they dial their own
// TLS connections. Their packages test skipping verification.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - []*fixture: the fixtures, in the order of [fixtures].
func skipVerifyFixtures(t *testing.T) []*fixture {
	t.Helper()

	var selected []*fixture

	seen := map[string]bool{}

	for i := range fixtures {
		fx := &fixtures[i]
		if seen[fx.scheme] {
			continue
		}

		seen[fx.scheme] = true

		service, err := locateDefault(t, fx.url, log.New(io.Discard, "", 0))
		require.NoError(t, err, fx.scheme)

		if _, dials := service.(types.DialContextSetter); dials {
			continue
		}

		if hasKey(format.GetServiceConfig(service), "skiptlsverify") {
			selected = append(selected, fx)
		}
	}

	return selected
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

// fixtureFor returns the first contract fixture for scheme.
//
// Parameters:
//   - t: the test.
//   - scheme: the service scheme.
//
// Returns:
//   - *fixture: the fixture.
func fixtureFor(t *testing.T, scheme string) *fixture {
	t.Helper()

	for i := range fixtures {
		if fixtures[i].scheme == scheme {
			return &fixtures[i]
		}
	}

	t.Fatalf("no fixture for scheme %q", scheme)

	return nil
}

// withURLQuery returns rawURL with the query parameter key set to value.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - key: the query parameter.
//   - value: its value.
//
// Returns:
//   - string: the service URL with the parameter set.
func withURLQuery(t *testing.T, rawURL, key, value string) string {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	query := serviceURL.Query()
	query.Set(key, value)
	serviceURL.RawQuery = query.Encode()

	return serviceURL.String()
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

// runUnknownParams verifies that a send with a param that is neither a config
// key nor reserved fails as an unknown config key, so a misspelled param is
// reported instead of ignored.
func runUnknownParams(t *testing.T, fx *fixture) failure {
	t.Helper()

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)

		service, err := env.locate(t)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		params := types.Params{"contract_unknown": "value"}

		if result = env.send(service, &params); result.kind != failNone {
			return
		}

		if !errors.Is(env.sendErr, format.ErrInvalidConfigKey) &&
			(env.sendErr == nil || !strings.Contains(env.sendErr.Error(), format.ErrInvalidConfigKey.Error())) {
			result = fail(failUnknownParamAccepted, "an unknown param did not fail the send: %v", env.sendErr)
		}
	})

	return result
}

// runCheckedConfig verifies that a service checks the config of each send as
// reading a service URL checks it. The service's Config is replaced with the
// zero value of its type, as a caller building one in code would leave it.
// When reading the zero config's own URL fails, the send must fail too. It must
// fail without a panic and before any request or dial. The check skips a
// service without an exported Config field or with a zero config that reads back.
func runCheckedConfig(t *testing.T, fx *fixture) failure {
	t.Helper()

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)

		service, err := env.locate(t)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		field, ok := configField(service)
		if !ok {
			return
		}

		zero, ok := reflect.TypeAssert[types.ServiceConfig](reflect.New(field.Type().Elem()))
		if !ok {
			return
		}

		var readBack error

		if panicMsg := recovering(func() { readBack = zero.SetURL(zero.GetURL()) }); panicMsg != "" {
			result = fail(failPanic, "reading back a zero config %s", panicMsg)

			return
		}

		if readBack == nil {
			return
		}

		field.Set(reflect.New(field.Type().Elem()))

		defaultBefore, lookupBefore := defaultTransportHits.Load(), lookupHits.Load()

		if result = env.send(service, nil); result.kind != failNone {
			return
		}

		// A request through http.DefaultTransport or another uninjected transport
		// counts as a request too.
		requests := int64(len(env.client.Calls)) +
			defaultTransportHits.Load() - defaultBefore +
			lookupHits.Load() - lookupBefore
		dials := env.dials.Load()

		if env.sendErr == nil || requests > 0 || dials > 0 {
			result = fail(failUncheckedConfig, "a send with a zero config returned %v after %d request(s) and %d dial(s)",
				env.sendErr, requests, dials)
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
	field, ok := configField(service)
	if !ok || field.IsNil() {
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

// runNilPointerHTTPClient verifies that an injected client that is a nil
// pointer of a type other than *http.Client fails the send with
// types.ErrNilHTTPClient. It is injected through the router's options and
// through SetHTTPClient. The send must not panic, and must not fall back to
// the default client, which would bypass the client the caller meant to inject.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failNilClientUnreported, failDefaultTransport, failPanic, or
//     failInit failure, or the zero value.
func runNilPointerHTTPClient(t *testing.T, fx *fixture) failure {
	t.Helper()

	if fx.kind != netHTTP {
		return failure{}
	}

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)
		nilClient := (*mockTypes.MockHTTPClient)(nil)

		// paths lists the ways a caller injects the nil client.
		paths := []struct {
			locate func() (types.Service, error)
			name   string
		}{
			{name: "router options", locate: func() (types.Service, error) {
				return env.locateWith(t, types.SenderOptions{HTTPClient: nilClient, DialContext: env.dial})
			}},
			{name: "SetHTTPClient", locate: func() (types.Service, error) {
				service, err := env.locate(t)
				if setter, ok := service.(types.HTTPClientSetter); ok && err == nil {
					setter.SetHTTPClient(nilClient)
				}

				return service, err
			}},
		}

		for _, path := range paths {
			service, err := path.locate()
			if err != nil {
				result = fail(failInit, "initialization failed: %v", err)

				return
			}

			defaultBefore, lookupBefore := defaultTransportHits.Load(), lookupHits.Load()

			if result = env.send(service, nil); result.kind != failNone {
				return
			}

			switch {
			case defaultTransportHits.Load() > defaultBefore || lookupHits.Load() > lookupBefore:
				result = fail(failDefaultTransport, "%s: Send fell back to the default client", path.name)
			case !errors.Is(env.sendErr, types.ErrNilHTTPClient):
				result = fail(failNilClientUnreported, "%s: Send returned %v", path.name, env.sendErr)
			default:
			}

			if result.kind != failNone {
				return
			}
		}
	})

	return result
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

// runClientFieldOrder verifies that a service's exported client field and
// SetHTTPClient follow last-write-wins. A field set before SetHTTPClient loses
// to it, and a field set after it wins. A field set back to nil restores the
// client from SetHTTPClient. A service without an exported client field passes.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failClientFieldOrder, failPanic, or failInit failure, or the
//     zero value.
func runClientFieldOrder(t *testing.T, fx *fixture) failure {
	t.Helper()

	if fx.kind != netHTTP {
		return failure{}
	}

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)

		service, err := env.locate(t)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		setter, isSetter := service.(types.HTTPClientSetter)

		field, hasField := clientField(service)
		if !isSetter || !hasField {
			return
		}

		var fieldCalls atomic.Int32

		answer := respond(fx, false)
		record := func(req *http.Request) (*http.Response, error) {
			fieldCalls.Add(1)

			return answer(req)
		}

		fieldClient := reflect.ValueOf(httpClientFunc(record))
		if field.Type() == reflect.TypeFor[*http.Client]() {
			fieldClient = reflect.ValueOf(&http.Client{Transport: roundTripperFunc(record)})
		}

		// sendThrough sends and reports whether only the expected client carried
		// the requests.
		sendThrough := func(step string, wantField bool) bool {
			fieldBefore, injectedBefore := fieldCalls.Load(), env.httpCalls()

			if result = env.send(service, nil); result.kind != failNone {
				return false
			}

			fieldUsed := fieldCalls.Load() > fieldBefore
			injectedUsed := env.httpCalls() > injectedBefore

			switch {
			case wantField && (!fieldUsed || injectedUsed):
				result = fail(failClientFieldOrder, "%s: Send did not use only the client field", step)
			case !wantField && (fieldUsed || !injectedUsed):
				result = fail(failClientFieldOrder, "%s: Send did not use only the client from SetHTTPClient", step)
			}

			return result.kind == failNone
		}

		field.Set(fieldClient)
		setter.SetHTTPClient(env.client)

		if !sendThrough("field set before SetHTTPClient", false) {
			return
		}

		field.Set(fieldClient)

		if !sendThrough("field set after SetHTTPClient", true) {
			return
		}

		field.SetZero()
		sendThrough("field set to nil", false)
	})

	return result
}

// clientField returns a service's exported HTTP client field, which callers
// can set in place of calling SetHTTPClient. The field holds an *http.Client
// or an interface that [types.HTTPClient] satisfies.
//
// Parameters:
//   - service: the located service.
//
// Returns:
//   - reflect.Value: the settable field.
//   - bool: whether the service has such a field.
func clientField(service types.Service) (reflect.Value, bool) {
	value := reflect.ValueOf(service)
	if value.Kind() != reflect.Pointer || value.Elem().Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	clientType := reflect.TypeFor[types.HTTPClient]()

	for field, fieldValue := range value.Elem().Fields() {
		if !field.IsExported() || field.Anonymous {
			continue
		}

		isInterface := field.Type.Kind() == reflect.Interface && field.Type.NumMethod() > 0

		if field.Type == reflect.TypeFor[*http.Client]() || isInterface && clientType.Implements(field.Type) {
			return fieldValue, true
		}
	}

	return reflect.Value{}, false
}

// configField returns a service's exported Config field, which callers can
// read or replace in place of reading a service URL.
//
// Parameters:
//   - service: the located service.
//
// Returns:
//   - reflect.Value: the settable field, which may hold nil.
//   - bool: whether the service has a Config pointer field.
func configField(service types.Service) (reflect.Value, bool) {
	value := reflect.ValueOf(service)
	if value.Kind() != reflect.Pointer || value.Elem().Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	field := value.Elem().FieldByName("Config")
	if !field.IsValid() || field.Kind() != reflect.Pointer {
		return reflect.Value{}, false
	}

	return field, true
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

// runStandardKeyNames verifies that every config key naming a TLS option is
// one of [standardKeys]. A key whose description starts with "Deprecated:",
// as the docs show it, is kept only for compatibility and is not checked.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failNonStandardKey or failInit failure, or the zero value.
func runStandardKeyNames(t *testing.T, fx *fixture) failure {
	t.Helper()

	config, result := fixtureConfig(t, fx.url)
	if config == nil {
		return result
	}

	var keys []string

	for _, item := range format.GetConfigFormat(config).Items {
		if strings.HasPrefix(item.Field().Description, "Deprecated:") {
			continue
		}

		for _, key := range item.Field().Keys {
			if standardKeyPattern.MatchString(key) && !slices.Contains(standardKeys, key) {
				keys = append(keys, key)
			}
		}
	}

	if len(keys) > 0 {
		return fail(failNonStandardKey, "TLS option keys with nonstandard names: %s", strings.Join(keys, ", "))
	}

	return failure{}
}

// runKeyDescriptions verifies that every keyed config field has a description,
// which the docs and the CLI show.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failMissingDescription or failInit failure, or the zero value.
func runKeyDescriptions(t *testing.T, fx *fixture) failure {
	t.Helper()

	config, result := fixtureConfig(t, fx.url)
	if config == nil {
		return result
	}

	var keys []string

	for _, item := range format.GetConfigFormat(config).Items {
		if field := item.Field(); len(field.Keys) > 0 && field.Description == "" {
			keys = append(keys, field.Keys[0])
		}
	}

	if len(keys) > 0 {
		return fail(failMissingDescription, "keys without a description: %s", strings.Join(keys, ", "))
	}

	return failure{}
}

// runURLParts verifies that each name in a config field's url tag is a part of
// a URL. The docs, the CLI, and the Playground place the field by its tag. A
// misspelled name parses as the query, like the name query itself, so they
// would show the field as a query prop. The tag names are read before parsing
// to tell the two apart.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failInvalidURLPart or failInit failure, or the zero value.
func runURLParts(t *testing.T, fx *fixture) failure {
	t.Helper()

	config, result := fixtureConfig(t, fx.url)
	if config == nil {
		return result
	}

	var fields []string

	configType := reflect.Indirect(reflect.ValueOf(config)).Type()
	for field := range configType.Fields() {
		tag, ok := field.Tag.Lookup("url")
		if !ok {
			continue
		}

		for name := range strings.SplitSeq(tag, ",") {
			if format.ParseURLPart(name) == format.URLQuery && !strings.EqualFold(name, "query") {
				fields = append(fields, field.Name)

				break
			}
		}
	}

	if len(fields) > 0 {
		return fail(failInvalidURLPart, "fields whose url tag names no part of a URL: %s", strings.Join(fields, ", "))
	}

	return failure{}
}

// runDeprecatedFields verifies that the config URL holds no deprecated key, and
// that each deprecated config field has a valid tag. A value set only through a
// deprecated field must reach the config URL and come back in its replacement.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failDeprecatedField or failInit failure, or the zero value.
func runDeprecatedFields(t *testing.T, fx *fixture) failure {
	t.Helper()

	config, result := fixtureConfig(t, fx.url)
	if config == nil {
		return result
	}

	deprecatedKeys := deprecatedKeysOf(config)
	if key := deprecatedKeyIn(config.GetURL(), deprecatedKeys); key != "" {
		return fail(failDeprecatedField, "the config URL holds the deprecated key %q", key)
	}

	configType := reflect.TypeOf(config)
	if configType.Kind() != reflect.Pointer || configType.Elem().Kind() != reflect.Struct {
		return failure{}
	}

	if _, err := format.SyncDeprecatedFields(cloneConfig(t, config)); err != nil {
		return fail(failDeprecatedField, "%v", err)
	}

	for field := range configType.Elem().Fields() {
		if replacement, tagged := field.Tag.Lookup("deprecated"); tagged {
			if result := checkDeprecatedField(t, config, field.Name, replacement, deprecatedKeys); result.kind != failNone {
				return result
			}
		}
	}

	return failure{}
}

// checkDeprecatedField sets only a deprecated field on a copy of config and
// verifies that its value reaches the config URL and comes back in the
// replacement field.
//
// Parameters:
//   - t: the test for this case.
//   - config: the fixture's config.
//   - name: the deprecated field.
//   - replacement: the field that replaces it.
//   - deprecatedKeys: the config's deprecated keys, which the URL must not hold.
//
// Returns:
//   - failure: a failDeprecatedField failure, or the zero value.
func checkDeprecatedField(
	t *testing.T,
	config types.ServiceConfig,
	name, replacement string,
	deprecatedKeys []string,
) failure {
	t.Helper()

	clone := cloneConfig(t, config)
	fields := reflect.ValueOf(clone).Elem()

	sample, ok := sampleValue(fields.FieldByName(name).Type())
	if !ok {
		return fail(failDeprecatedField, "the check cannot set %s of type %s", name, fields.FieldByName(name).Type())
	}

	fields.FieldByName(replacement).SetZero()
	fields.FieldByName(name).Set(sample)

	configURL := clone.GetURL()
	if key := deprecatedKeyIn(configURL, deprecatedKeys); key != "" {
		return fail(failDeprecatedField, "the config URL holds the deprecated key %q", key)
	}

	restored, result := fixtureConfig(t, configURL.String())
	if restored == nil {
		return fail(failDeprecatedField, "the config URL with %s set does not parse: %s", name, result.detail)
	}

	if got := reflect.ValueOf(restored).Elem().FieldByName(replacement); !got.Equal(sample) {
		return fail(failDeprecatedField, "%s set to %v came back in %s as %v", name, sample, replacement, got)
	}

	return failure{}
}

// runSendBudget verifies that a direct Send without a deadline bounds every
// request and dial by the service's send budget. The budget is what the
// service reports through [types.ServiceTimeout], or
// [types.DefaultSendTimeout].
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failBudget, failNotInjected, failPanic, or failInit failure, or
//     the zero value.
func runSendBudget(t *testing.T, fx *fixture) failure {
	t.Helper()

	if fx.kind == netNone {
		return failure{}
	}

	var result failure

	synctest.Test(t, func(t *testing.T) {
		var (
			mu        sync.Mutex
			deadlines []time.Time
			missing   int
		)

		record := func(ctx context.Context) {
			mu.Lock()
			defer mu.Unlock()

			if deadline, ok := ctx.Deadline(); ok {
				deadlines = append(deadlines, deadline)
			} else {
				missing++
			}
		}

		answer := respond(fx, false)
		client := httpClientFunc(func(req *http.Request) (*http.Response, error) {
			record(req.Context())

			return answer(req)
		})
		dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
			record(ctx)

			return nil, errDial
		}

		serviceRouter, err := router.NewWithOptions(
			log.New(io.Discard, "", 0),
			types.SenderOptions{HTTPClient: client, DialContext: dial},
		)
		require.NoError(t, err)

		service, err := serviceRouter.Locate(fx.url)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		t.Cleanup(func() { closeService(t, service) })

		budget := types.DefaultSendTimeout
		if reporter, ok := service.(types.ServiceTimeout); ok && reporter.ServiceTimeout(nil) > 0 {
			budget = reporter.ServiceTimeout(nil)
		}

		start := time.Now()

		if outcome := sendRecovering(service, nil); outcome.panicMsg != "" {
			result = fail(failPanic, "%s", outcome.panicMsg)

			return
		}

		mu.Lock()
		defer mu.Unlock()

		switch {
		case missing+len(deadlines) == 0:
			result = fail(failNotInjected, "Send made no request or dial")
		case missing > 0:
			result = fail(failBudget, "%d of %d request(s) and dial(s) had no deadline", missing, missing+len(deadlines))
		default:
			for _, deadline := range deadlines {
				if !deadline.Equal(start.Add(budget)) {
					result = fail(failBudget, "a request or dial had a %v deadline, not the %v budget", deadline.Sub(start), budget)

					return
				}
			}
		}
	})

	return result
}

// runSendKeepsDeadline verifies that a send through SendContext with a
// caller's deadline gives every request and dial that deadline unchanged, so
// no request within the send adds a timeout of its own. A service without
// SendContext passes.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failDeadlineChanged, failNotInjected, failPanic, or failInit
//     failure, or the zero value.
func runSendKeepsDeadline(t *testing.T, fx *fixture) failure {
	t.Helper()

	if fx.kind == netNone {
		return failure{}
	}

	var result failure

	synctest.Test(t, func(t *testing.T) {
		var (
			mu        sync.Mutex
			deadlines []time.Time
			missing   int
		)

		record := func(ctx context.Context) {
			mu.Lock()
			defer mu.Unlock()

			if deadline, ok := ctx.Deadline(); ok {
				deadlines = append(deadlines, deadline)
			} else {
				missing++
			}
		}

		answer := respond(fx, false)
		client := httpClientFunc(func(req *http.Request) (*http.Response, error) {
			record(req.Context())

			return answer(req)
		})
		dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
			record(ctx)

			return nil, errDial
		}

		serviceRouter, err := router.NewWithOptions(
			log.New(io.Discard, "", 0),
			types.SenderOptions{HTTPClient: client, DialContext: dial},
		)
		require.NoError(t, err)

		service, err := serviceRouter.Locate(fx.url)
		if err != nil {
			result = fail(failInit, "initialization failed: %v", err)

			return
		}

		t.Cleanup(func() { closeService(t, service) })

		sender, ok := service.(types.ContextSender)
		if !ok {
			return
		}

		ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
		defer cancel()

		want, _ := ctx.Deadline()

		if panicMsg := recovering(func() { _ = sender.SendContext(ctx, "contract message", nil) }); panicMsg != "" {
			result = fail(failPanic, "%s", panicMsg)

			return
		}

		mu.Lock()
		defer mu.Unlock()

		switch {
		case missing+len(deadlines) == 0:
			result = fail(failNotInjected, "SendContext made no request or dial")
		case missing > 0:
			result = fail(failDeadlineChanged, "%d of %d request(s) and dial(s) had no deadline", missing, missing+len(deadlines))
		default:
			for _, deadline := range deadlines {
				if !deadline.Equal(want) {
					result = fail(failDeadlineChanged, "a request or dial had a deadline %v before the caller's", want.Sub(deadline))

					return
				}
			}
		}
	})

	return result
}

// runConflictingKeys verifies that a URL giving one config key in two
// spellings fails to initialize with format.ErrConflictingKeys. The spellings
// are the key and its upper case. The key is the first one with a non-empty
// value, so the value is valid for both spellings. A service without such a
// key passes.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failConflictAccepted or failInit failure, or the zero value.
func runConflictingKeys(t *testing.T, fx *fixture) failure {
	t.Helper()

	config, result := fixtureConfig(t, fx.url)
	if config == nil {
		return result
	}

	resolver := format.GetConfigQueryResolver(config)

	for _, key := range resolver.QueryFields() {
		value, err := resolver.Get(key)
		if err != nil || value == "" {
			continue
		}

		rawURL := withURLQuery(t, withURLQuery(t, fx.url, key, value), strings.ToUpper(key), value)

		if _, err := locateDefault(t, rawURL, log.New(io.Discard, "", 0)); !errors.Is(err, format.ErrConflictingKeys) {
			return fail(failConflictAccepted, "a URL with %q and %q was not rejected as conflicting: %v", key, strings.ToUpper(key), err)
		}

		return failure{}
	}

	return failure{}
}

// runPlaintextWarning verifies that a service with a disabletls key logs the
// shared plaintext warning once over two sends with TLS disabled. The sends go
// through the injected client and dialer, because the warning applies to them
// too.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failWarningCount, failPanic, or failInit failure, or the zero
//     value. A URL with disabletls that the service rejects passes.
func runPlaintextWarning(t *testing.T, fx *fixture) failure {
	t.Helper()

	config, result := fixtureConfig(t, fx.url)
	if config == nil || !hasKey(config, "disabletls") {
		return result
	}

	var warning string

	(&logging.Once{}).Plaintext(t.Context(), logFunc(func(msg string) { warning = msg }))

	synctest.Test(t, func(t *testing.T) {
		logs := &syncBuffer{}
		dial := func(context.Context, string, string) (net.Conn, error) { return nil, errDial }

		serviceRouter, err := router.NewWithOptions(
			log.New(logs, "", 0),
			types.SenderOptions{HTTPClient: httpClientFunc(respond(fx, false)), DialContext: dial},
		)
		require.NoError(t, err)

		// A scheme that always uses TLS, such as xmpps, rejects disabletls, so
		// it never sends without TLS.
		service, err := serviceRouter.Locate(withURLQuery(t, fx.url, "disabletls", "yes"))
		if err != nil {
			return
		}

		t.Cleanup(func() { closeService(t, service) })

		for range 2 {
			if outcome := sendRecovering(service, nil); outcome.panicMsg != "" {
				result = fail(failPanic, "%s", outcome.panicMsg)

				return
			}
		}

		if count := strings.Count(logs.String(), warning); count != 1 {
			result = fail(failWarningCount, "the plaintext warning was logged %d times over two sends", count)
		}
	})

	return result
}

// runLeveledLogs verifies that a service logs only leveled records, over sends
// that succeed and sends whose transport fails. A plain logger receives each
// record as a line that starts with its level, so a line without one was
// printed as text. The logger service's notification is the only text a
// service prints.
//
// With a working transport, the sends of HTTP services and of services without
// network access must succeed, so their success paths are checked. The dialer
// fails every TCP connection, so TCP services are checked on their failure
// paths only. Their package tests cover sends to a server.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//
// Returns:
//   - failure: a failInit, failPanic, failUnleveledLog, or failSendFailed
//     failure, or the zero value.
func runLeveledLogs(t *testing.T, fx *fixture) failure {
	t.Helper()

	var result failure

	for _, failTransport := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			env := newEnv(t, fx, failTransport)

			service, err := env.locate(t)
			if err != nil {
				result = fail(failInit, "initialization failed: %v", err)

				return
			}

			// A send that fails with a working transport is reported after the
			// log lines are checked, so it cannot hide an unleveled line.
			var sendErr error

			for range 2 {
				if result = env.send(service, nil); result.kind != failNone {
					return
				}

				if !failTransport && fx.kind != netTCP && env.sendErr != nil {
					sendErr = env.sendErr

					break
				}
			}

			for line := range strings.Lines(env.logs.String()) {
				line = strings.TrimSuffix(line, "\n")
				if fx.scheme == "logger" && line == "contract message" {
					continue
				}

				leveled := strings.HasPrefix(line, "DEBUG ") ||
					strings.HasPrefix(line, "INFO ") ||
					strings.HasPrefix(line, "WARN ")
				if !leveled || strings.Contains(line, "Warning:") {
					result = fail(failUnleveledLog, "logged %q", line)

					return
				}
			}

			if sendErr != nil {
				result = fail(failSendFailed, "send failed with a working transport: %v", sendErr)
			}
		})

		if result.kind != failNone {
			break
		}
	}

	return result
}

// runSkipTLSVerifyURL verifies that a service sends nothing to a server whose
// certificate cannot be verified, until its URL sets skiptlsverify.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//   - urls: the service URLs for the server.
//   - hits: counts the requests that reached the server.
//
// Returns:
//   - failure: a failUnverifiedAccepted or failSkipIgnored failure, or the zero
//     value.
func runSkipTLSVerifyURL(t *testing.T, _ *fixture, urls skipVerifyURLs, hits *atomic.Int32, _ string) failure {
	t.Helper()

	service, err := locateDefault(t, urls.base, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	_ = sendRecovering(service, nil)

	if hits.Load() > 0 {
		return fail(failUnverifiedAccepted, "a request reached the server without skiptlsverify")
	}

	service, err = locateDefault(t, urls.skip, log.New(io.Discard, "", 0))
	if err != nil {
		return fail(failSkipIgnored, "the URL with skiptlsverify was rejected: %v", err)
	}

	_ = sendRecovering(service, nil)

	if hits.Load() == 0 {
		return fail(failSkipIgnored, "no request reached the server with skiptlsverify in the URL")
	}

	return failure{}
}

// runSkipTLSVerifyParams verifies that skiptlsverify in the send params lets a
// request reach a server whose certificate cannot be verified.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//   - urls: the service URLs for the server.
//   - hits: counts the requests that reached the server.
//
// Returns:
//   - failure: a failSkipIgnored failure, or the zero value.
func runSkipTLSVerifyParams(t *testing.T, _ *fixture, urls skipVerifyURLs, hits *atomic.Int32, _ string) failure {
	t.Helper()

	service, err := locateDefault(t, urls.base, log.New(io.Discard, "", 0))
	require.NoError(t, err)

	outcome := sendRecovering(service, &types.Params{"skiptlsverify": "yes"})

	if hits.Load() == 0 {
		return fail(
			failSkipIgnored,
			"no request reached the server with skiptlsverify in the params: %v%s",
			outcome.err, outcome.panicMsg,
		)
	}

	return failure{}
}

// runInsecureWarning verifies that the shared warning is logged once over two
// sends through the insecure default client, and not at all when the caller
// injected a client.
//
// Parameters:
//   - t: the test for this case.
//   - fx: the fixture under test.
//   - urls: the service URLs for the server.
//   - warning: the shared warning text.
//
// Returns:
//   - failure: a failWarningCount, failWarnedInjected, or failInit failure, or
//     the zero value.
func runInsecureWarning(t *testing.T, fx *fixture, urls skipVerifyURLs, _ *atomic.Int32, warning string) failure {
	t.Helper()

	logs := &syncBuffer{}

	service, err := locateDefault(t, urls.skip, log.New(logs, "", 0))
	if err != nil {
		return fail(failInit, "the URL with skiptlsverify was rejected: %v", err)
	}

	_ = sendRecovering(service, nil)
	_ = sendRecovering(service, nil)

	if count := strings.Count(logs.String(), warning); count != 1 {
		return fail(failWarningCount, "the warning was logged %d times over two sends", count)
	}

	injectedLogs := &syncBuffer{}

	serviceRouter, err := router.NewWithOptions(
		log.New(injectedLogs, "", 0),
		types.SenderOptions{HTTPClient: httpClientFunc(respond(fx, false))},
	)
	require.NoError(t, err)

	service, err = serviceRouter.Locate(urls.skip)
	require.NoError(t, err)

	_ = sendRecovering(service, nil)

	if strings.Contains(injectedLogs.String(), warning) {
		return fail(failWarnedInjected, "the warning was logged although the caller injected a client")
	}

	return failure{}
}

// fixtureConfig builds the service for rawURL through a router without
// injected clients and returns its config.
//
// Parameters:
//   - t: the test for this case.
//   - rawURL: the service URL.
//
// Returns:
//   - types.ServiceConfig: the service's config, or nil when it has none or
//     failed to initialize.
//   - failure: a failInit failure when the service failed to initialize.
func fixtureConfig(t *testing.T, rawURL string) (types.ServiceConfig, failure) {
	t.Helper()

	service, err := locateDefault(t, rawURL, log.New(io.Discard, "", 0))
	if err != nil {
		return nil, fail(failInit, "initialization failed: %v", err)
	}

	return format.GetServiceConfig(service), failure{}
}

// locateDefault builds the service for rawURL through a router without
// injected clients, so it uses its own default client. The service is closed
// when the test ends.
//
// Parameters:
//   - t: the test for this case.
//   - rawURL: the service URL.
//   - logger: receives the service's log output.
//
// Returns:
//   - types.Service: the service.
//   - error: the initialization error.
func locateDefault(t *testing.T, rawURL string, logger *log.Logger) (types.Service, error) {
	t.Helper()

	serviceRouter, err := router.NewWithOptions(logger, types.SenderOptions{})
	require.NoError(t, err)

	service, err := serviceRouter.Locate(rawURL)
	if err != nil {
		return nil, fmt.Errorf("locating service: %w", err)
	}

	t.Cleanup(func() { closeService(t, service) })

	return service, nil
}

// cloneConfig returns a copy of a config that points to a struct.
//
// Parameters:
//   - t: the test for this case.
//   - config: the config to copy.
//
// Returns:
//   - types.ServiceConfig: a pointer to a new copy of the struct.
func cloneConfig(t *testing.T, config types.ServiceConfig) types.ServiceConfig {
	t.Helper()

	value := reflect.ValueOf(config).Elem()
	clone := reflect.New(value.Type())
	clone.Elem().Set(value)

	cloned, ok := reflect.TypeAssert[types.ServiceConfig](clone)
	require.True(t, ok, "%T is not a service config", clone.Interface())

	return cloned
}

// sampleValue returns a value other than the zero value for a field type.
//
// Parameters:
//   - fieldType: the type of the field.
//
// Returns:
//   - reflect.Value: the sample, converted to fieldType.
//   - bool: false when the check has no sample for the type's kind.
func sampleValue(fieldType reflect.Type) (reflect.Value, bool) {
	switch fieldType.Kind() {
	case reflect.Bool:
		return reflect.ValueOf(true).Convert(fieldType), true
	case reflect.String:
		return reflect.ValueOf("contract").Convert(fieldType), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflect.ValueOf(1).Convert(fieldType), true
	default:
		return reflect.Value{}, false
	}
}

// hasKey reports whether a config has a field with key among its keys.
//
// Parameters:
//   - config: the service config.
//   - key: the key in lower case.
//
// Returns:
//   - bool: whether a field has the key.
func hasKey(config types.ServiceConfig, key string) bool {
	for _, item := range format.GetConfigFormat(config).Items {
		if slices.Contains(item.Field().Keys, key) {
			return true
		}
	}

	return false
}

// deprecatedKeysOf lists the deprecated keys of every field of a config.
//
// Parameters:
//   - config: the service config.
//
// Returns:
//   - []string: the deprecated keys in lower case.
func deprecatedKeysOf(config types.ServiceConfig) []string {
	var keys []string

	for _, item := range format.GetConfigFormat(config).Items {
		keys = append(keys, item.Field().DeprecatedKeys...)
	}

	return keys
}

// deprecatedKeyIn returns a deprecated key that a config URL's query holds.
//
// Parameters:
//   - configURL: the config URL.
//   - deprecatedKeys: the deprecated keys in lower case.
//
// Returns:
//   - string: a deprecated key in the query, or "" when there is none.
func deprecatedKeyIn(configURL *url.URL, deprecatedKeys []string) string {
	for key := range configURL.Query() {
		if slices.Contains(deprecatedKeys, strings.ToLower(key)) {
			return key
		}
	}

	return ""
}

// sendRecovering sends a message and reports a panic instead of crashing.
//
// Parameters:
//   - service: the service to send through.
//   - params: the send params, or nil.
//
// Returns:
//   - sendOutcome: the send's error, or its panic message.
func sendRecovering(service types.Service, params *types.Params) sendOutcome {
	var outcome sendOutcome

	outcome.panicMsg = recovering(func() { outcome.err = service.Send("contract message", params) })

	return outcome
}

// recovering calls fn and reports a panic instead of crashing.
//
// Parameters:
//   - fn: the call to make.
//
// Returns:
//   - string: a description of the panic, or empty when fn returned.
func recovering(fn func()) (panicMsg string) { //nolint:nonamedreturns // Recover sets the panic description.
	defer func() {
		if r := recover(); r != nil {
			panicMsg = fmt.Sprintf("panicked: %v", r)
		}
	}()

	fn()

	return ""
}

// newEnv creates the injected dependencies for one case. The HTTP client answers
// every request with the fixture's response, or fails like net/http with a
// *url.Error carrying the full request URL when failTransport is set.
func newEnv(t *testing.T, fx *fixture, failTransport bool) *env {
	t.Helper()

	logs := &syncBuffer{}

	e := &env{
		fx:     fx,
		client: mockTypes.NewMockHTTPClient(t),
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

// trySend sends a message, recording its error and any panic in the env.
//
// Parameters:
//   - service: the service to send through.
//   - params: the send params, or nil.
func (e *env) trySend(service types.Service, params *types.Params) {
	e.panicMsg = recovering(func() { e.sendErr = service.Send("contract message", params) })
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

// Enabled reports true for every level.
//
// Returns:
//   - bool: always true.
func (logFunc) Enabled(context.Context, slog.Level) bool { return true }

// Handle passes the record's message to f.
//
// Parameters:
//   - record: the warning's record.
//
// Returns:
//   - error: always nil.
//
//nolint:gocritic // slog.Handler defines Handle with a Record value.
func (f logFunc) Handle(_ context.Context, record slog.Record) error {
	f(record.Message)

	return nil
}

// Slog returns a slog logger with f as its handler.
//
// Returns:
//   - *slog.Logger: the logger.
func (f logFunc) Slog() *slog.Logger { return slog.New(f) }

// WithAttrs returns f. The warnings add no attributes to the handler.
//
// Returns:
//   - slog.Handler: f.
func (f logFunc) WithAttrs([]slog.Attr) slog.Handler { return f }

// WithGroup returns f. The warnings add no groups to the handler.
//
// Returns:
//   - slog.Handler: f.
func (f logFunc) WithGroup(string) slog.Handler { return f }

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
