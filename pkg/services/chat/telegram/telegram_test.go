package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// sendSuccessBody is a sendMessage response that the Telegram API returns on success.
const sendSuccessBody = `{"ok":true,"result":{"message_id":1}}`

var (
	envTelegramURL string
	logger         *log.Logger

	_ = ginkgo.BeforeSuite(func() {
		envTelegramURL = os.Getenv("SHOUTRRR_TELEGRAM_URL")
		logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)
	})
)

var _ = ginkgo.Describe("the telegram service", func() {
	var telegram *Service // No telegram. prefix needed

	ginkgo.BeforeEach(func() {
		telegram = &Service{}
	})

	ginkgo.When("running integration tests", func() {
		ginkgo.It("should not error out", func() {
			if envTelegramURL == "" {
				return
			}

			serviceURL, _ := url.Parse(envTelegramURL)
			err := telegram.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = telegram.Send("This is an integration test Message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.When("given a Message that exceeds the max length", func() {
			ginkgo.It("should generate an error", func() {
				if envTelegramURL == "" {
					return
				}

				hundredChars := "this string is exactly (to the letter) a hundred characters long which will make the send func error"
				serviceURL, _ := url.Parse("telegram://12345:mock-token@telegram/?chats=channel-1")

				builder := strings.Builder{}
				for range 42 {
					builder.WriteString(hundredChars)
				}

				err := telegram.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = telegram.Send(builder.String(), nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
		})
		ginkgo.When("given a valid request with a faked token", func() {
			if envTelegramURL == "" {
				return
			}

			ginkgo.It("should generate a 401", func() {
				serviceURL, _ := url.Parse(
					"telegram://000000000:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA@telegram/?chats=channel-id",
				)
				message := "this is a perfectly valid Message"

				err := telegram.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				err = telegram.Send(message, nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(strings.Contains(err.Error(), "401 Unauthorized")).To(gomega.BeTrue())
			})
		})
	})

	ginkgo.Describe("creating configurations", func() {
		ginkgo.When("given an url", func() {
			ginkgo.It("should return an error if no arguments where supplied", func() {
				expectErrorAndEmptyObject(telegram, "telegram://", logger)
			})
			ginkgo.It("should return an error if the token has an invalid format", func() {
				expectErrorAndEmptyObject(telegram, "telegram://invalid-token", logger)
			})
			ginkgo.It("should not echo an invalid token in the error", func() {
				err := telegram.Initialize(testutils.URLMust("telegram://bot:SECRETtoken@telegram/?chats=1"), logger)
				gomega.Expect(err).To(gomega.MatchError(ErrInvalidToken))
				gomega.Expect(err.Error()).NotTo(gomega.ContainSubstring("SECRETtoken"))
			})
			ginkgo.It("should return an error if only the api token where supplied", func() {
				expectErrorAndEmptyObject(telegram, "telegram://12345:mock-token@telegram", logger)
			})

			ginkgo.When("the url is valid", func() {
				var config *Config // No telegram. prefix

				var err error

				ginkgo.BeforeEach(func() {
					serviceURL, _ := url.Parse(
						"telegram://12345:mock-token@telegram/?chats=channel-1,channel-2,channel-3",
					)
					err = telegram.Initialize(serviceURL, logger)
					config = telegram.GetConfig()
				})

				ginkgo.It("should create a config object", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(config).ToNot(gomega.BeNil())
				})
				ginkgo.It("should create a config object containing the API Token", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(config.Token).To(gomega.Equal("12345:mock-token"))
				})
				ginkgo.It("should add every chats query field as a chat ID", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(config.Chats).To(gomega.Equal([]string{
						"channel-1",
						"channel-2",
						"channel-3",
					}))
				})
			})
		})
	})

	ginkgo.Describe("sending the payload", func() {
		var err error

		ginkgo.BeforeEach(func() {
			httpmock.Activate()
		})
		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})
		ginkgo.It("should not report an error if the server accepts the payload", func() {
			serviceURL, _ := url.Parse(
				"telegram://12345:mock-token@telegram/?chats=channel-1,channel-2,channel-3",
			)
			err = telegram.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupSendMessageResponder(telegram.GetConfig().Token, 200, sendSuccessBody)

			err = telegram.Send("Message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should report a transport error without the token", func() {
			serviceURL := testutils.URLMust("telegram://12345:mock-token@telegram/?chats=channel-1")
			err = telegram.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			transportErr := errors.New("connection reset")
			httpmock.RegisterResponder(
				"POST",
				"https://api.telegram.org/bot12345:mock-token/sendMessage",
				httpmock.NewErrorResponder(transportErr),
			)

			err = telegram.Send("Message", nil)
			gomega.Expect(err).To(gomega.MatchError(transportErr))
			gomega.Expect(err.Error()).NotTo(gomega.ContainSubstring("mock-token"))
		})

		ginkgo.It("should report a response that is not a Telegram error", func() {
			serviceURL := testutils.URLMust("telegram://12345:mock-token@telegram/?chats=channel-1")
			err = telegram.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupSendMessageResponder("12345:mock-token", http.StatusBadGateway, "<html>Bad Gateway</html>")

			err = telegram.Send("Message", nil)
			gomega.Expect(err).To(gomega.MatchError(jsonclient.ErrUnexpectedStatus))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("502"))
		})

		ginkgo.It("should report the Telegram API error with the HTTP status", func() {
			serviceURL := testutils.URLMust("telegram://12345:mock-token@telegram/?chats=channel-1")
			err = telegram.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupSendMessageResponder(
				"12345:mock-token",
				http.StatusBadRequest,
				`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`,
			)

			err = telegram.Send("Message", nil)
			gomega.Expect(err).To(gomega.MatchError(jsonclient.ErrUnexpectedStatus))

			apiErr, ok := errors.AsType[*responseError](err)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(apiErr.Description).To(gomega.Equal("Bad Request: chat not found"))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("400"))
		})

		ginkgo.It("should report a failed response without an error description", func() {
			serviceURL := testutils.URLMust("telegram://12345:mock-token@telegram/?chats=channel-1")
			err = telegram.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupSendMessageResponder("12345:mock-token", http.StatusOK, `{"ok":false}`)

			err = telegram.Send("Message", nil)
			gomega.Expect(err).To(gomega.MatchError(ErrUnexpectedResponse))
		})

		ginkgo.It("should send to the chats set by the send params", func() {
			serviceURL := testutils.URLMust("telegram://12345:mock-token@telegram/?chats=channel-1")
			err = telegram.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			var chats []string

			httpmock.RegisterResponder(
				"POST",
				"https://api.telegram.org/bot12345:mock-token/sendMessage",
				func(req *http.Request) (*http.Response, error) {
					body, readErr := io.ReadAll(req.Body)
					gomega.Expect(readErr).NotTo(gomega.HaveOccurred())

					payload := struct {
						ChatID string `json:"chat_id"`
					}{}
					gomega.Expect(json.Unmarshal(body, &payload)).To(gomega.Succeed())
					chats = append(chats, payload.ChatID)

					return httpmock.NewStringResponse(http.StatusOK, sendSuccessBody), nil
				},
			)

			err = telegram.Send("Message", &types.Params{"chats": "channel-9"})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(chats).To(gomega.Equal([]string{"channel-9"}))
		})
	})

	ginkgo.It("should implement basic service API methods correctly", func() {
		serviceURL, _ := url.Parse("telegram://12345:mock-token@telegram/?chats=channel-1")
		err := telegram.Initialize(serviceURL, logger)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		config := telegram.GetConfig()
		testutils.TestConfigGetInvalidQueryValue(config)
		testutils.TestConfigSetInvalidQueryValue(
			config,
			"telegram://12345:mock-token@telegram/?chats=channel-1&foo=bar",
		)
		testutils.TestConfigGetEnumsCount(config, 1)
		testutils.TestConfigGetFieldsCount(config, 6)
	})
	ginkgo.It("should return the correct service ID", func() {
		service := &Service{}
		gomega.Expect(service.GetID()).To(gomega.Equal("telegram"))
	})
})

func TestTelegram(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Telegram Suite")
}

func expectErrorAndEmptyObject(telegram *Service, rawURL string, logger *log.Logger) {
	serviceURL, _ := url.Parse(rawURL)
	err := telegram.Initialize(serviceURL, logger)
	gomega.Expect(err).To(gomega.HaveOccurred())

	config := telegram.GetConfig()
	gomega.Expect(config.Token).To(gomega.BeEmpty())
	gomega.Expect(config.Chats).To(gomega.BeEmpty())
}

// setupSendMessageResponder answers sendMessage requests for token with code and body.
func setupSendMessageResponder(token string, code int, body string) {
	targetURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	httpmock.RegisterResponder("POST", targetURL, httpmock.NewStringResponder(code, body))
}
