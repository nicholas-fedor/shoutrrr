package twilio

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const testHookURL = "https://api.twilio.com/2010-04-01/Accounts/ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/Messages.json"

var _ = ginkgo.Describe("Service Unit Tests", func() {
	var (
		service    *Service
		mockClient *mockServiceHTTPClient
	)

	ginkgo.BeforeEach(func() {
		service = &Service{
			Config: &Config{
				AccountSID: "ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
				AuthToken:  "authToken",
				FromNumber: "+15551234567",
				ToNumbers:  []string{"+15559876543"},
			},
			HTTPClient: &mockServiceHTTPClient{},
		}
		service.SetLogger(&mockLogger{})
		service.pkr = *initPKR(service.Config)
		mockClient = service.HTTPClient.(*mockServiceHTTPClient)
		mockClient.response = &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(`{"sid": "SM123"}`)),
		}
	})

	ginkgo.Describe("Initialize", func() {
		ginkgo.It("should initialize from a valid URL", func() {
			svc := &Service{}
			serviceURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = svc.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(svc.Config.AccountSID).To(gomega.Equal("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
			gomega.Expect(svc.HTTPClient).NotTo(gomega.BeNil())
		})

		ginkgo.It("should return an error for invalid URL", func() {
			svc := &Service{}
			serviceURL, err := url.Parse("twilio://:@/")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = svc.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("GetID", func() {
		ginkgo.It("should return 'twilio'", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal("twilio"))
		})
	})

	ginkgo.Describe("Send", func() {
		ginkgo.It("should send successfully to a single recipient", func() {
			err := service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
		})

		ginkgo.It("should send to multiple recipients", func() {
			service.Config.ToNumbers = []string{"+15559876543", "+15551111111"}
			mockClient.response = &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader("")),
			}

			err := service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(2))
		})

		ginkgo.It("should prepend title to message body when title is set", func() {
			service.Config.Title = "Alert"
			mockClient.captureBody = true

			err := service.Send("Something happened", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			// Body is URL-encoded form data
			gomega.Expect(mockClient.lastBody).To(gomega.ContainSubstring("Alert"))
			gomega.Expect(mockClient.lastBody).To(gomega.ContainSubstring("Something"))
		})

		ginkgo.It("should return an error on HTTP failure", func() {
			mockClient.err = errors.New("network error")

			err := service.Send("Message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should return an error for non-2xx status codes", func() {
			mockClient.response = &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("")),
			}

			err := service.Send("Message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("Params update", func() {
		ginkgo.It("should update config from params", func() {
			params := types.Params{"title": "Updated Title"}

			err := service.Send("Message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Title).To(gomega.Equal("Updated Title"))
		})
	})
})

// initPKR initializes a PropKeyResolver for testing.
func initPKR(config *Config) *format.PropKeyResolver {
	pkr := format.NewPropKeyResolver(config)
	return &pkr
}

// mockServiceHTTPClient is a test helper that implements HTTPClient interface.
type mockServiceHTTPClient struct {
	response       *http.Response
	err            error
	callCount      int
	captureBody    bool
	captureHeaders bool
	lastBody       string
	lastRequest    *http.Request
}

func (m *mockServiceHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.callCount++

	if m.captureHeaders {
		m.lastRequest = req
	}

	if m.captureBody && req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err == nil {
			m.lastBody = string(body)
		}
	}

	if m.err != nil {
		return nil, m.err
	}

	// Return a fresh response body for each call (io.NopCloser can only be read once)
	if m.response != nil && m.callCount > 1 {
		return &http.Response{
			StatusCode: m.response.StatusCode,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}

	return m.response, nil
}

// mockLogger is a test helper that implements StdLogger interface.
type mockLogger struct{}

func (m *mockLogger) Print(_ ...any)            {}
func (m *mockLogger) Printf(_ string, _ ...any) {}
func (m *mockLogger) Println(_ ...any)          {}
