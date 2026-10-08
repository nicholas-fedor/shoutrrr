package matrix

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	ginkgo "github.com/onsi/ginkgo/v2"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix/mocks"
)

// fakeHomeserver answers Matrix API requests and counts password logins.
type fakeHomeserver struct {
	// logins counts POST requests to the login endpoint.
	logins atomic.Int32
	// failFlows makes the next login flows request fail with a server error.
	failFlows atomic.Bool
	// flowsStarted, when set, receives a value as each login flows request arrives.
	flowsStarted chan struct{}
	// releaseFlows, when set, holds each login flows request until it is closed.
	releaseFlows chan struct{}
	// mu guards tokens.
	mu sync.Mutex
	// tokens records the Authorization header of each message sent.
	tokens []string
}

// passwordURL is a Matrix URL that authenticates with a user and password.
const passwordURL = "matrix://user:SECRETpassword@matrix.example.com/?rooms=!room:example.com"

var _ = ginkgo.Describe("Service", func() {
	ginkgo.Describe("GetID", func() {
		ginkgo.It("should return the matrix scheme identifier", func() {
			svc := &Service{}
			gomega.Expect(svc.GetID()).To(gomega.Equal(Scheme))
		})

		ginkgo.It("should always return 'matrix' regardless of service state", func() {
			svc := &Service{
				Config: &Config{
					Host: "matrix.example.com",
				},
			}
			gomega.Expect(svc.GetID()).To(gomega.Equal("matrix"))
		})
	})

	ginkgo.Describe("Initialize", func() {
		var svc *Service

		ginkgo.BeforeEach(func() {
			svc = &Service{}
		})

		ginkgo.Context("with invalid URL", func() {
			ginkgo.It("should return error when host is missing", func() {
				invalidURL := &url.URL{
					Scheme: "matrix",
					User:   url.UserPassword("user", "password"),
				}
				err := svc.Initialize(invalidURL, nil)
				gomega.Expect(err).To(gomega.MatchError(ErrMissingHost))
			})

			ginkgo.It("should return error when password is missing", func() {
				invalidURL := &url.URL{
					Scheme: "matrix",
					Host:   "matrix.example.com",
					User:   url.User("user"),
				}
				err := svc.Initialize(invalidURL, nil)
				gomega.Expect(err).To(gomega.MatchError(ErrMissingCredentials))
			})
		})

		ginkgo.Context("with valid URL but no client creation", func() {
			// Test that we can set config without triggering client initialization
			// The dummy URL check happens after config parsing, so we test config validation separately
			ginkgo.It("should reject URL without user info when credentials required", func() {
				// URL with host but no user - should fail at config validation
				invalidURL := &url.URL{
					Scheme: "matrix",
					Host:   "matrix.example.com",
				}
				err := svc.Initialize(invalidURL, nil)
				gomega.Expect(err).To(gomega.MatchError(ErrMissingCredentials))
			})
		})
	})

	ginkgo.Describe("lazy login", func() {
		var (
			server     *fakeHomeserver
			httpClient *mocks.MockHTTPClient
			svc        *Service
		)

		ginkgo.BeforeEach(func() {
			server = &fakeHomeserver{}
			httpClient = mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			httpClient.EXPECT().Do(mock.Anything).RunAndReturn(server.Do).Maybe()

			svc = &Service{}
			svc.SetHTTPClient(httpClient)
		})

		ginkgo.It("should not contact the homeserver during Initialize", func() {
			gomega.Expect(svc.Initialize(testutils.URLMust(passwordURL), nil)).To(gomega.Succeed())
			httpClient.AssertNotCalled(ginkgo.GinkgoT(), "Do", mock.Anything)
		})

		ginkgo.It("should log in on the first send through the injected client", func() {
			gomega.Expect(svc.Initialize(testutils.URLMust(passwordURL), nil)).To(gomega.Succeed())

			gomega.Expect(svc.Send("first", nil)).To(gomega.Succeed())
			gomega.Expect(svc.Send("second", nil)).To(gomega.Succeed())

			gomega.Expect(server.logins.Load()).To(gomega.Equal(int32(1)))
			gomega.Expect(server.sentTokens()).To(gomega.Equal([]string{"Bearer access", "Bearer access"}))
		})

		ginkgo.It("should log in once across concurrent sends", func() {
			gomega.Expect(svc.Initialize(testutils.URLMust(passwordURL), nil)).To(gomega.Succeed())

			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					defer ginkgo.GinkgoRecover()

					gomega.Expect(svc.Send("concurrent", nil)).To(gomega.Succeed())
				})
			}

			wg.Wait()
			gomega.Expect(server.logins.Load()).To(gomega.Equal(int32(1)))
		})

		ginkgo.It("should retry a failed login on the next send", func() {
			gomega.Expect(svc.Initialize(testutils.URLMust(passwordURL), nil)).To(gomega.Succeed())

			server.failFlows.Store(true)

			err := svc.Send("first", nil)
			gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("logging in to Matrix")))
			gomega.Expect(err.Error()).NotTo(gomega.ContainSubstring("SECRETpassword"))

			gomega.Expect(svc.Send("second", nil)).To(gomega.Succeed())
			gomega.Expect(server.logins.Load()).To(gomega.Equal(int32(1)))
		})

		ginkgo.It("should stop waiting for another send's login when its context ends", func() {
			server.flowsStarted = make(chan struct{}, 1)
			server.releaseFlows = make(chan struct{})

			gomega.Expect(svc.Initialize(testutils.URLMust(passwordURL), nil)).To(gomega.Succeed())

			firstDone := make(chan error, 1)

			go func() { firstDone <- svc.Send("first", nil) }()

			gomega.Eventually(server.flowsStarted).Should(gomega.Receive())

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := svc.SendWithContext(ctx, "second", nil)
			gomega.Expect(errors.Is(err, context.Canceled)).To(gomega.BeTrue())

			close(server.releaseFlows)
			gomega.Eventually(firstDone).Should(gomega.Receive(gomega.Succeed()))
			gomega.Expect(server.logins.Load()).To(gomega.Equal(int32(1)))
		})

		ginkgo.It("should drop the previous client when reinitialized with the placeholder URL", func() {
			gomega.Expect(svc.Initialize(testutils.URLMust(passwordURL), nil)).To(gomega.Succeed())
			gomega.Expect(svc.Initialize(testutils.URLMust("matrix://dummy@dummy.com"), nil)).To(gomega.Succeed())

			gomega.Expect(svc.Send("message", nil)).To(gomega.MatchError(ErrClientNotInitialized))
			httpClient.AssertNotCalled(ginkgo.GinkgoT(), "Do", mock.Anything)
		})

		ginkgo.It("should use an access token without logging in", func() {
			tokenURL := "matrix://:token@matrix.example.com/?rooms=!room:example.com"
			gomega.Expect(svc.Initialize(testutils.URLMust(tokenURL), nil)).To(gomega.Succeed())

			gomega.Expect(svc.Send("message", nil)).To(gomega.Succeed())
			gomega.Expect(server.logins.Load()).To(gomega.BeZero())
			gomega.Expect(server.sentTokens()).To(gomega.Equal([]string{"Bearer token"}))
		})
	})

	ginkgo.Describe("Send", func() {
		var svc *Service

		ginkgo.BeforeEach(func() {
			svc = &Service{}
		})

		ginkgo.Context("when client is not initialized", func() {
			ginkgo.It("should return ErrClientNotInitialized", func() {
				svc.Config = &Config{}
				err := svc.Send("test message", nil)
				gomega.Expect(err).To(gomega.MatchError(ErrClientNotInitialized))
			})
		})

		ginkgo.Context("with initialized service but no client", func() {
			ginkgo.BeforeEach(func() {
				// Set config directly without initializing client
				svc.Config = &Config{
					Host: "matrix.example.com",
				}
				// Forcefully set client to nil to test the error path
				svc.client = nil
			})

			ginkgo.It("should return error when client is nil", func() {
				err := svc.Send("test message", nil)
				gomega.Expect(err).To(gomega.MatchError(ErrClientNotInitialized))
			})
		})
	})
})

