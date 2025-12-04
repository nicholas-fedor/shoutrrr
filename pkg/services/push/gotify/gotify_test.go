package gotify

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Test constants.
// These constants define test URLs and endpoints used throughout the test suite
// for mocking Gotify API interactions and verifying URL construction.
const (
	TargetURL = "https://my.gotify.tld/message?token=Aaa.bbb.ccc.ddd" // Standard test URL with token in query parameter
)

// TestGotify runs the Ginkgo test suite for the Gotify package.
func TestGotify(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Gotify Suite")
}

// Test suite global variables and setup.
// These variables maintain state across test cases and provide shared test infrastructure.
// The BeforeSuite block initializes common test resources used throughout the test suite.
var (
	service      *Service    // Global service instance for testing
	logger       *log.Logger // Test logger for capturing service output
	envGotifyURL *url.URL    // Environment-provided Gotify URL for integration tests
	_            = ginkgo.BeforeSuite(func() {
		service = &Service{} // Initialize fresh service instance
		logger = log.New(
			ginkgo.GinkgoWriter,
			"Test",
			log.LstdFlags,
		) // Create logger that writes to Ginkgo output
		var err error
		envGotifyURL, err = url.Parse(
			os.Getenv("SHOUTRRR_GOTIFY_URL"),
		) // Parse integration test URL from environment
		if err != nil {
			envGotifyURL = &url.URL{} // Default to empty URL if parsing fails
		}
	})
)

