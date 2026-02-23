package lark

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestLark(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Lark Suite")
}

var (
	service *Service
	logger  *log.Logger
	_       = ginkgo.BeforeSuite(func() {
		logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)
	})
)

const fullURL = "lark://open.larksuite.com/token?secret=sss"

var _ = ginkgo.Describe("Lark Test", func() {
	ginkgo.BeforeEach(func() {
		service = &Service{}
	})

	ginkgo.When("parsing the configuration URL", func() {
		ginkgo.It("should be identical after de-/serialization", func() {
			url := testutils.URLMust(fullURL)
			config := &Config{}
			pkr := format.NewPropKeyResolver(config)
			err := config.setURL(&pkr, url)
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

			outputURL := config.GetURL()
			ginkgo.GinkgoT().Logf("\n\n%s\n%s\n\n-", outputURL, fullURL)
			gomega.Expect(outputURL.String()).To(gomega.Equal(fullURL))
		})
	})

	ginkgo.Context("basic service API methods", func() {
		var config *Config

		ginkgo.BeforeEach(func() {
			config = &Config{}
		})
		ginkgo.It("should not allow getting invalid query values", func() {
			testutils.TestConfigGetInvalidQueryValue(config)
		})
		ginkgo.It("should not allow setting invalid query values", func() {
			testutils.TestConfigSetInvalidQueryValue(
				config,
				"lark://endpoint/token?secret=sss&foo=bar",
			)
		})
		ginkgo.It("should have the expected number of fields and enums", func() {
			testutils.TestConfigGetEnumsCount(config, 0)
			testutils.TestConfigGetFieldsCount(config, 3)
		})
	})

	ginkgo.When("initializing the service", func() {
		ginkgo.It("should fail with invalid host", func() {
			err := service.Initialize(testutils.URLMust("lark://invalid.com/token"), logger)
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidHost))
		})
		ginkgo.It("should fail with no path", func() {
			err := service.Initialize(testutils.URLMust("lark://open.larksuite.com"), logger)
			gomega.Expect(err).To(gomega.MatchError(ErrNoPath))
		})
	})

	ginkgo.When("sending a message", func() {
		ginkgo.When("the message is too large", func() {
			ginkgo.It("should return large message error", func() {
				data := make([]string, 410)
				for i := range data {
					data[i] = "0123456789"
				}

				message := strings.Join(data, "")
				service := Service{Config: &Config{Host: larkHost, Path: "token"}}
				gomega.Expect(service.Send(message, nil)).To(gomega.MatchError(ErrLargeMessage))
			})
		})

		ginkgo.When("an invalid param is passed", func() {
			ginkgo.It("should fail to send messages", func() {
				service := Service{Config: &Config{Host: larkHost, Path: "token"}}
				gomega.Expect(
					service.Send("test message", &types.Params{"invalid": "value"}),
				).To(gomega.MatchError(gomega.ContainSubstring("not a valid config key: invalid")))
			})
		})

		ginkgo.Context("sending message by HTTP", func() {
			ginkgo.BeforeEach(func() {
				httpmock.ActivateNonDefault(httpClient)
			})
			ginkgo.AfterEach(func() {
				httpmock.DeactivateAndReset()
			})

			ginkgo.It("should send text message successfully", func() {
				httpmock.RegisterResponder(
					http.MethodPost,
					"/open-apis/bot/v2/hook/token",
					httpmock.NewJsonResponderOrPanic(
						http.StatusOK,
						map[string]any{"code": 0, "msg": "success"},
					),
				)

				err := service.Initialize(testutils.URLMust(fullURL), logger)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = service.Send("message", nil)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.It("should send post message with title successfully", func() {
				httpmock.RegisterResponder(
					http.MethodPost,
					"/open-apis/bot/v2/hook/token",
					httpmock.NewJsonResponderOrPanic(
						http.StatusOK,
						map[string]any{"code": 0, "msg": "success"},
					),
				)

				err := service.Initialize(testutils.URLMust(fullURL), logger)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = service.Send("message", &types.Params{"title": "title"})
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.It("should send post message with link successfully", func() {
				httpmock.RegisterResponder(
					http.MethodPost,
					"/open-apis/bot/v2/hook/token",
					httpmock.NewJsonResponderOrPanic(
						http.StatusOK,
						map[string]any{"code": 0, "msg": "success"},
					),
				)

				err := service.Initialize(testutils.URLMust(fullURL), logger)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = service.Send(
					"message",
					&types.Params{"title": "title", "link": "https://example.com"},
				)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
			})

			ginkgo.It("should return error on network failure", func() {
				httpmock.RegisterResponder(
					http.MethodPost,
					"/open-apis/bot/v2/hook/token",
					httpmock.NewErrorResponder(errors.New("network error")),
				)

				err := service.Initialize(testutils.URLMust(fullURL), logger)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = service.Send("message", nil)
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("network error")))
			})

			ginkgo.It("should return error on invalid JSON response", func() {
				httpmock.RegisterResponder(
					http.MethodPost,
					"/open-apis/bot/v2/hook/token",
					httpmock.NewStringResponder(http.StatusOK, "some response"),
				)

				err := service.Initialize(testutils.URLMust(fullURL), logger)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = service.Send("message", nil)
				gomega.Expect(err).
					To(gomega.MatchError(gomega.ContainSubstring("invalid character")))
			})

			ginkgo.It("should return error on non-zero response code", func() {
				httpmock.RegisterResponder(
					http.MethodPost,
					"/open-apis/bot/v2/hook/token",
					httpmock.NewJsonResponderOrPanic(
						http.StatusOK,
						map[string]any{"code": 1, "msg": "some error"},
					),
				)

				err := service.Initialize(testutils.URLMust(fullURL), logger)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = service.Send("message", nil)
				gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("some error")))
			})

			ginkgo.It("should fail on HTTP 400 status", func() {
				httpmock.RegisterResponder(
					http.MethodPost,
					"/open-apis/bot/v2/hook/token",
					httpmock.NewStringResponder(http.StatusBadRequest, "bad request"),
				)

				err := service.Initialize(testutils.URLMust(fullURL), logger)
				gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				err = service.Send("message", nil)
				gomega.Expect(err).
					To(gomega.MatchError(gomega.ContainSubstring("unexpected status 400")))
			})
		})
	})
})
