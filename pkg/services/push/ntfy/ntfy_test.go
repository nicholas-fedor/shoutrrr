package ntfy

import (
	"io"
	"net/http"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	mock "github.com/stretchr/testify/mock"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
	jsonclientmocks "github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient/mocks"
)

var _ = ginkgo.Describe("Service", func() {
	var (
		service  *Service
		mockJSON *jsonclientmocks.MockClient
		logger   types.StdLogger
	)

	ginkgo.BeforeEach(func() {
		mockJSON = newMockJSONClient()
		logger = &noOpLogger{}
		service = &Service{
			Config: &Config{
				Scheme: "https",
				Host:   "ntfy.sh",
				Topic:  "mytopic",
			},
		}
		service.SetLogger(logger)
		service.client = mockJSON
	})

	ginkgo.Describe("GetID", func() {
		ginkgo.It("should return the scheme name", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal(Scheme))
		})
	})

	ginkgo.Describe("Initialize", func() {
		ginkgo.It("should parse a valid URL and initialize config", func() {
			serviceURL := mustParseURL("ntfy://user:pass@ntfy.example.com/mytopic?priority=5")

			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config).NotTo(gomega.BeNil())
			gomega.Expect(service.Config.Host).To(gomega.Equal("ntfy.example.com"))
			gomega.Expect(service.Config.Topic).To(gomega.Equal("mytopic"))
			gomega.Expect(service.Config.Username).To(gomega.Equal("user"))
			gomega.Expect(service.Config.Password).To(gomega.Equal("pass"))
			gomega.Expect(service.Config.Priority).To(gomega.Equal(PriorityMax))
		})

		ginkgo.It("should return an error when topic is missing", func() {
			serviceURL := mustParseURL("ntfy://ntfy.example.com/")

			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).To(gomega.Equal(ErrTopicRequired))
		})

		ginkgo.It("should skip topic validation for dummy URL", func() {
			serviceURL := mustParseURL("ntfy://dummy@dummy.com")

			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should force HTTP scheme when DisableTLS is set", func() {
			serviceURL := mustParseURL("ntfy://ntfy.example.com/mytopic?disabletls=yes")

			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Scheme).To(gomega.Equal("http"))
		})

		ginkgo.It("should create HTTP client with TLS 1.2 minimum when TLS is enabled", func() {
			serviceURL := mustParseURL("ntfy://ntfy.example.com/mytopic")

			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.httpClient).NotTo(gomega.BeNil())
		})

		ginkgo.It("should create HTTP client with InsecureSkipVerify when DisableTLSVerification is set", func() {
			serviceURL := mustParseURL("ntfy://ntfy.example.com/mytopic?disabletlsverification=yes")

			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.httpClient).NotTo(gomega.BeNil())
		})

		ginkgo.It("should use existing HTTP client when already set", func() {
			existingClient := &http.Client{}
			service.httpClient = existingClient
			serviceURL := mustParseURL("ntfy://ntfy.example.com/mytopic")

			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.httpClient).To(gomega.BeIdenticalTo(existingClient))
		})

		ginkgo.It("should create jsonclient from HTTP client", func() {
			serviceURL := mustParseURL("ntfy://ntfy.example.com/mytopic")

			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.client).NotTo(gomega.BeNil())
		})
	})

	ginkgo.Describe("Send", func() {
		ginkgo.BeforeEach(func() {
			mockJSON.On("Headers").Return(http.Header{})
		})

		ginkgo.It("should send message via sendAPI", func() {
			serviceURL := mustParseURL("ntfy://ntfy.example.com/mytopic")
			gomega.Expect(service.Initialize(serviceURL, logger)).NotTo(gomega.HaveOccurred())
			service.client = mockJSON

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.Send("hello", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should update config from params before sending", func() {
			serviceURL := mustParseURL("ntfy://ntfy.example.com/mytopic")
			gomega.Expect(service.Initialize(serviceURL, logger)).NotTo(gomega.HaveOccurred())
			service.client = mockJSON

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			params := &types.Params{"title": "New Title"}
			err := service.Send("hello", params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Title).To(gomega.Equal("New Title"))
		})

		ginkgo.It("should return error when sendAPI fails", func() {
			serviceURL := mustParseURL("ntfy://ntfy.example.com/mytopic")
			gomega.Expect(service.Initialize(serviceURL, logger)).NotTo(gomega.HaveOccurred())
			service.client = mockJSON

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(io.ErrClosedPipe)
			mockJSON.EXPECT().ErrorResponse(mock.Anything, mock.Anything).
				Return(false)

			err := service.Send("hello", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("SetHTTPClient", func() {
		ginkgo.It("should set the HTTP client and recreate jsonclient", func() {
			newClient := &http.Client{}
			service.SetHTTPClient(newClient)
			gomega.Expect(service.httpClient).To(gomega.BeIdenticalTo(newClient))
			gomega.Expect(service.client).NotTo(gomega.BeNil())
		})

		ginkgo.It("should not recreate jsonclient when client is nil", func() {
			existingJSON := newMockJSONClient()
			service.client = existingJSON
			service.SetHTTPClient(nil)
			gomega.Expect(service.httpClient).To(gomega.BeNil())
			gomega.Expect(service.client).To(gomega.BeIdenticalTo(existingJSON))
		})
	})

	ginkgo.Describe("sendAPI", func() {
		var headers http.Header

		ginkgo.BeforeEach(func() {
			service.Config = &Config{
				Scheme: "https",
				Host:   "ntfy.example.com",
				Topic:  "mytopic",
			}
			headers = http.Header{}
			mockJSON.On("Headers").Return(headers)
		})

		ginkgo.It("should set Content-Type to text/plain by default", func() {
			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Content-Type")).To(gomega.Equal("text/plain; charset=utf-8"))
		})

		ginkgo.It("should set Content-Type to text/markdown when Markdown is enabled", func() {
			service.Config.Markdown = true

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "**hello**")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Content-Type")).To(gomega.Equal("text/markdown"))
		})

		ginkgo.It("should set Content-Type to text/plain when Markdown is disabled", func() {
			service.Config.Markdown = false

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Content-Type")).To(gomega.Equal("text/plain; charset=utf-8"))
		})

		ginkgo.It("should set User-Agent header with shoutrrr version", func() {
			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("User-Agent")).To(gomega.ContainSubstring("shoutrrr/"))
		})

		ginkgo.It("should set Title header when configured", func() {
			service.Config.Title = "Alert"

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Title")).To(gomega.Equal("Alert"))
		})

		ginkgo.It("should set Priority header when configured", func() {
			service.Config.Priority = PriorityHigh

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Priority")).To(gomega.Equal("High"))
		})

		ginkgo.It("should set Tags header as comma-separated list", func() {
			service.Config.Tags = []string{"warning", "skull"}

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Tags")).To(gomega.Equal("warning,skull"))
		})

		ginkgo.It("should set Basic Auth header when username and password are provided", func() {
			service.Config.Username = "user"
			service.Config.Password = "pass"

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Authorization")).To(gomega.HavePrefix("Basic "))
		})

		ginkgo.It("should skip Cache header when Cache is disabled", func() {
			service.Config.Cache = false

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Cache")).To(gomega.Equal("no"))
		})

		ginkgo.It("should skip Firebase header when Firebase is disabled", func() {
			service.Config.Firebase = false

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Firebase")).To(gomega.Equal("no"))
		})

		ginkgo.It("should set Actions header when configured", func() {
			service.Config.Actions = []string{"view", "open"}

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Actions")).To(gomega.Equal("view;open"))
		})

		ginkgo.It("should set Delay header when configured", func() {
			service.Config.Delay = "2h"

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Delay")).To(gomega.Equal("2h"))
		})

		ginkgo.It("should set Click header when configured", func() {
			service.Config.Click = "https://example.com"

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Click")).To(gomega.Equal("https://example.com"))
		})

		ginkgo.It("should set Attach header when configured", func() {
			service.Config.Attach = "https://example.com/image.png"

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Attach")).To(gomega.Equal("https://example.com/image.png"))
		})

		ginkgo.It("should set X-Icon header when configured", func() {
			service.Config.Icon = "https://example.com/icon.png"

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("X-Icon")).To(gomega.Equal("https://example.com/icon.png"))
		})

		ginkgo.It("should set Filename header when configured", func() {
			service.Config.Filename = "document.pdf"

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Filename")).To(gomega.Equal("document.pdf"))
		})

		ginkgo.It("should set Email header when configured", func() {
			service.Config.Email = "user@example.com"

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Email")).To(gomega.Equal("user@example.com"))
		})

		ginkgo.It("should call Post with the API URL and message body", func() {
			mockJSON.On("Post", service.Config.GetAPIURL(), "hello", mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should set Basic Auth header with username only", func() {
			service.Config.Username = "user"
			service.Config.Password = ""

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Authorization")).To(gomega.HavePrefix("Basic "))
		})

		ginkgo.It("should set Basic Auth header with password only", func() {
			service.Config.Username = ""
			service.Config.Password = "pass"

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Authorization")).To(gomega.HavePrefix("Basic "))
		})

		ginkgo.It("should send empty message without error", func() {
			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should set single tag without comma separator", func() {
			service.Config.Tags = []string{"warning"}

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(headers.Get("Tags")).To(gomega.Equal("warning"))
		})

		ginkgo.It("should return error when Post fails", func() {
			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(io.ErrClosedPipe)
			mockJSON.On("ErrorResponse", mock.Anything, mock.Anything).
				Return(false)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should return apiResponseError when API returns structured error", func() {
			responseBody := `{"code":400,"error":"invalid request","link":"https://docs.ntfy.sh"}`

			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(jsonclient.Error{
					StatusCode: 400,
					Body:       responseBody,
				})
			mockJSON.EXPECT().ErrorResponse(mock.Anything, mock.Anything).
				RunAndReturn(func(_ error, response any) bool {
					if resp, ok := response.(*apiResponseError); ok {
						resp.Code = 400
						resp.Message = "invalid request"
						resp.Link = "https://docs.ntfy.sh"
					}

					return true
				})

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid request"))
		})

		ginkgo.It("should return wrapped error when Post fails and ErrorResponse parsing fails", func() {
			mockJSON.On("Post", mock.Anything, mock.Anything, mock.Anything).
				Return(io.ErrClosedPipe)
			mockJSON.On("ErrorResponse", mock.Anything, mock.Anything).
				Return(false)

			err := service.sendAPI(service.Config, "hello")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("posting to ntfy API"))
		})
	})
})

var _ = ginkgo.Describe("addHeaderIfNotEmpty", func() {
	ginkgo.It("should add header when value is non-empty", func() {
		headers := http.Header{}
		addHeaderIfNotEmpty(&headers, "X-Test", "value")
		gomega.Expect(headers.Get("X-Test")).To(gomega.Equal("value"))
	})

	ginkgo.It("should not add header when value is empty", func() {
		headers := http.Header{}
		addHeaderIfNotEmpty(&headers, "X-Test", "")
		gomega.Expect(headers.Get("X-Test")).To(gomega.Equal(""))
	})

	ginkgo.It("should append multiple values for the same header", func() {
		headers := http.Header{}
		addHeaderIfNotEmpty(&headers, "X-Test", "value1")
		addHeaderIfNotEmpty(&headers, "X-Test", "value2")
		gomega.Expect(headers["X-Test"]).To(gomega.Equal([]string{"value1", "value2"}))
	})
})

var _ = ginkgo.Describe("TLS configuration", func() {
	var (
		svc        *Service
		testLogger types.StdLogger
	)

	ginkgo.BeforeEach(func() {
		testLogger = &noOpLogger{}
	})

	ginkgo.Describe("DisableTLSVerification", func() {
		ginkgo.It("should set InsecureSkipVerify when DisableTLSVerification is true", func() {
			svc = &Service{}
			serviceURL := mustParseURL("ntfy://example.com/test?disabletlsverification=yes")
			gomega.Expect(svc.Initialize(serviceURL, testLogger)).To(gomega.Succeed())
			transport := svc.httpClient.(*http.Client).Transport.(*http.Transport)
			gomega.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(gomega.BeTrue())
		})

		ginkgo.It("should set DisableTLSVerification config when enabled", func() {
			svc = &Service{}
			serviceURL := mustParseURL("ntfy://example.com/test?disabletlsverification=yes")
			gomega.Expect(svc.Initialize(serviceURL, testLogger)).To(gomega.Succeed())
			gomega.Expect(svc.Config.DisableTLSVerification).To(gomega.BeTrue())
		})

		ginkgo.It("should not set InsecureSkipVerify when DisableTLSVerification is false", func() {
			svc = &Service{}
			serviceURL := mustParseURL("ntfy://example.com/test")
			gomega.Expect(svc.Initialize(serviceURL, testLogger)).To(gomega.Succeed())
			transport := svc.httpClient.(*http.Client).Transport.(*http.Transport)
			gomega.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("DisableTLS", func() {
		ginkgo.It("should use HTTP scheme when DisableTLS is true", func() {
			svc = &Service{}
			serviceURL := mustParseURL("ntfy://example.com/test?disabletls=yes")
			gomega.Expect(svc.Initialize(serviceURL, testLogger)).To(gomega.Succeed())
			gomega.Expect(svc.Config.GetAPIURL()).To(gomega.Equal("http://example.com/test"))
		})

		ginkgo.It("should use HTTPS scheme when DisableTLS is false", func() {
			config := &Config{
				Host:       "example.com",
				Topic:      "test",
				Scheme:     "https",
				DisableTLS: false,
			}
			gomega.Expect(config.GetAPIURL()).To(gomega.Equal("https://example.com/test"))
		})
	})
})

var _ = ginkgo.Describe("service identification", func() {
	ginkgo.It("should return the correct service ID", func() {
		svc := &Service{}
		gomega.Expect(svc.GetID()).To(gomega.Equal(Scheme))
	})
})

var _ = ginkgo.Describe("service API compliance", func() {
	var (
		svc        *Service
		testLogger types.StdLogger
	)

	ginkgo.BeforeEach(func() {
		testLogger = &noOpLogger{}
	})

	ginkgo.It("should pass config API compliance checks", func() {
		testutils.TestConfigSetInvalidQueryValue(&Config{}, "ntfy://host/topic?foo=bar")
		testutils.TestConfigGetInvalidQueryValue(&Config{})
		testutils.TestConfigSetDefaultValues(&Config{})
		testutils.TestConfigGetEnumsCount(&Config{}, 1)
		testutils.TestConfigGetFieldsCount(&Config{}, 18)
	})

	ginkgo.It("should pass service API compliance checks", func() {
		svc = &Service{}
		serviceURL := mustParseURL("ntfy://:devicekey@hostname/testtopic")
		gomega.Expect(svc.Initialize(serviceURL, testLogger)).To(gomega.Succeed())
		testutils.TestServiceSetInvalidParamValue(svc, "foo", "bar")
	})
})