// Main test suite for Gotify service functionality.
// This comprehensive test suite covers all aspects of the Gotify service including
// configuration parsing, URL construction, token validation, HTTP communication,
// error handling, and various authentication methods.
var _ = ginkgo.Describe("the Gotify service", func() {
	ginkgo.When("running integration tests", func() {
		ginkgo.It("sends a message successfully with a valid ENV URL", func() {
			if envGotifyURL.String() == "" {
				ginkgo.Skip("No integration test ENV URL was set")

				return
			}
			serviceURL := testutils.URLMust(envGotifyURL.String())
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Send("This is an integration test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("the service", func() {
		ginkgo.BeforeEach(func() {
			service = &Service{}
			service.SetLogger(logger)
		})
		ginkgo.It("returns the correct service identifier", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal("gotify"))
		})
	})

	ginkgo.When("parsing the configuration URL", func() {
		ginkgo.BeforeEach(func() {
			service = &Service{}
			service.SetLogger(logger)
		})
		ginkgo.It("builds a valid Gotify URL without path", func() {
			configURL := testutils.URLMust("gotify://test.tld/Aaa.bbb.ccc.ddd")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.GetURL().String()).To(gomega.Equal(configURL.String()))
		})
		ginkgo.When("TLS is disabled", func() {
			ginkgo.It("uses http scheme", func() {
				configURL := testutils.URLMust(
					"gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?disabletls=yes",
				)
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(service.Config.DisableTLS).To(gomega.BeTrue())
			})
		})
		ginkgo.When("a custom path is provided", func() {
			ginkgo.It("includes the path in the URL", func() {
				configURL := testutils.URLMust("gotify://my.gotify.tld/gotify/Aaa.bbb.ccc.ddd")
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(service.Config.GetURL().String()).To(gomega.Equal(configURL.String()))
			})
		})
		ginkgo.When("the token has an invalid length", func() {
			ginkgo.It("reports an error during send", func() {
				configURL := testutils.URLMust("gotify://my.gotify.tld/short") // Length < 15
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = service.Send("Message", nil)
				gomega.Expect(err).To(gomega.MatchError("invalid gotify token: \"short\""))
			})
		})
		ginkgo.When("the token has an invalid prefix", func() {
			ginkgo.It("reports an error during send", func() {
				configURL := testutils.URLMust(
					"gotify://my.gotify.tld/Chwbsdyhwwgarxd",
				) // Starts with 'C', not 'A'
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = service.Send("Message", nil)
				gomega.Expect(err).
					To(gomega.MatchError("invalid gotify token: \"Chwbsdyhwwgarxd\""))
			})
		})
		ginkgo.It("is identical after de-/serialization with path", func() {
			testURL := "gotify://my.gotify.tld/gotify/Aaa.bbb.ccc.ddd?title=Test+title"
			serviceURL := testutils.URLMust(testURL)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.GetURL().String()).To(gomega.Equal(testURL))
		})
		ginkgo.It("is identical after de-/serialization without path", func() {
			testURL := "gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?disabletls=Yes&priority=1&title=Test+title"
			serviceURL := testutils.URLMust(testURL)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.GetURL().String()).To(gomega.Equal(testURL))
		})
		ginkgo.It("allows slash at the end of the token", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.ddd/")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Token).To(gomega.Equal("Aaa.bbb.ccc.ddd"))
		})
		ginkgo.It("allows slash at the end of the token with additional path", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld/path/to/gotify/Aaa.bbb.ccc.ddd/")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Token).To(gomega.Equal("Aaa.bbb.ccc.ddd"))
		})
		ginkgo.It("does not crash on empty token or path slash", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld//")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Token).To(gomega.Equal(""))
		})
		ginkgo.When("handling malformed inputs", func() {
			ginkgo.BeforeEach(func() {
				service = &Service{}
				service.SetLogger(logger)
			})
			ginkgo.It("handles invalid URL schemes gracefully", func() {
				invalidURL, err := url.Parse("invalid://url")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = service.Initialize(invalidURL, logger)
				// Service may accept invalid schemes, but Send should fail
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
			ginkgo.It("handles URLs with invalid host", func() {
				invalidURL, err := url.Parse("gotify://[::1]:99999/Aaa.bbb.ccc.ddd")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = service.Initialize(invalidURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Send should fail due to invalid host
				err = service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
			ginkgo.It("handles malformed JSON in extras parameter", func() {
				configURL := testutils.URLMust(
					"gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?extras={invalid}",
				)
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("parsing extras JSON"))
			})
			ginkgo.It("handles empty host in URL", func() {
				configURL := testutils.URLMust("gotify:///Aaa.bbb.ccc.ddd")
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(service.Config.Host).To(gomega.Equal(""))
			})
			ginkgo.It("handles URL with only host and no token", func() {
				configURL := testutils.URLMust("gotify://my.gotify.tld/")
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(service.Config.Token).To(gomega.Equal(""))
			})
			ginkgo.It("handles extreme priority values", func() {
				configURL := testutils.URLMust("gotify://test.tld/Aaa.bbb.ccc.ddd")
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				params := types.Params{"priority": "-100"}
				err = service.Send("Message", &params)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err).To(gomega.MatchError("priority must be between -2 and 10"))
			})
			ginkgo.It("handles nil config gracefully", func() {
				service.Config = nil
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
			ginkgo.It("handles empty config extras", func() {
				configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.ddd")
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				service.Config.Extras = map[string]any{}
				httpmock.ActivateNonDefault(service.GetHTTPClient())
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						gomega.Expect(requestBody["extras"]).To(gomega.Equal(map[string]any{}))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  "Message",
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				err = service.Send("Message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				httpmock.DeactivateAndReset()
			})
		})
		ginkgo.It("parses valid extras JSON from URL parameters", func() {
			extrasJSON := `{"key1":"value1","key2":42}`
			configURL := testutils.URLMust(
				"gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?extras=" + url.QueryEscape(extrasJSON),
			)
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Extras).To(gomega.Equal(map[string]any{
				"key1": "value1",
				"key2": float64(42),
			}))
		})
		ginkgo.It("reports error on invalid extras JSON from URL parameters", func() {
			configURL := testutils.URLMust(
				"gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?extras=invalid-json",
			)
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("parsing extras JSON from URL query"))
		})
		ginkgo.It("handles empty extras JSON from URL parameters", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?extras=")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Extras).To(gomega.BeNil())
		})
		ginkgo.It("parses useheader parameter from URL", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?useheader=yes")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.UseHeader).To(gomega.BeTrue())
		})
		ginkgo.It("handles malformed URLs gracefully", func() {
			// Test with URL that has invalid extras JSON
			invalidURL := testutils.URLMust(
				"gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?extras=invalid-json",
			)
			err := service.Initialize(invalidURL, logger)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("builds URL without token when useheader is enabled", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?useheader=yes")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			builtURL, err := buildURL(service.Config)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(builtURL).To(gomega.Equal("https://my.gotify.tld/message"))
			gomega.Expect(builtURL).NotTo(gomega.ContainSubstring("token="))
		})
	})

	ginkgo.When("the token contains invalid characters", func() {
		ginkgo.It("reports an error during send", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.dd!")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.MatchError("invalid gotify token: \"Aaa.bbb.ccc.dd!\""))
		})
	})
	ginkgo.When("the token has exactly 15 chars but invalid prefix", func() {
		ginkgo.It("reports an error during send", func() {
			configURL := testutils.URLMust(
				"gotify://my.gotify.tld/Baa.bbb.ccc.ddd",
			) // Starts with 'B', not 'A'
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.MatchError("invalid gotify token: \"Baa.bbb.ccc.ddd\""))
		})
	})
	ginkgo.When("the token has valid prefix but invalid characters at different positions", func() {
		ginkgo.It("reports an error for invalid char at position 5", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa!bbb.ccc.ddd")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.MatchError("invalid gotify token: \"Aaa!bbb.ccc.ddd\""))
		})
		ginkgo.It("reports an error for invalid char at position 10", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb!ccc.ddd")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.MatchError("invalid gotify token: \"Aaa.bbb!ccc.ddd\""))
		})
		ginkgo.It("reports an error for invalid char at position 15", func() {
			configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.dd!")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.MatchError("invalid gotify token: \"Aaa.bbb.ccc.dd!\""))
		})
	})

	ginkgo.Describe("sending the payload", func() {
		ginkgo.BeforeEach(func() {
			service = &Service{}
			service.SetLogger(logger)
			configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.ddd")
			err := service.Initialize(configURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			httpmock.ActivateNonDefault(service.GetHTTPClient())
		})
		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})
		ginkgo.When("sending via webhook URL", func() {
			ginkgo.It("does not report an error if the server accepts the payload", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(200, map[string]any{
						"id":       float64(1),
						"appid":    float64(1),
						"message":  "Message",
						"title":    "Shoutrrr notification",
						"priority": float64(0),
						"date":     "2023-01-01T00:00:00Z",
					}),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It(
				"reports an error if the server rejects the payload with an error response",
				func() {
					httpmock.RegisterResponder(
						"POST",
						TargetURL,
						testutils.JSONRespondMust(401, map[string]any{
							"error":            "Unauthorized",
							"errorCode":        float64(401),
							"errorDescription": "you need to provide a valid access token or user credentials to access this api",
						}),
					)
					err := service.Send("Message", nil)
					gomega.Expect(err).
						To(gomega.MatchError("server responded with Unauthorized (401): you need to provide a valid access token or user credentials to access this api"))
				},
			)
			ginkgo.It("reports an error for 403 Forbidden response", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(403, map[string]any{
						"error":            "Forbidden",
						"errorCode":        float64(403),
						"errorDescription": "access denied",
					}),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("server responded with Forbidden (403)"))
			})
			ginkgo.It("reports an error for 404 Not Found response", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(404, map[string]any{
						"error":            "Not Found",
						"errorCode":        float64(404),
						"errorDescription": "endpoint not found",
					}),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("server responded with Not Found (404)"))
			})
			ginkgo.It("reports an error for 429 Too Many Requests response", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(429, map[string]any{
						"error":            "Too Many Requests",
						"errorCode":        float64(429),
						"errorDescription": "rate limit exceeded",
					}),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("server responded with Too Many Requests (429)"))
			})
			ginkgo.It("reports an error for 500 Internal Server Error response", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(500, map[string]any{
						"error":            "Internal Server Error",
						"errorCode":        float64(500),
						"errorDescription": "internal error",
					}),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("server responded with Internal Server Error (500)"))
			})
			ginkgo.It("reports an error for 502 Bad Gateway response", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(502, map[string]any{
						"error":            "Bad Gateway",
						"errorCode":        float64(502),
						"errorDescription": "bad gateway",
					}),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("server responded with Bad Gateway (502)"))
			})
			ginkgo.It("reports an error for 503 Service Unavailable response", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(503, map[string]any{
						"error":            "Service Unavailable",
						"errorCode":        float64(503),
						"errorDescription": "service unavailable",
					}),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("server responded with Service Unavailable (503)"))
			})
			ginkgo.It("reports an error for 504 Gateway Timeout response", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(504, map[string]any{
						"error":            "Gateway Timeout",
						"errorCode":        float64(504),
						"errorDescription": "gateway timeout",
					}),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("server responded with Gateway Timeout (504)"))
			})
			ginkgo.It("reports an error if sending fails with a network error", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					httpmock.NewErrorResponder(errors.New("network failure")),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).
					To(gomega.MatchError("failed to send notification to Gotify: sending POST request to \"https://my.gotify.tld/message?token=Aaa.bbb.ccc.ddd\": Post \"https://my.gotify.tld/message?token=Aaa.bbb.ccc.ddd\": network failure"))
			})
			ginkgo.It("returns an error if params update fails", func() {
				params := types.Params{"priority": "invalid"}
				err := service.Send("Message", &params)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("failed to update config from params"))
			})
			ginkgo.It("returns an error if message is empty", func() {
				err := service.Send("", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.Equal("message cannot be empty"))
			})
			ginkgo.It("returns an error if priority is too low", func() {
				params := types.Params{"priority": "-3"}
				err := service.Send("Message", &params)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err).To(gomega.MatchError("priority must be between -2 and 10"))
			})
			ginkgo.It("returns an error if priority is too high", func() {
				params := types.Params{"priority": "11"}
				err := service.Send("Message", &params)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err).To(gomega.MatchError("priority must be between -2 and 10"))
			})
			ginkgo.It("recreates HTTP client when service.httpClient is nil", func() {
				// Set httpClient to nil to simulate recreation
				service.httpClient = nil
				service.client = nil
				// Call Send to trigger recreation (it will fail due to no httpmock)
				_ = service.Send("Message", nil)
				// Now activate httpmock on the recreated client
				httpmock.ActivateNonDefault(service.httpClient)
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(200, map[string]any{
						"id":       float64(1),
						"appid":    float64(1),
						"message":  "Message",
						"title":    "Shoutrrr notification",
						"priority": float64(0),
						"date":     "2023-01-01T00:00:00Z",
					}),
				)
				// Verify client was recreated
				gomega.Expect(service.httpClient).NotTo(gomega.BeNil())
				gomega.Expect(service.client).NotTo(gomega.BeNil())
				// Now call Send again with httpmock active
				err := service.Send("Message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("handles very long messages", func() {
				longMessage := strings.Repeat("a", 10000)
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						gomega.Expect(requestBody["message"]).To(gomega.Equal(longMessage))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  longMessage,
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				err := service.Send(longMessage, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("handles messages with special characters", func() {
				specialMessage := "Message with special chars: éñüñ 中文 🚀 \n\t\"quotes\" 'single'"
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						gomega.Expect(requestBody["message"]).To(gomega.Equal(specialMessage))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  specialMessage,
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				err := service.Send(specialMessage, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("handles non-JSON error responses", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					httpmock.NewStringResponder(500, "Internal Server Error"),
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("failed to send notification to Gotify"))
			})
			ginkgo.It("handles concurrent access from multiple goroutines", func() {
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					testutils.JSONRespondMust(200, map[string]any{
						"id":       float64(1),
						"appid":    float64(1),
						"message":  "Message",
						"title":    "Shoutrrr notification",
						"priority": float64(0),
						"date":     "2023-01-01T00:00:00Z",
					}),
				)
				numGoroutines := 10
				errChan := make(chan error, numGoroutines)
				for i := range numGoroutines {
					go func(id int) {
						err := service.Send("Concurrent message "+strconv.Itoa(id), nil)
						errChan <- err
					}(i)
				}
				for range numGoroutines {
					err := <-errChan
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}
			})
			ginkgo.It("handles very large extras payload", func() {
				// Create a large extras payload
				largeExtras := make(map[string]any)
				for i := range 1000 {
					largeExtras[strconv.Itoa(i)] = strings.Repeat(
						"data",
						100,
					) // ~400 bytes per entry
				}
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						gomega.Expect(requestBody["extras"]).To(gomega.Equal(largeExtras))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  "Message",
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				service.Config.Extras = largeExtras
				err := service.Send("Message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("handles combined large message and extras", func() {
				largeMessage := strings.Repeat("message", 2000) // ~12KB message
				largeExtras := make(map[string]any)
				for i := range 500 {
					largeExtras[strconv.Itoa(i)] = strings.Repeat(
						"extra",
						50,
					) // ~250 bytes per entry
				}
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						gomega.Expect(requestBody["message"]).To(gomega.Equal(largeMessage))
						gomega.Expect(requestBody["extras"]).To(gomega.Equal(largeExtras))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  largeMessage,
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				service.Config.Extras = largeExtras
				err := service.Send(largeMessage, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})
		ginkgo.When("sending with extras from params", func() {
			ginkgo.It("includes extras in request payload from params", func() {
				extrasJSON := `{"paramKey":"paramValue"}`
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						gomega.Expect(requestBody["extras"]).To(gomega.Equal(map[string]any{
							"paramKey": "paramValue",
						}))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  "Message",
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				params := types.Params{"extras": extrasJSON}
				err := service.Send("Message", &params)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("includes extras in request payload from config", func() {
				service.Config.Extras = map[string]any{"configKey": "configValue"}
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						gomega.Expect(requestBody["extras"]).To(gomega.Equal(map[string]any{
							"configKey": "configValue",
						}))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  "Message",
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("prioritizes extras from params over config", func() {
				service.Config.Extras = map[string]any{"configKey": "configValue"}
				extrasJSON := `{"paramKey":"paramValue"}`
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						gomega.Expect(requestBody["extras"]).To(gomega.Equal(map[string]any{
							"paramKey": "paramValue",
						}))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  "Message",
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				params := types.Params{"extras": extrasJSON}
				err := service.Send("Message", &params)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})
		ginkgo.When("sending with date parameter", func() {
			ginkgo.It("includes date in request payload when provided", func() {
				customDate := "2023-12-04T20:00:00Z"
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						gomega.Expect(requestBody["date"]).To(gomega.Equal(customDate))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  "Message",
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     customDate,
						})(req)
					},
				)
				params := types.Params{"date": customDate}
				err := service.Send("Message", &params)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})
		ginkgo.When("using header authentication", func() {
			ginkgo.BeforeEach(func() {
				service = &Service{}
				service.SetLogger(logger)
				configURL := testutils.URLMust(
					"gotify://my.gotify.tld/Aaa.bbb.ccc.ddd?useheader=yes",
				)
				err := service.Initialize(configURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				httpmock.ActivateNonDefault(service.GetHTTPClient())
			})
			ginkgo.AfterEach(func() {
				httpmock.DeactivateAndReset()
			})
			ginkgo.It("sends X-Gotify-Key header when useheader is enabled", func() {
				httpmock.RegisterResponder(
					"POST",
					"https://my.gotify.tld/message",
					func(req *http.Request) (*http.Response, error) {
						gomega.Expect(req.Header.Get("X-Gotify-Key")).
							To(gomega.Equal("Aaa.bbb.ccc.ddd"))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  "Message",
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("cleans up X-Gotify-Key header after send", func() {
				httpmock.RegisterResponder(
					"POST",
					"https://my.gotify.tld/message",
					func(req *http.Request) (*http.Response, error) {
						gomega.Expect(req.Header.Get("X-Gotify-Key")).
							To(gomega.Equal("Aaa.bbb.ccc.ddd"))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  "Message",
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				err := service.Send("Message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				// Verify header was cleaned up
				gomega.Expect(service.client.Headers().Get("X-Gotify-Key")).
					To(gomega.Equal(""))
			})
		})
		ginkgo.When("parsing extras with invalid JSON", func() {
			ginkgo.It("logs error and falls back to config extras", func() {
				service.Config.Extras = map[string]any{"configKey": "configValue"}
				httpmock.RegisterResponder(
					"POST",
					TargetURL,
					func(req *http.Request) (*http.Response, error) {
						var requestBody map[string]any
						if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
							return nil, err
						}
						// Should fall back to config extras since params extras are invalid
						gomega.Expect(requestBody["extras"]).To(gomega.Equal(map[string]any{
							"configKey": "configValue",
						}))

						return testutils.JSONRespondMust(200, map[string]any{
							"id":       float64(1),
							"appid":    float64(1),
							"message":  "Message",
							"title":    "Shoutrrr notification",
							"priority": float64(0),
							"date":     "2023-01-01T00:00:00Z",
						})(req)
					},
				)
				params := types.Params{"extras": "invalid-json"}
				err := service.Send("Message", &params)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})
		ginkgo.When("extras contain non-marshallable data", func() {
			ginkgo.It("fails with marshalling error", func() {
				service.Config.Extras = map[string]any{"bad": make(chan int)}
				err := service.Send("Message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("marshaling request to JSON"))
			})
		})
	})

	// Helper functions test suite.
	// This section tests the internal utility functions that support the main service operations,
	// including request preparation, configuration validation, URL building, and extras parsing.
	ginkgo.Describe("helper functions", func() {
		ginkgo.Describe("prepareRequest", func() {
			ginkgo.It("constructs payload correctly", func() {
				config := &Config{
					Title:    "Test Title",
					Priority: 5,
				}
				extras := map[string]any{"key": "value"}
				message := "Test message"
				date := "2023-01-01T00:00:00Z"

				request := service.prepareRequest(message, config, extras, &date)

				gomega.Expect(request.Message).To(gomega.Equal(message))
				gomega.Expect(request.Title).To(gomega.Equal(config.Title))
				gomega.Expect(request.Priority).To(gomega.Equal(config.Priority))
				gomega.Expect(request.Date).To(gomega.Equal(&date))
				gomega.Expect(request.Extras).To(gomega.Equal(extras))
			})
		})
		ginkgo.Describe("createTransport", func() {
			ginkgo.It("configures TLS correctly when TLS is disabled", func() {
				service.Config = &Config{}
				service.Config.DisableTLS = true
				transport := service.createTransport()
				gomega.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(gomega.BeTrue())
			})
			ginkgo.It("configures TLS correctly when TLS is enabled", func() {
				service.Config = &Config{}
				service.Config.DisableTLS = false
				transport := service.createTransport()
				gomega.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(gomega.BeFalse())
			})
		})
		ginkgo.Describe("createHTTPClient", func() {
			ginkgo.It("sets timeout and transport correctly", func() {
				transport := &http.Transport{}
				client := service.createHTTPClient(transport)
				gomega.Expect(client.Timeout).To(gomega.Equal(10 * time.Second))
				gomega.Expect(client.Transport).To(gomega.Equal(transport))
			})
		})
		ginkgo.Describe("parseExtras", func() {
			ginkgo.It("parses valid JSON extras from params", func() {
				params := &types.Params{"extras": `{"key":"value"}`}
				config := &Config{}
				result := service.parseExtras(params, config)
				gomega.Expect(result).To(gomega.Equal(map[string]any{"key": "value"}))
			})
			ginkgo.It("returns config extras when no params extras", func() {
				params := &types.Params{}
				config := &Config{Extras: map[string]any{"config": "value"}}
				result := service.parseExtras(params, config)
				gomega.Expect(result).To(gomega.Equal(map[string]any{"config": "value"}))
			})
		})
		ginkgo.Describe("validateToken", func() {
			ginkgo.It("returns true for valid token", func() {
				gomega.Expect(validateToken("Aaa.bbb.ccc.ddd")).To(gomega.BeTrue())
			})

			ginkgo.It("returns false for token too short", func() {
				gomega.Expect(validateToken("short")).To(gomega.BeFalse())
			})
			ginkgo.It("returns false for token not starting with A", func() {
				gomega.Expect(validateToken("Baa.bbb.ccc.ddd")).To(gomega.BeFalse())
			})
			ginkgo.It("returns false for invalid character at position 5", func() {
				gomega.Expect(validateToken("Aaa!bbb.ccc.ddd")).To(gomega.BeFalse())
			})
			ginkgo.It("returns false for invalid character at position 10", func() {
				gomega.Expect(validateToken("Aaa.bbb!ccc.ddd")).To(gomega.BeFalse())
			})
			ginkgo.It("returns false for invalid character at position 15", func() {
				gomega.Expect(validateToken("Aaa.bbb.ccc.dd!")).To(gomega.BeFalse())
			})
		})
		ginkgo.Describe("buildURL", func() {
			ginkgo.It("builds URL with token in query for useheader false", func() {
				config := &Config{
					Host:       "example.com",
					Path:       "",
					Token:      "Aaa.bbb.ccc.ddd",
					UseHeader:  false,
					DisableTLS: false,
				}
				url, err := buildURL(config)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(url).
					To(gomega.Equal("https://example.com/message?token=Aaa.bbb.ccc.ddd"))
			})
			ginkgo.It("builds URL without token for useheader true", func() {
				config := &Config{
					Host:       "example.com",
					Path:       "",
					Token:      "Aaa.bbb.ccc.ddd",
					UseHeader:  true,
					DisableTLS: false,
				}
				url, err := buildURL(config)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(url).To(gomega.Equal("https://example.com/message"))
			})
			ginkgo.It("uses http when DisableTLS true", func() {
				config := &Config{
					Host:       "example.com",
					Path:       "",
					Token:      "Aaa.bbb.ccc.ddd",
					UseHeader:  false,
					DisableTLS: true,
				}
				url, err := buildURL(config)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(url).
					To(gomega.Equal("http://example.com/message?token=Aaa.bbb.ccc.ddd"))
			})
			ginkgo.It("returns error for invalid token", func() {
				config := &Config{
					Host:       "example.com",
					Path:       "",
					Token:      "invalid",
					UseHeader:  false,
					DisableTLS: false,
				}
				_, err := buildURL(config)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid gotify token"))
			})
		})
	})
})

// TestSend tests basic message sending functionality.
func TestSend(t *testing.T) {
	gomega.RegisterTestingT(t)

	service := &Service{}
	logger := log.New(os.Stderr, "Test", log.LstdFlags)
	service.SetLogger(logger)

	configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.ddd")
	err := service.Initialize(configURL, logger)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	httpmock.ActivateNonDefault(service.GetHTTPClient())
	httpmock.RegisterResponder(
		"POST",
		TargetURL,
		testutils.JSONRespondMust(200, map[string]any{
			"id":       float64(1),
			"appid":    float64(1),
			"message":  "Message",
			"title":    "Shoutrrr notification",
			"priority": float64(0),
			"date":     "2023-01-01T00:00:00Z",
		}),
	)

	err = service.Send("Message", nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	httpmock.DeactivateAndReset()
}

// TestSendWithPriority tests sending a message with a custom priority.
func TestSendWithPriority(t *testing.T) {
	gomega.RegisterTestingT(t)

	service := &Service{}
	logger := log.New(os.Stderr, "Test", log.LstdFlags)
	service.SetLogger(logger)

	configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.ddd")
	err := service.Initialize(configURL, logger)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	httpmock.ActivateNonDefault(service.GetHTTPClient())
	httpmock.RegisterResponder(
		"POST",
		TargetURL,
		func(req *http.Request) (*http.Response, error) {
			var requestBody map[string]any
			if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
				return nil, err
			}

			gomega.Expect(requestBody["priority"]).To(gomega.Equal(float64(5)))

			return testutils.JSONRespondMust(200, map[string]any{
				"id":       float64(1),
				"appid":    float64(1),
				"message":  "Message",
				"title":    "Shoutrrr notification",
				"priority": float64(5),
				"date":     "2023-01-01T00:00:00Z",
			})(req)
		},
	)

	params := types.Params{"priority": "5"}
	err = service.Send("Message", &params)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	httpmock.DeactivateAndReset()
}

// TestSendWithTitle tests sending a message with a custom title.
func TestSendWithTitle(t *testing.T) {
	gomega.RegisterTestingT(t)

	service := &Service{}
	logger := log.New(os.Stderr, "Test", log.LstdFlags)
	service.SetLogger(logger)

	configURL := testutils.URLMust("gotify://my.gotify.tld/Aaa.bbb.ccc.ddd")
	err := service.Initialize(configURL, logger)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	httpmock.ActivateNonDefault(service.GetHTTPClient())
	httpmock.RegisterResponder(
		"POST",
		TargetURL,
		func(req *http.Request) (*http.Response, error) {
			var requestBody map[string]any
			if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
				return nil, err
			}

			gomega.Expect(requestBody["title"]).To(gomega.Equal("Custom Title"))

			return testutils.JSONRespondMust(200, map[string]any{
				"id":       float64(1),
				"appid":    float64(1),
				"message":  "Message",
				"title":    "Custom Title",
				"priority": float64(0),
				"date":     "2023-01-01T00:00:00Z",
			})(req)
		},
	)

	params := types.Params{"title": "Custom Title"}
	err = service.Send("Message", &params)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	httpmock.DeactivateAndReset()
}

// TestTimeout tests timeout handling using synctest for instant execution.

// TestTimeout tests timeout handling using synctest for instant execution.
func TestTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)
		// Create fake network connection
		srvConn, cliConn := net.Pipe()
		defer srvConn.Close()
		defer cliConn.Close()

		errChan := make(chan error, 1)

		// Create service with custom HTTP client using fake connection
		service := &Service{}
		logger := log.New(os.Stderr, "Test", log.LstdFlags)
		service.SetLogger(logger)

		configURL := testutils.URLMust("gotify://test.tld/Aaa.bbb.ccc.ddd")
		err := service.Initialize(configURL, logger)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		// Override the HTTP client to use fake connection
		service.httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return cliConn, nil
				},
			},
			Timeout: 10 * time.Second,
		}
		service.client = jsonclient.NewWithHTTPClient(service.httpClient)

		// Simulate slow server in goroutine
		go func() {
			req, err := http.ReadRequest(bufio.NewReader(srvConn))
			if err != nil {
				return
			}
			// Simulate slow response
			time.Sleep(15 * time.Second)

			resp, err := testutils.JSONRespondMust(200, map[string]any{
				"id":       float64(1),
				"appid":    float64(1),
				"message":  "Message",
				"title":    "Shoutrrr notification",
				"priority": float64(0),
				"date":     "2023-01-01T00:00:00Z",
			})(req)
			if err != nil {
				return
			}

			err = resp.Write(srvConn)
			if err != nil {
				errChan <- err

				return
			}

			resp.Body.Close()
		}()

		// Send request and expect timeout
		err = service.Send("Message", nil)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("context deadline exceeded"))

		select {
		case writeErr := <-errChan:
			t.Fatalf("failed to write response: %v", writeErr)
		default:
		}
	})
}
