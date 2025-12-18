package notifiarr_test

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/specialized/notifiarr"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestNotifiarr runs the Notifiarr service test suite using Ginkgo.
func TestNotifiarr(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Notifiarr Suite")
}

var (
	service         *notifiarr.Service
	envNotifiarrURL *url.URL
	logger          *log.Logger
	_               = ginkgo.BeforeSuite(func() {
		service = &notifiarr.Service{}
		envNotifiarrURL, _ = url.Parse(os.Getenv("SHOUTRRR_NOTIFIARR_URL"))
		logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)
	})
)

var _ = ginkgo.Describe("the notifiarr service", func() {
	ginkgo.When("running integration tests", func() {
		ginkgo.It("should work without errors", func() {
			if envNotifiarrURL.String() == "" {
				ginkgo.Skip("No integration test ENV URL was set")

				return
			}

			serviceURL, _ := url.Parse(envNotifiarrURL.String())
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("this is an integration test", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("the service", func() {
		ginkgo.BeforeEach(func() {
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.It("returns the correct service identifier", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal("notifiarr"))
		})
	})

	ginkgo.When("parsing a custom URL", func() {
		ginkgo.BeforeEach(func() {
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.It("correctly sets API key from custom URL", func() {
			customURL := testutils.URLMust("notifiarr://apikey123")
			serviceURL, err := service.GetConfigURLFromCustom(customURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.APIKey).To(gomega.Equal("apikey123"))
		})
	})

	ginkgo.Describe("the notifiarr config", func() {
		ginkgo.When("parsing the configuration URL", func() {
			ginkgo.It("should be identical after de-/serialization", func() {
				testURL := "notifiarr://apikey123?channel=123456789"
				expectedURL := "notifiarr://apikey123?channel=123456789"

				serviceURL := testutils.URLMust(testURL)
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(service.Config.GetURL().String()).To(gomega.Equal(expectedURL))
			})
		})

		ginkgo.When("parsing from webhook URL", func() {
			ginkgo.It("sets config properties from webhook URL query parameters", func() {
				webhookURL := testutils.URLMust(
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123?channel=123456789",
				)

				config, _, err := notifiarr.ConfigFromWebhookURL(*webhookURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				gomega.Expect(config.APIKey).To(gomega.Equal("apikey123"))
				gomega.Expect(config.Channel).To(gomega.Equal("123456789"))
			})

			ginkgo.It(
				"sets config properties correctly from webhook URL with template and other parameters",
				func() {
					webhookURL := testutils.URLMust(
						"https://notifiarr.com/api/v1/notification/watchtower?template=json&contenttype=application/json&method=POST&titlekey=customtitle&messagekey=custommessage&extra=param",
					)

					config, _, err := notifiarr.ConfigFromWebhookURL(*webhookURL)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())

					gomega.Expect(config.APIKey).To(gomega.Equal(""))
					gomega.Expect(config.Channel).To(gomega.Equal(""))
					gomega.Expect(config.GetURL().RawQuery).
						To(gomega.Equal("contenttype=application%2Fjson&extra=param&messagekey=custommessage&method=POST&template=json&titlekey=customtitle"))
				},
			)

			ginkgo.It("handles custom headers and extra data from webhook URL", func() {
				webhookURL := testutils.URLMust(
					"https://example.com/webhook?@Authorization=Bearer token&$extraKey=extraValue&template=json",
				)

				config, _, err := notifiarr.ConfigFromWebhookURL(*webhookURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				gomega.Expect(config.APIKey).To(gomega.Equal(""))
				gomega.Expect(config.Channel).To(gomega.Equal(""))
				gomega.Expect(config.GetURL().RawQuery).
					To(gomega.Equal("%24extraKey=extraValue&%40Authorization=Bearer+token&template=json"))
			})
		})
	})

	ginkgo.Describe("sending the payload", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.When("sending via API", func() {
			ginkgo.It("succeeds if the server accepts the payload", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					httpmock.NewStringResponder(200, ""),
				)

				err = service.Send("Test message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("reports an error if sending fails", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					httpmock.NewErrorResponder(http.ErrHandlerTimeout),
				)

				err = service.Send("Test message", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})

			ginkgo.It("includes Discord channel in JSON payload when configured", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							To(gomega.ContainSubstring(`"ids":{"channel":123456789}`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				params := types.Params{"title": "Test Title"}
				err = service.Send("Test message", &params)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("includes title in notification payload", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							To(gomega.ContainSubstring(`"discord":{"text":{"title":"Test Title","description":"Test message"}}`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				params := types.Params{"title": "Test Title"}
				err = service.Send("Test message", &params)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("does not mutate the given params", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					httpmock.NewStringResponder(200, ""),
				)

				params := types.Params{"title": "ORIGINAL TITLE"}
				err = service.Send("Test message", &params)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(params).To(gomega.Equal(types.Params{"title": "ORIGINAL TITLE"}))
			})

			ginkgo.It("returns error for empty message", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				err = service.Send("", nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err).To(gomega.Equal(notifiarr.ErrEmptyMessage))
			})
		})
	})

	ginkgo.Describe("event ID parameter handling", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("includes event ID in payload when provided", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						To(gomega.ContainSubstring(`"event":"event-123"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{"id": "event-123"}
			err = service.Send("Test message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("omits event ID when not provided", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						NotTo(gomega.ContainSubstring(`"id"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("handles empty event ID gracefully", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						NotTo(gomega.ContainSubstring(`"id"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{"id": ""}
			err = service.Send("Test message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("Discord mention parsing", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.Describe("parsing user mentions", func() {
			ginkgo.It("parses single user mention <@123>", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							To(gomega.ContainSubstring(`"ping":{"pingUser":123}`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("Hello <@123>!", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("parses user mention with nickname <@!456>", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							To(gomega.ContainSubstring(`"ping":{"pingUser":456}`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("Hey <@!456> there", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("parses multiple user mentions", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							To(gomega.ContainSubstring(`"ping":{"pingUser":123}`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("<@123> and <@!456> are here", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.Describe("parsing role mentions", func() {
			ginkgo.It("parses single role mention <@&789>", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							To(gomega.ContainSubstring(`"ping":{"pingRole":789}`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("Role: <@&789>", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("parses multiple role mentions", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							To(gomega.ContainSubstring(`"ping":{"pingRole":111}`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("<@&111> and <@&222> notified", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.Describe("parsing mixed mentions", func() {
			ginkgo.It("parses user and role mentions together", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							To(gomega.ContainSubstring(`"ping":{"pingUser":123,"pingRole":456}`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("<@123> <@&456> <@!789>", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.Describe("invalid mentions", func() {
			ginkgo.It("ignores malformed mentions", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							NotTo(gomega.ContainSubstring(`"ping"`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("Invalid <@abc> <@&> <@123def>", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("ignores mentions without closing bracket", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							NotTo(gomega.ContainSubstring(`"ping"`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("Broken <@123 mention", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("ignores mentions with invalid characters", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							NotTo(gomega.ContainSubstring(`"ping"`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("<@12a3> <@&45b6>", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.Describe("edge cases", func() {
			ginkgo.It("handles message without mentions", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							NotTo(gomega.ContainSubstring(`"ping"`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("Just a regular message", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("handles mentions at message boundaries", func() {
				serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
				err := service.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				httpmock.RegisterResponder(
					"POST",
					"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
					func(req *http.Request) (*http.Response, error) {
						body, _ := io.ReadAll(req.Body)
						gomega.Expect(string(body)).
							To(gomega.ContainSubstring(`"ping":{"pingUser":123}`))

						return httpmock.NewStringResponse(200, ""), nil
					},
				)

				err = service.Send("<@123>", nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})
	})

	ginkgo.Describe("image/thumbnail URL parameters", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("includes thumbnail URL in payload when configured", func() {
			serviceURL := testutils.URLMust(
				"notifiarr://apikey123?thumbnail=https://example.com/image.png",
			)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						To(gomega.ContainSubstring(`"images":{"thumbnail":"https://example.com/image.png"}`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("omits thumbnail when not configured", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						NotTo(gomega.ContainSubstring(`"thumbnail"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("handles empty thumbnail URL gracefully", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123?thumbnail=")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						NotTo(gomega.ContainSubstring(`"thumbnail"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("includes image URL in payload when configured", func() {
			serviceURL := testutils.URLMust(
				"notifiarr://apikey123?image=https://example.com/image.png",
			)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						To(gomega.ContainSubstring(`"images":{"image":"https://example.com/image.png"}`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("omits image when not configured", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						NotTo(gomega.ContainSubstring(`"image"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("handles empty image URL gracefully", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123?image=")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						NotTo(gomega.ContainSubstring(`"image"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("includes both thumbnail and image in payload when both configured", func() {
			serviceURL := testutils.URLMust(
				"notifiarr://apikey123?thumbnail=https://example.com/thumb.png&image=https://example.com/img.png",
			)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						To(gomega.ContainSubstring(`"images":{"thumbnail":"https://example.com/thumb.png","image":"https://example.com/img.png"}`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("color customization parameters", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("includes color in payload when configured", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123?color=16711680")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						To(gomega.ContainSubstring(`"color":"16711680"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("includes hex color in payload", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123?color=%23FF0000")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						To(gomega.ContainSubstring(`"color":"#FF0000"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("omits color when not configured", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						NotTo(gomega.ContainSubstring(`"color"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("handles empty color value gracefully", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123?color=")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						NotTo(gomega.ContainSubstring(`"color"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("URL parsing of new parameters", func() {
		ginkgo.BeforeEach(func() {
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.It("parses all new parameters from URL", func() {
			serviceURL := testutils.URLMust(
				"notifiarr://apikey123?channel=123456789&thumbnail=https://example.com/img.png&color=blue",
			)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(service.Config.Channel).To(gomega.Equal("123456789"))
			gomega.Expect(service.Config.Thumbnail).To(gomega.Equal("https://example.com/img.png"))
			gomega.Expect(service.Config.Color).To(gomega.Equal("blue"))
		})

		ginkgo.It("handles URL-encoded values", func() {
			serviceURL := testutils.URLMust(
				"notifiarr://apikey123?thumbnail=https%3A//example.com/image%20with%20spaces.png&color=%23FF0000",
			)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(service.Config.Thumbnail).
				To(gomega.Equal("https://example.com/image with spaces.png"))
			gomega.Expect(service.Config.Color).To(gomega.Equal("#FF0000"))
		})

		ginkgo.It("preserves existing parameters while adding new ones", func() {
			serviceURL := testutils.URLMust(
				"notifiarr://apikey123?channel=123456789&thumbnail=https://example.com/img.png&color=blue",
			)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Test that existing channel parameter still works
			gomega.Expect(service.Config.Channel).To(gomega.Equal("123456789"))
			// Test new parameters
			gomega.Expect(service.Config.Thumbnail).To(gomega.Equal("https://example.com/img.png"))
			gomega.Expect(service.Config.Color).To(gomega.Equal("blue"))
		})
	})

	ginkgo.Describe("integration tests with all features combined", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("includes all features in payload when fully configured", func() {
			serviceURL := testutils.URLMust(
				"notifiarr://apikey123?channel=123456789&thumbnail=https://example.com/img.png&color=16711680",
			)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					bodyStr := string(body)

					// Check event ID
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"event":"test-event-123"`))
					// Check title and description
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"text":{"title":"Test Title","description":"Hello \u003c@123\u003e and \u003c@\u0026456\u003e!"}`))
					// Check Discord channel
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"ids":{"channel":123456789}`))
					// Check thumbnail
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"images":{"thumbnail":"https://example.com/img.png"}`))
					// Check color
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"color":"16711680"`))
					// Check mentions
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"ping":{"pingUser":123,"pingRole":456}`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{"id": "test-event-123", "title": "Test Title"}
			err = service.Send("Hello <@123> and <@&456>!", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("handles partial configuration gracefully", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123?channel=123456789")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					bodyStr := string(body)

					// Should have channel
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"ids":{"channel":123456789}`))
					// Should not have thumbnail or color
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"images"`))
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"color"`))
					// Should have mentions from message
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"ping":{"pingUser":123}`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{"id": "event-456"}
			err = service.Send("Message with <@123>", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("works with minimal configuration", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					bodyStr := string(body)

					// Should have basic fields
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"discord":{"text":{"title":"Test","description":"Simple message"}}`))
					// Should not have Discord section for channel, images, color, ping
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"ids"`))
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"images"`))
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"color"`))
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"ping"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{"title": "Test"}
			err = service.Send("Simple message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("advanced text fields (icon, content, footer)", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("includes icon in payload when provided", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						To(gomega.ContainSubstring(`"text":{"icon":"https://example.com/icon.png","description":"Test message"}`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{"icon": "https://example.com/icon.png"}
			err = service.Send("Test message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("includes content in payload when provided", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					gomega.Expect(string(body)).
						To(gomega.ContainSubstring(`"text":{"content":"Custom content","description":"Test message"}`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{"content": "Custom content"}
			err = service.Send("Test message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("includes footer in payload when provided", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					bodyStr := string(body)
					gomega.Expect(bodyStr).To(gomega.ContainSubstring(`"footer":"Footer text"`))
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"description":"Test message"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{"footer": "Footer text"}
			err = service.Send("Test message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("includes all advanced text fields when provided", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					bodyStr := string(body)
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"icon":"https://example.com/icon.png"`))
					gomega.Expect(bodyStr).To(gomega.ContainSubstring(`"content":"Custom content"`))
					gomega.Expect(bodyStr).To(gomega.ContainSubstring(`"footer":"Footer text"`))
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"description":"Test message"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{
				"icon":    "https://example.com/icon.png",
				"content": "Custom content",
				"footer":  "Footer text",
			}
			err = service.Send("Test message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("omits advanced text fields when not provided", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					bodyStr := string(body)
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"text":{"description":"Test message"}`))
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"icon"`))
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"content"`))
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"footer"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("combined usage of multiple new fields", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("includes all new fields in payload when fully configured", func() {
			serviceURL := testutils.URLMust(
				"notifiarr://apikey123?channel=123456789&thumbnail=https://example.com/thumb.png&image=https://example.com/img.png&color=16711680",
			)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					bodyStr := string(body)

					// Check event ID
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"event":"combined-test"`))
					// Check Discord channel
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"ids":{"channel":123456789}`))
					// Check images
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"images":{"thumbnail":"https://example.com/thumb.png","image":"https://example.com/img.png"}`))
					// Check color
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"color":"16711680"`))
					// Check all text fields
					gomega.Expect(bodyStr).To(gomega.ContainSubstring(`"title":"Combined Test"`))
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"icon":"https://example.com/icon.png"`))
					gomega.Expect(bodyStr).To(gomega.ContainSubstring(`"content":"Custom content"`))
					gomega.Expect(bodyStr).To(gomega.ContainSubstring(`"footer":"Footer info"`))
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"description":"Hello \u003c@123\u003e!"`))
					// Check mentions
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"ping":{"pingUser":123}`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{
				"id":      "combined-test",
				"title":   "Combined Test",
				"icon":    "https://example.com/icon.png",
				"content": "Custom content",
				"footer":  "Footer info",
			}
			err = service.Send("Hello <@123>!", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("handles mixed URL and parameter configuration", func() {
			serviceURL := testutils.URLMust(
				"notifiarr://apikey123?channel=123456789&thumbnail=https://example.com/thumb.png",
			)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					bodyStr := string(body)

					// URL parameters
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"ids":{"channel":123456789}`))
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"thumbnail":"https://example.com/thumb.png"`))
					// Parameter overrides
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"color":"blue"`))
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"image":"https://example.com/param-image.png"`))
					// Text fields from params
					gomega.Expect(bodyStr).To(gomega.ContainSubstring(`"title":"Param Title"`))
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"icon":"https://example.com/param-icon.png"`))
					gomega.Expect(bodyStr).To(gomega.ContainSubstring(`"content":"Param content"`))
					gomega.Expect(bodyStr).To(gomega.ContainSubstring(`"description":"Test"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{
				"color":   "blue",
				"image":   "https://example.com/param-image.png",
				"title":   "Param Title",
				"icon":    "https://example.com/param-icon.png",
				"content": "Param content",
			}
			err = service.Send("Test", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("works with partial new field combinations", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123?color=red")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				func(req *http.Request) (*http.Response, error) {
					body, _ := io.ReadAll(req.Body)
					bodyStr := string(body)

					// Color from URL
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"color":"red"`))
					// Some text fields from params
					gomega.Expect(bodyStr).
						To(gomega.ContainSubstring(`"text":{"title":"Partial","content":"Some content","description":"Message"}`))
					// No images or channel
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"ids"`))
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"images"`))
					// No footer or icon
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"footer"`))
					gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring(`"icon"`))

					return httpmock.NewStringResponse(200, ""), nil
				},
			)

			params := types.Params{
				"title":   "Partial",
				"content": "Some content",
			}
			err = service.Send("Message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("parsing functions", func() {
		ginkgo.Describe("ParseChannelID", func() {
			ginkgo.It("parses valid numeric channel ID", func() {
				service := &notifiarr.Service{}
				channelID, err := service.ParseChannelID("123456789")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(channelID).To(gomega.Equal(123456789))
			})

			ginkgo.It("returns error for non-numeric channel ID", func() {
				service := &notifiarr.Service{}
				_, err := service.ParseChannelID("invalid")
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid channel ID"))
			})

			ginkgo.It("returns error for empty channel ID", func() {
				service := &notifiarr.Service{}
				_, err := service.ParseChannelID("")
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid channel ID"))
			})

			ginkgo.It("parses large numeric channel ID", func() {
				service := &notifiarr.Service{}
				channelID, err := service.ParseChannelID("9223372036854775807")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(channelID).To(gomega.Equal(9223372036854775807))
			})
		})

		ginkgo.Describe("ParseFields", func() {
			ginkgo.It("parses valid JSON fields", func() {
				service := &notifiarr.Service{}
				fields, err := service.ParseFields(
					`[{"title":"Test","text":"Content","inline":true}]`,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(fields).To(gomega.HaveLen(1))
				gomega.Expect(fields[0].Title).To(gomega.Equal("Test"))
				gomega.Expect(fields[0].Text).To(gomega.Equal("Content"))
				gomega.Expect(fields[0].Inline).To(gomega.BeTrue())
			})

			ginkgo.It("returns error for invalid JSON", func() {
				service := &notifiarr.Service{}
				_, err := service.ParseFields(`invalid json`)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})

			ginkgo.It("parses empty JSON array", func() {
				service := &notifiarr.Service{}
				fields, err := service.ParseFields(`[]`)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(fields).To(gomega.BeEmpty())
			})

			ginkgo.It("parses multiple fields", func() {
				service := &notifiarr.Service{}
				fields, err := service.ParseFields(
					`[{"title":"Field1","text":"Text1"},{"title":"Field2","text":"Text2"}]`,
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(fields).To(gomega.HaveLen(2))
			})
		})

		ginkgo.Describe("parseMention", func() {
			ginkgo.It("parses user mention <@123>", func() {
				mentionType, id := notifiarr.ParseMention("<@123>")
				gomega.Expect(mentionType).To(gomega.Equal(notifiarr.MentionTypeUser))
				gomega.Expect(id).To(gomega.Equal(123))
			})

			ginkgo.It("parses user mention with nickname <@!456>", func() {
				mentionType, id := notifiarr.ParseMention("<@!456>")
				gomega.Expect(mentionType).To(gomega.Equal(notifiarr.MentionTypeUser))
				gomega.Expect(id).To(gomega.Equal(456))
			})

			ginkgo.It("parses role mention <@&789>", func() {
				mentionType, id := notifiarr.ParseMention("<@&789>")
				gomega.Expect(mentionType).To(gomega.Equal(notifiarr.MentionTypeRole))
				gomega.Expect(id).To(gomega.Equal(789))
			})

			ginkgo.It("returns none for invalid mention format", func() {
				mentionType, id := notifiarr.ParseMention("invalid")
				gomega.Expect(mentionType).To(gomega.Equal(notifiarr.MentionTypeNone))
				gomega.Expect(id).To(gomega.Equal(0))
			})

			ginkgo.It("returns none for mention without closing bracket", func() {
				mentionType, id := notifiarr.ParseMention("<@123")
				gomega.Expect(mentionType).To(gomega.Equal(notifiarr.MentionTypeNone))
				gomega.Expect(id).To(gomega.Equal(0))
			})

			ginkgo.It("returns none for non-numeric ID", func() {
				mentionType, id := notifiarr.ParseMention("<@abc>")
				gomega.Expect(mentionType).To(gomega.Equal(notifiarr.MentionTypeNone))
				gomega.Expect(id).To(gomega.Equal(0))
			})
		})

		ginkgo.Describe("ExtractPingIDs", func() {
			ginkgo.It("extracts user and role IDs from mentions", func() {
				service := &notifiarr.Service{}
				mentions := []string{"<@123>", "<@&456>", "<@!789>"}
				userIDs, roleIDs := service.ExtractPingIDs(mentions)
				gomega.Expect(userIDs).To(gomega.Equal([]int{123, 789}))
				gomega.Expect(roleIDs).To(gomega.Equal([]int{456}))
			})

			ginkgo.It("handles empty mentions list", func() {
				service := &notifiarr.Service{}
				mentions := []string{}
				userIDs, roleIDs := service.ExtractPingIDs(mentions)
				gomega.Expect(userIDs).To(gomega.BeEmpty())
				gomega.Expect(roleIDs).To(gomega.BeEmpty())
			})

			ginkgo.It("ignores invalid mentions", func() {
				service := &notifiarr.Service{}
				mentions := []string{"<@123>", "invalid", "<@&456>"}
				userIDs, roleIDs := service.ExtractPingIDs(mentions)
				gomega.Expect(userIDs).To(gomega.Equal([]int{123}))
				gomega.Expect(roleIDs).To(gomega.Equal([]int{456}))
			})
		})

		ginkgo.Describe("parseUpdateFlag", func() {
			ginkgo.It("parses 'true' string to true", func() {
				params := types.Params{"update": "true"}
				result := notifiarr.ParseUpdateFlag(params)
				gomega.Expect(result).NotTo(gomega.BeNil())
				gomega.Expect(*result).To(gomega.BeTrue())
			})

			ginkgo.It("parses 'false' string to false", func() {
				params := types.Params{"update": "false"}
				result := notifiarr.ParseUpdateFlag(params)
				gomega.Expect(result).NotTo(gomega.BeNil())
				gomega.Expect(*result).To(gomega.BeFalse())
			})

			ginkgo.It("returns nil for invalid value", func() {
				params := types.Params{"update": "invalid"}
				result := notifiarr.ParseUpdateFlag(params)
				gomega.Expect(result).To(gomega.BeNil())
			})

			ginkgo.It("returns nil when update key is missing", func() {
				params := types.Params{}
				result := notifiarr.ParseUpdateFlag(params)
				gomega.Expect(result).To(gomega.BeNil())
			})

			ginkgo.It("returns nil for empty update value", func() {
				params := types.Params{"update": ""}
				result := notifiarr.ParseUpdateFlag(params)
				gomega.Expect(result).To(gomega.BeNil())
			})
		})
	})

	ginkgo.Describe("Send method edge cases", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = &notifiarr.Service{}
			service.SetLogger(logger)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("handles nil params gracefully", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				httpmock.NewStringResponder(200, ""),
			)

			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("handles empty params map", func() {
			serviceURL := testutils.URLMust("notifiarr://apikey123")
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				"https://notifiarr.com/api/v1/notification/passthrough/apikey123",
				httpmock.NewStringResponder(200, ""),
			)

			emptyParams := types.Params{}
			err = service.Send("Test message", &emptyParams)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
