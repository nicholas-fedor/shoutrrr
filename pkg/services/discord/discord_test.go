package discord_test

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestDiscord runs the Discord service test suite using Ginkgo.
func TestDiscord(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Discord Suite")
}

var (
	dummyColors   = [types.MessageLevelCount]uint{}
	service       *discord.Service
	envDiscordURL *url.URL
	logger        *log.Logger
	_             = ginkgo.BeforeSuite(func() {
		service = &discord.Service{}
		envDiscordURL, _ = url.Parse(os.Getenv("SHOUTRRR_DISCORD_URL"))
		logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)
	})
)

var _ = ginkgo.Describe("the discord service", func() {
	ginkgo.When("running integration tests", func() {
		ginkgo.It("should work without errors", func() {
			if envDiscordURL.String() == "" {
				return
			}

			serviceURL, _ := url.Parse(envDiscordURL.String())
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("this is an integration test", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
	ginkgo.Describe("the service", func() {
		ginkgo.It("should implement Service interface", func() {
			var impl types.Service = service
			gomega.Expect(impl).ToNot(gomega.BeNil())
		})
		ginkgo.It("returns the correct service identifier", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal("discord"))
		})
	})
	ginkgo.Describe("creating a config", func() {
		ginkgo.When("given a URL and a message", func() {
			ginkgo.It("should return an error if no arguments are supplied", func() {
				serviceURL, _ := url.Parse("discord://")
				err := service.Initialize(serviceURL, nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
			ginkgo.It("should not return an error if exactly two arguments are given", func() {
				serviceURL, _ := url.Parse("discord://dummyToken@dummyChannel")
				err := service.Initialize(serviceURL, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("should not return an error when given the raw path parameter", func() {
				serviceURL, _ := url.Parse("discord://dummyToken@dummyChannel/raw")
				err := service.Initialize(serviceURL, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
			ginkgo.It("should set the JSON flag when given the raw path parameter", func() {
				serviceURL, _ := url.Parse("discord://dummyToken@dummyChannel/raw")
				config := discord.Config{}
				err := config.SetURL(serviceURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(config.JSON).To(gomega.BeTrue())
			})
			ginkgo.It("should not set the JSON flag when not provided raw path parameter", func() {
				serviceURL, _ := url.Parse("discord://dummyToken@dummyChannel")
				config := discord.Config{}
				err := config.SetURL(serviceURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(config.JSON).NotTo(gomega.BeTrue())
			})
			ginkgo.It("should return an error if more than two arguments are given", func() {
				serviceURL, _ := url.Parse("discord://dummyToken@dummyChannel/illegal-argument")
				err := service.Initialize(serviceURL, nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
		})
		ginkgo.When("parsing the configuration URL", func() {
			ginkgo.It("should be identical after de-/serialization", func() {
				testURL := "discord://token@channel?avatar=TestBot.jpg&color=0x112233&colordebug=0x223344&colorerror=0x334455&colorinfo=0x445566&colorwarn=0x556677&splitlines=No&title=Test+Title&username=TestBot"

				url, err := url.Parse(testURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "parsing")

				config := &discord.Config{}
				err = config.SetURL(url)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "verifying")

				outputURL := config.GetURL()
				gomega.Expect(outputURL.String()).To(gomega.Equal(testURL))
			})
			ginkgo.It("should include thread_id in URL after de-/serialization", func() {
				testURL := "discord://token@channel?color=0x50d9ff&thread_id=123456789&title=Test+Title"

				url, err := url.Parse(testURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "parsing")

				config := &discord.Config{}
				resolver := format.NewPropKeyResolver(config)
				err = resolver.SetDefaultProps(config)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "setting defaults")
				err = config.SetURL(url)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "verifying")
				gomega.Expect(config.ThreadID).To(gomega.Equal("123456789"))

				outputURL := config.GetURL()
				gomega.Expect(outputURL.String()).To(gomega.Equal(testURL))
			})
			ginkgo.It("should handle thread_id with whitespace correctly", func() {
				testURL := "discord://token@channel?color=0x50d9ff&thread_id=%20%20123456789%20%20&title=Test+Title"
				expectedThreadID := "123456789"

				url, err := url.Parse(testURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "parsing")

				config := &discord.Config{}
				resolver := format.NewPropKeyResolver(config)
				err = resolver.SetDefaultProps(config)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "setting defaults")
				err = config.SetURL(url)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "verifying")
				gomega.Expect(config.ThreadID).To(gomega.Equal(expectedThreadID))
				gomega.Expect(config.GetURL().Query().Get("thread_id")).
					To(gomega.Equal(expectedThreadID))
				gomega.Expect(config.GetURL().String()).
					To(gomega.Equal("discord://token@channel?color=0x50d9ff&thread_id=123456789&title=Test+Title"))
			})
			ginkgo.It("should not include thread_id in URL when empty", func() {
				config := &discord.Config{}
				resolver := format.NewPropKeyResolver(config)
				err := resolver.SetDefaultProps(config)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "setting defaults")

				serviceURL, _ := url.Parse("discord://token@channel?title=Test+Title")
				err = config.SetURL(serviceURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "setting URL")

				outputURL := config.GetURL()
				gomega.Expect(outputURL.Query().Get("thread_id")).To(gomega.BeEmpty())
				gomega.Expect(outputURL.String()).
					To(gomega.Equal("discord://token@channel?color=0x50d9ff&title=Test+Title"))
			})
		})
	})
	ginkgo.Describe("creating a json payload", func() {
		ginkgo.When("given a blank message", func() {
			ginkgo.When("split lines is enabled", func() {
				ginkgo.It("should return an error", func() {
					items := []types.MessageItem{}
					gomega.Expect(items).To(gomega.BeEmpty())
					_, err := discord.CreatePayloadFromItems(items, "title", dummyColors)
					gomega.Expect(err).To(gomega.HaveOccurred())
				})
			})
			ginkgo.When("split lines is disabled", func() {
				ginkgo.It("should return an error", func() {
					batches := discord.CreateItemsFromPlain("", false)
					items := batches[0]
					gomega.Expect(items).To(gomega.BeEmpty())
					_, err := discord.CreatePayloadFromItems(items, "title", dummyColors)
					gomega.Expect(err).To(gomega.HaveOccurred())
				})
			})
		})
		ginkgo.When("given a message that exceeds the max length", func() {
			ginkgo.It("should return a payload with chunked messages", func() {
				payload, err := buildPayloadFromHundreds(42, "Title", dummyColors)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				items := payload.Embeds
				gomega.Expect(items).To(gomega.HaveLen(3))
				gomega.Expect(items[0].Content).To(gomega.HaveLen(1994))
				gomega.Expect(items[1].Content).To(gomega.HaveLen(1999))
				gomega.Expect(items[2].Content).To(gomega.HaveLen(205))
			})
			ginkgo.It("omit characters above total max", func() {
				payload, err := buildPayloadFromHundreds(62, "", dummyColors)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				items := payload.Embeds
				gomega.Expect(items).To(gomega.HaveLen(4))
				gomega.Expect(items[0].Content).To(gomega.HaveLen(1994))
				gomega.Expect(items[1].Content).To(gomega.HaveLen(1999))
				gomega.Expect(items[2].Content).To(gomega.HaveLen(1999))
				gomega.Expect(items[3].Content).To(gomega.HaveLen(5))
			})
			ginkgo.When("no title is supplied and content fits", func() {
				ginkgo.It("should return a payload without a meta chunk", func() {
					payload, err := buildPayloadFromHundreds(42, "", dummyColors)
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
					gomega.Expect(payload.Embeds[0].Footer).To(gomega.BeNil())
					gomega.Expect(payload.Embeds[0].Title).To(gomega.BeEmpty())
				})
			})
			ginkgo.When("title is supplied, but content fits", func() {
				ginkgo.It("should return a payload with a meta chunk", func() {
					payload, err := buildPayloadFromHundreds(42, "Title", dummyColors)
					gomega.Expect(err).ToNot(gomega.HaveOccurred())
					gomega.Expect(payload.Embeds[0].Title).ToNot(gomega.BeEmpty())
				})
			})
			ginkgo.It("rich test 1", func() {
				testTime, _ := time.Parse(time.RFC3339, time.RFC3339)
				items := []types.MessageItem{
					{
						Text:      "Message",
						Timestamp: testTime,
						Level:     types.Warning,
					},
				}
				payload, err := discord.CreatePayloadFromItems(items, "Title", dummyColors)
				gomega.Expect(err).ToNot(gomega.HaveOccurred())

				item := payload.Embeds[0]
				gomega.Expect(payload.Embeds).To(gomega.HaveLen(1))
				gomega.Expect(item.Footer.Text).To(gomega.Equal(types.Warning.String()))
				gomega.Expect(item.Title).To(gomega.Equal("Title"))
				gomega.Expect(item.Color).To(gomega.Equal(dummyColors[types.Warning]))
			})
		})
	})
	ginkgo.Describe("sending the payload", func() {
		dummyConfig := discord.Config{
			WebhookID: "1",
			Token:     "dummyToken",
		}
		var service discord.Service
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
			service = discord.Service{}
			if err := service.Initialize(dummyConfig.GetURL(), logger); err != nil {
				panic(fmt.Errorf("service initialization failed: %w", err))
			}
		})
		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})
		ginkgo.It("should not report an error if the server accepts the payload", func() {
			setupResponder(&dummyConfig, 204)
			gomega.Expect(service.Send("Message", nil)).To(gomega.Succeed())
		})
		ginkgo.It("should report an error if the server response is not OK", func() {
			setupResponder(&dummyConfig, 400)
			gomega.Expect(service.Initialize(dummyConfig.GetURL(), logger)).To(gomega.Succeed())
			gomega.Expect(service.Send("Message", nil)).NotTo(gomega.Succeed())
		})
		ginkgo.It("should report an error if the message is empty", func() {
			setupResponder(&dummyConfig, 204)
			gomega.Expect(service.Initialize(dummyConfig.GetURL(), logger)).To(gomega.Succeed())
			gomega.Expect(service.Send("", nil)).NotTo(gomega.Succeed())
		})
		ginkgo.When("using a custom json payload", func() {
			ginkgo.It("should report an error if the server response is not OK", func() {
				config := dummyConfig
				config.JSON = true
				setupResponder(&config, 400)
				gomega.Expect(service.Initialize(config.GetURL(), logger)).To(gomega.Succeed())
				gomega.Expect(service.Send("Message", nil)).NotTo(gomega.Succeed())
			})
		})
		ginkgo.It("should trim whitespace from thread_id in API URL", func() {
			config := discord.Config{
				WebhookID: "1",
				Token:     "dummyToken",
				ThreadID:  "  123456789  ",
			}
			service := discord.Service{}
			err := service.Initialize(config.GetURL(), logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupResponder(&config, 204)
			err = service.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Verify the API URL used in the HTTP request
			targetURL := discord.CreateAPIURLFromConfig(&config)
			gomega.Expect(targetURL).
				To(gomega.Equal("https://discord.com/api/webhooks/1/dummyToken?thread_id=123456789"))
		})
	})
})

// buildPayloadFromHundreds creates a Discord webhook payload from a repeated 100-character string.
func buildPayloadFromHundreds(
	hundreds int,
	title string,
	colors [types.MessageLevelCount]uint,
) (discord.WebhookPayload, error) {
	hundredChars := "this string is exactly (to the letter) a hundred characters long which will make the send func error"
	builder := strings.Builder{}

	for range hundreds {
		builder.WriteString(hundredChars)
	}

	batches := discord.CreateItemsFromPlain(
		builder.String(),
		false,
	) // SplitLines is always false in these tests
	items := batches[0]

	return discord.CreatePayloadFromItems(items, title, colors)
}

// setupResponder configures an HTTP mock responder for a Discord webhook URL with the given status code.
func setupResponder(config *discord.Config, code int) {
	targetURL := discord.CreateAPIURLFromConfig(config)
	httpmock.RegisterResponder("POST", targetURL, httpmock.NewStringResponder(code, ""))
}
