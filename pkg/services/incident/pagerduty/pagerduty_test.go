package pagerduty

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	mockIntegrationKey = "a1b2c3d4e5f678901234567890abcdef"
	mockHost           = "events.pagerduty.com"
)

func TestPagerDuty(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Pagerduty Suite")
}

var _ = ginkgo.Describe("the PagerDuty service", func() {
	var (
		// a simulated http server to mock out PagerDuty itself
		mockServer *httptest.Server
		// the host of our mock server
		mockHost string
		// the port of our mock server
		mockPort string
		// function to check if the http request received by the mock server is as expected
		checkRequest func(body string, header http.Header)
		// the shoutrrr PagerDuty service
		service *Service
		// just a mock logger
		mockLogger *log.Logger
		// test-scoped HTTP client that trusts the mock server's certificate
		mockHTTPClient *http.Client
	)

	ginkgo.BeforeEach(func() {
		// Initialize a mock http server
		httpHandler := func(_ http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			defer r.Body.Close()

			checkRequest(string(body), r.Header)
		}
		mockServer = httptest.NewTLSServer(http.HandlerFunc(httpHandler))

		// Create a test-scoped HTTP client that trusts the mock server's self-signed certificate
		mockHTTPClient = mockServer.Client()

		// Determine the host and port of our mock http server
		mockServerURL, err := url.Parse(mockServer.URL)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		mockHost, mockPort, err = net.SplitHostPort(mockServerURL.Host)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())

		// Initialize a mock logger
		var buf bytes.Buffer
		mockLogger = log.New(&buf, "", 0)
	})

	ginkgo.AfterEach(func() {
		mockServer.Close()
	})

	ginkgo.Context("without query parameters", func() {
		ginkgo.BeforeEach(func() {
			// Initialize service
			serviceURL, err := url.Parse(
				fmt.Sprintf(
					"pagerduty://%s/%s",
					net.JoinHostPort(mockHost, mockPort),
					mockIntegrationKey,
				),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			service = &Service{}
			err = service.Initialize(serviceURL, mockLogger)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			service.SetHTTPClient(mockHTTPClient)
		})

		ginkgo.When("sending a simple alert", func() {
			ginkgo.It(
				"should send a request to our mock PagerDuty server with default values",
				func() {
					checkRequest = func(body string, header http.Header) {
						gomega.Expect(header["Content-Type"][0]).
							To(gomega.Equal("application/json"))
						gomega.Expect(body).To(gomega.Equal(`{` +
							`"payload":{` +
							`"summary":"hello world",` +
							`"severity":"error",` +
							`"source":"default"` +
							`},` +
							`"routing_key":"a1b2c3d4e5f678901234567890abcdef",` +
							`"event_action":"trigger"` +
							`}`))
					}

					err := service.Send("hello world", &types.Params{})
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				},
			)
		})

		ginkgo.Context("message truncation", func() {
			ginkgo.BeforeEach(func() {
				serviceURL, err := url.Parse(
					fmt.Sprintf(
						"pagerduty://%s/%s",
						net.JoinHostPort(mockHost, mockPort),
						mockIntegrationKey,
					),
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				service = &Service{}
				err = service.Initialize(serviceURL, mockLogger)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				service.SetHTTPClient(mockHTTPClient)
			})

			ginkgo.When("sending a message longer than 1024 characters", func() {
				ginkgo.It("should truncate the message to 1024 characters", func() {
					longMessage := strings.Repeat("a", 1100)
					checkRequest = func(body string, header http.Header) {
						gomega.Expect(header["Content-Type"][0]).
							To(gomega.Equal("application/json"))
						var payload EventPayload
						err := json.Unmarshal([]byte(body), &payload)
						gomega.Expect(err).ToNot(gomega.HaveOccurred())
						gomega.Expect(payload.Payload.Summary).To(gomega.HaveLen(1024))
						gomega.Expect(payload.Payload.Summary).
							To(gomega.Equal(strings.Repeat("a", 1024)))
					}

					err := service.Send(longMessage, &types.Params{})
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Context("integration key validation", func() {
			ginkgo.When("using an invalid integration key", func() {
				ginkgo.It("should return an error during initialization", func() {
					invalidKey := "invalid-key"
					serviceURL, err := url.Parse(
						fmt.Sprintf("pagerduty://%s/%s", mockHost, invalidKey),
					)
					gomega.Expect(err).ToNot(gomega.HaveOccurred())

					service = &Service{}
					err = service.Initialize(serviceURL, mockLogger)
					gomega.Expect(err).To(gomega.HaveOccurred())
					gomega.Expect(err.Error()).
						To(gomega.ContainSubstring("invalid integration key format"))
				})
			})
		})

		ginkgo.Context("HTTP timeout scenarios", func() {
			ginkgo.BeforeEach(func() {
				serviceURL, err := url.Parse(
					fmt.Sprintf(
						"pagerduty://%s/%s",
						net.JoinHostPort(mockHost, mockPort),
						mockIntegrationKey,
					),
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				service = &Service{}
				err = service.Initialize(serviceURL, mockLogger)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				service.SetHTTPClient(mockHTTPClient)
			})

			ginkgo.When("HTTP client has a timeout", func() {
				ginkgo.It("should use the configured timeout", func() {
					// The test setup already uses a timeout, so we just verify the service works
					checkRequest = func(_ string, header http.Header) {
						gomega.Expect(header["Content-Type"][0]).
							To(gomega.Equal("application/json"))
					}

					err := service.Send("test message", &types.Params{})
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Context("optional fields support", func() {
			ginkgo.BeforeEach(func() {
				serviceURL, err := url.Parse(
					fmt.Sprintf(
						"pagerduty://%s/%s?client=my-monitor&client_url=https://example.com&details=%%7B%%22key%%22%%3A%%22value%%22%%7D",
						net.JoinHostPort(mockHost, mockPort),
						mockIntegrationKey,
					),
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				service = &Service{}
				err = service.Initialize(serviceURL, mockLogger)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				service.SetHTTPClient(mockHTTPClient)
			})

			ginkgo.When("sending with optional fields", func() {
				ginkgo.It("should include optional fields in the payload", func() {
					checkRequest = func(body string, header http.Header) {
						gomega.Expect(header["Content-Type"][0]).
							To(gomega.Equal("application/json"))
						var payload EventPayload
						err := json.Unmarshal([]byte(body), &payload)
						gomega.Expect(err).ToNot(gomega.HaveOccurred())
						gomega.Expect(payload.Client).To(gomega.Equal("my-monitor"))
						gomega.Expect(payload.ClientURL).To(gomega.Equal("https://example.com"))
						gomega.Expect(payload.Details).
							To(gomega.Equal(map[string]any{"key": "value"}))
					}

					err := service.Send("test message", &types.Params{})
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Context("contexts support", func() {
			ginkgo.BeforeEach(func() {
				serviceURL, err := url.Parse(
					fmt.Sprintf(
						"pagerduty://%s/%s?contexts=link:http://example.com,text:Additional+context,image:http://example.com/img.png",
						net.JoinHostPort(mockHost, mockPort),
						mockIntegrationKey,
					),
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				service = &Service{}
				err = service.Initialize(serviceURL, mockLogger)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				service.SetHTTPClient(mockHTTPClient)
			})

			ginkgo.When("sending with contexts", func() {
				ginkgo.It("should include contexts in the payload", func() {
					checkRequest = func(body string, header http.Header) {
						gomega.Expect(header["Content-Type"][0]).
							To(gomega.Equal("application/json"))
						var payload EventPayload
						err := json.Unmarshal([]byte(body), &payload)
						gomega.Expect(err).ToNot(gomega.HaveOccurred())
						gomega.Expect(payload.Contexts).To(gomega.HaveLen(2))
						gomega.Expect(
							payload.Contexts[0],
						).To(gomega.Equal(PagerDutyContext{Type: "link", Href: "http://example.com"}))
						gomega.Expect(
							payload.Contexts[1],
						).To(gomega.Equal(PagerDutyContext{Type: "image", Src: "http://example.com/img.png"}))
					}

					err := service.Send("test message", &types.Params{})
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				})
			})
		})

		ginkgo.Context("custom HTTP client", func() {
			ginkgo.BeforeEach(func() {
				serviceURL, err := url.Parse(
					fmt.Sprintf(
						"pagerduty://%s/%s",
						net.JoinHostPort(mockHost, mockPort),
						mockIntegrationKey,
					),
				)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				service = &Service{}
				err = service.Initialize(serviceURL, mockLogger)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				// Set a custom HTTP client with timeout, based on the mock client for TLS trust
				customClient := mockServer.Client()
				customClient.Timeout = 10 * time.Second
				service.SetHTTPClient(customClient)
			})

			ginkgo.When("using a custom HTTP client", func() {
				ginkgo.It("should use the custom client for requests", func() {
					checkRequest = func(_ string, header http.Header) {
						gomega.Expect(header["Content-Type"][0]).
							To(gomega.Equal("application/json"))
					}

					err := service.Send("test message", &types.Params{})
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				})
			})
		})
	})

	ginkgo.Context("with query parameters", func() {
		ginkgo.BeforeEach(func() {
			// Initialize service
			serviceURL, err := url.Parse(
				fmt.Sprintf(
					`pagerduty://%s/%s?severity=critical&source=beszel&action=resolve`,
					net.JoinHostPort(mockHost, mockPort),
					mockIntegrationKey,
				),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			service = &Service{}
			err = service.Initialize(serviceURL, mockLogger)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			service.SetHTTPClient(mockHTTPClient)
		})

		ginkgo.When("sending a simple alert", func() {
			ginkgo.It(
				"should send a request to our mock PagerDuty server with all fields populated from query parameters",
				func() {
					checkRequest = func(body string, header http.Header) {
						gomega.Expect(header["Content-Type"][0]).
							To(gomega.Equal("application/json"))
						gomega.Expect(body).To(gomega.Equal(`{` +
							`"payload":{` +
							`"summary":"An example alert message",` +
							`"severity":"critical",` +
							`"source":"beszel"` +
							`},` +
							`"routing_key":"a1b2c3d4e5f678901234567890abcdef",` +
							`"event_action":"resolve"` +
							`}`))
					}

					err := service.Send("An example alert message", &types.Params{})
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
				},
			)
		})
	})
})

var _ = ginkgo.Describe("parseContexts function", func() {
	ginkgo.When("parsing valid context strings", func() {
		ginkgo.It("should parse link contexts correctly", func() {
			contexts, err := parseContexts("link:http://example.com")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(contexts).To(gomega.HaveLen(1))
			gomega.Expect(
				contexts[0],
			).To(gomega.Equal(PagerDutyContext{Type: "link", Href: "http://example.com"}))
		})

		ginkgo.It("should parse image contexts correctly", func() {
			contexts, err := parseContexts("image:http://example.com/img.png")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(contexts).To(gomega.HaveLen(1))
			gomega.Expect(
				contexts[0],
			).To(gomega.Equal(PagerDutyContext{Type: "image", Src: "http://example.com/img.png"}))
		})

		ginkgo.It("should skip text contexts", func() {
			contexts, err := parseContexts("text:Some description")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(contexts).To(gomega.BeEmpty())
		})

		ginkgo.It("should parse multiple contexts correctly", func() {
			contexts, err := parseContexts(
				"link:http://example.com,text:Additional context,image:http://example.com/img.png",
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(contexts).To(gomega.HaveLen(2))
			gomega.Expect(
				contexts[0],
			).To(gomega.Equal(PagerDutyContext{Type: "link", Href: "http://example.com"}))
			gomega.Expect(
				contexts[1],
			).To(gomega.Equal(PagerDutyContext{Type: "image", Src: "http://example.com/img.png"}))
		})

		ginkgo.It("should handle empty string", func() {
			contexts, err := parseContexts("")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(contexts).To(gomega.BeNil())
		})

		ginkgo.It("should skip empty contexts", func() {
			contexts, err := parseContexts("link:http://example.com,,text:Some text")
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(contexts).To(gomega.HaveLen(1))
			gomega.Expect(
				contexts[0],
			).To(gomega.Equal(PagerDutyContext{Type: "link", Href: "http://example.com"}))
		})
	})

	ginkgo.When("parsing invalid context strings", func() {
		ginkgo.It("should return error for invalid format", func() {
			_, err := parseContexts("invalid")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid context format"))
		})

		ginkgo.It("should return error for empty type", func() {
			_, err := parseContexts(":value")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("type and value cannot be empty"))
		})

		ginkgo.It("should return error for empty value", func() {
			_, err := parseContexts("type:")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("type and value cannot be empty"))
		})

		ginkgo.It("should return error for missing colon", func() {
			_, err := parseContexts("typevalue")
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid context format"))
		})
	})
})

var _ = ginkgo.Describe("the PagerDuty Config struct", func() {
	ginkgo.When("generating a Config from a simple URL", func() {
		ginkgo.It("should populate the Config with host and integration key", func() {
			url, err := url.Parse(fmt.Sprintf("pagerduty://%s/%s", mockHost, mockIntegrationKey))
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			config := Config{}
			err = config.SetURL(url)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Expect(config.IntegrationKey).To(gomega.Equal(mockIntegrationKey))
			gomega.Expect(config.Host).To(gomega.Equal(mockHost))
		})
	})

	ginkgo.When("generating a Config from a url with port", func() {
		ginkgo.It("should populate the port field", func() {
			url, err := url.Parse(
				fmt.Sprintf(
					"pagerduty://%s/%s",
					net.JoinHostPort(mockHost, "12345"),
					mockIntegrationKey,
				),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			config := Config{}
			err = config.SetURL(url)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Expect(config.Port).To(gomega.Equal(uint16(12345)))
		})
	})

	ginkgo.When("generating a Config from a url with query parameters", func() {
		ginkgo.It("should populate the Config fields with the query parameter values", func() {
			queryParams := `severity=critical&source=my-app&action=trigger`
			url, err := url.Parse(
				fmt.Sprintf(
					"pagerduty://%s/%s?%s",
					net.JoinHostPort(mockHost, "12345"),
					mockIntegrationKey,
					queryParams,
				),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			config := Config{}
			err = config.SetURL(url)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Expect(config.Severity).To(gomega.Equal("critical"))
			gomega.Expect(config.Source).To(gomega.Equal("my-app"))
			gomega.Expect(config.Action).To(gomega.Equal("trigger"))
		})
	})

	ginkgo.When("generating a Config from a url with differently escaped spaces", func() {
		ginkgo.It("should parse the escaped spaces correctly", func() {
			// Use: '%20', '+' and a normal space
			queryParams := `source=my app`
			url, err := url.Parse(
				fmt.Sprintf(
					"pagerduty://%s/%s?%s",
					net.JoinHostPort(mockHost, "12345"),
					mockIntegrationKey,
					queryParams,
				),
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			config := Config{}
			err = config.SetURL(url)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			gomega.Expect(config.Source).To(gomega.Equal("my app"))
		})
	})
})
