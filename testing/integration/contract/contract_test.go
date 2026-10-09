package contract_test

import (
	"bytes"
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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

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

// TestContract runs each contract check against every registered scheme.
//
// Known failures are listed in [allowlist]. They are reported as skips, and an
// allowlisted case that starts passing fails, so each fix must remove its entry.
//
//nolint:paralleltest // Swaps http.DefaultTransport and counts its use per case.
func TestContract(t *testing.T) {
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