// Do answers a Matrix API request: login flows, password login, and message sends.
func (f *fakeHomeserver) Do(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
	}

	if req.Method == http.MethodGet && req.URL.Path == apiLogin && f.releaseFlows != nil {
		f.flowsStarted <- struct{}{}

		<-f.releaseFlows
	}

	switch {
	case req.Method == http.MethodGet && req.URL.Path == apiLogin && f.failFlows.CompareAndSwap(true, false):
		return fakeResponse(http.StatusInternalServerError, `{"errcode":"M_UNKNOWN","error":"unavailable"}`), nil
	case req.Method == http.MethodGet && req.URL.Path == apiLogin:
		return fakeResponse(http.StatusOK, `{"flows":[{"type":"m.login.password"}]}`), nil
	case req.Method == http.MethodPost && req.URL.Path == apiLogin:
		f.logins.Add(1)

		return fakeResponse(http.StatusOK, `{"access_token":"access"}`), nil
	case req.Method == http.MethodPut && strings.Contains(req.URL.Path, "/send/"):
		f.mu.Lock()
		f.tokens = append(f.tokens, req.Header.Get("Authorization"))
		f.mu.Unlock()

		return fakeResponse(http.StatusOK, `{"event_id":"$event"}`), nil
	default:
		return fakeResponse(http.StatusNotFound, `{"errcode":"M_NOT_FOUND","error":"not found"}`), nil
	}
}

// sentTokens returns the Authorization header of each message sent so far.
func (f *fakeHomeserver) sentTokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.tokens...)
}

// fakeResponse builds a JSON response with the given status and body.
func fakeResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
