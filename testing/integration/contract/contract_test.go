package contract_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// env holds the injected client, dialer and logger for one contract case.
type env struct {
	fx       *fixture
	client   *mocks.MockHTTPClient
	dials    atomic.Int32
	logs     *syncBuffer
	logger   *log.Logger
	sendErr  error
	panicMsg string
}

// syncBuffer is a goroutine-safe log sink.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

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
	checkNilHTTPClient     = "nil_http_client"
	checkNoSecretLeaks     = "no_secret_leaks"
)

// Failure kinds, one per distinct way a check can fail.
const (
	failNone             failureKind = iota // the check passed
	failInit                                // the fixture failed to initialize
	failInitIO                              // Initialize performed network I/O
	failPanic                               // Send panicked
	failDefaultTransport                    // Send used http.DefaultTransport
	failNotInjected                         // Send skipped the injected client or dialer
	failUnexpectedIO                        // a service without network access performed I/O
	failRejectedParams                      // reserved params were rejected as unknown keys
	failInjectedAfterNil                    // the injected client was still used after SetHTTPClient(nil)
	failSecretLeak                          // a secret appeared in errors or logs
)

var (
	errTransport        = errors.New("contract: transport failure")
	errDial             = errors.New("contract: dial blocked")
	errDefaultTransport = errors.New("contract: http.DefaultTransport must not be used")
	errLookupBlocked    = errors.New("contract: DNS lookups are blocked")
)

// defaultTransportHits counts uses of http.DefaultTransport across the package.
var defaultTransportHits atomic.Int64

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
		{checkNilHTTPClient, runNilHTTPClient},
		{checkNoSecretLeaks, runNoSecretLeaks},
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

	var result failure

	synctest.Test(t, func(t *testing.T) {
		env := newEnv(t, fx, false)
		before := defaultTransportHits.Load()

		_, err := env.locate(t)

		if hits := int64(env.ioCount()) + defaultTransportHits.Load() - before; hits > 0 {
			result = fail(failInitIO, "performed %d network operation(s) during Initialize", hits)
		} else if err != nil {
			t.Fatalf("fixture %q failed to initialize: %v", fx.url, err)
		}
	})

	return result
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

// runNilHTTPClient verifies that SetHTTPClient(nil) restores a usable default
// instead of leaving the service to panic on Send.
func runNilHTTPClient(t *testing.T, fx *fixture) failure {
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

		setter.SetHTTPClient(nil)

		httpBefore := env.httpCalls()

		if result = env.send(service, nil); result.kind != failNone {
			return
		}

		if env.httpCalls() > httpBefore {
			result = fail(failInjectedAfterNil, "Send still used the injected client after SetHTTPClient(nil)")
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
		if err == nil {
			if result = env.send(service, nil); result.kind != failNone {
				return
			}

			err = env.sendErr
		}

		output := env.logs.String()
		if err != nil {
			output += "\n" + err.Error()
		}

		for _, secret := range fx.secrets {
			if strings.Contains(output, secret) {
				result = fail(failSecretLeak, "secret %q leaked into errors or logs", secret)

				return
			}
		}
	})

	return result
}

// newEnv creates the injected dependencies for one case. The HTTP client answers
// every request with the fixture's response, or fails like net/http with a
// *url.Error carrying the full request URL when failTransport is set.
func newEnv(t *testing.T, fx *fixture, failTransport bool) *env {
	t.Helper()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(respond(fx, failTransport)).Maybe()

	logs := &syncBuffer{}

	return &env{
		fx:     fx,
		client: client,
		logs:   logs,
		logger: log.New(logs, "", 0),
	}
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

// dial is the injected DialContext; it records and refuses every dial.
func (e *env) dial(context.Context, string, string) (net.Conn, error) {
	e.dials.Add(1)

	return nil, errDial
}

// httpCalls returns how many requests reached the injected client.
func (e *env) httpCalls() int { return len(e.client.Calls) }

func (e *env) ioCount() int { return e.httpCalls() + int(e.dials.Load()) }

// locate builds the service through the router, so injection follows the same
// path consumers use.
func (e *env) locate(t *testing.T) (types.Service, error) {
	t.Helper()

	serviceRouter, err := router.NewWithOptions(e.logger, types.SenderOptions{
		HTTPClient:  e.client,
		DialContext: e.dial,
	})
	if err != nil {
		t.Fatalf("creating router: %v", err)
	}

	service, err := serviceRouter.Locate(e.fx.url)
	if err != nil {
		return nil, fmt.Errorf("locating service: %w", err)
	}

	return service, nil
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
