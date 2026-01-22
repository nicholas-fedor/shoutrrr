package discord

import (
	"net/url"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	testWebhookURL           = "https://discord.com/api/webhooks/123456789/test-token"
	testWebhookURLWithThread = "https://discord.com/api/webhooks/123456789/test-token?thread_id=987654321"
)

var _ = ginkgo.Describe("Discord Config Unit Tests", func() {
	ginkgo.Describe("LevelColors method", func() {
		ginkgo.It("should return correct color mapping for default colors", func() {
			config := &Config{
				Color:      0x50D9ff,
				ColorError: 0xd60510,
				ColorWarn:  0xffc441,
				ColorInfo:  0x2488ff,
				ColorDebug: 0x7b00ab,
			}

			colors := config.LevelColors()

			gomega.Expect(colors[types.Unknown]).To(gomega.Equal(uint(0x50D9ff)))
			gomega.Expect(colors[types.Error]).To(gomega.Equal(uint(0xd60510)))
			gomega.Expect(colors[types.Warning]).To(gomega.Equal(uint(0xffc441)))
			gomega.Expect(colors[types.Info]).To(gomega.Equal(uint(0x2488ff)))
			gomega.Expect(colors[types.Debug]).To(gomega.Equal(uint(0x7b00ab)))
		})

		ginkgo.It("should return correct color mapping for custom colors", func() {
			config := &Config{
				Color:      0x123456,
				ColorError: 0x654321,
				ColorWarn:  0xabcdef,
				ColorInfo:  0xfedcba,
				ColorDebug: 0x987654,
			}

			colors := config.LevelColors()

			gomega.Expect(colors[types.Unknown]).To(gomega.Equal(uint(0x123456)))
			gomega.Expect(colors[types.Error]).To(gomega.Equal(uint(0x654321)))
			gomega.Expect(colors[types.Warning]).To(gomega.Equal(uint(0xabcdef)))
			gomega.Expect(colors[types.Info]).To(gomega.Equal(uint(0xfedcba)))
			gomega.Expect(colors[types.Debug]).To(gomega.Equal(uint(0x987654)))
		})

		ginkgo.It("should handle zero values", func() {
			config := &Config{}

			colors := config.LevelColors()

			gomega.Expect(colors[types.Unknown]).To(gomega.Equal(uint(0)))
			gomega.Expect(colors[types.Error]).To(gomega.Equal(uint(0)))
			gomega.Expect(colors[types.Warning]).To(gomega.Equal(uint(0)))
			gomega.Expect(colors[types.Info]).To(gomega.Equal(uint(0)))
			gomega.Expect(colors[types.Debug]).To(gomega.Equal(uint(0)))
		})
	})

	ginkgo.Describe("GetURL and SetURL methods", func() {
		ginkgo.It("should serialize and deserialize basic config correctly", func() {
			originalConfig := &Config{
				WebhookID: "123456789",
				Token:     "test-token",
			}

			url := originalConfig.GetURL()
			gomega.Expect(url.Scheme).To(gomega.Equal("discord"))
			gomega.Expect(url.Host).To(gomega.Equal("123456789"))
			gomega.Expect(url.User.Username()).To(gomega.Equal("test-token"))

			newConfig := &Config{}
			err := newConfig.SetURL(url)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(newConfig.WebhookID).To(gomega.Equal("123456789"))
			gomega.Expect(newConfig.Token).To(gomega.Equal("test-token"))
		})

		ginkgo.It("should handle all query parameters correctly", func() {
			originalConfig := &Config{
				WebhookID:  "123456789",
				Token:      "test-token",
				Title:      "Test Title",
				Username:   "TestBot",
				Avatar:     "https://example.com/avatar.png",
				Color:      0x50d9ff,
				ColorError: 0xd60510,
				ColorWarn:  0xffc441,
				ColorInfo:  0x2488ff,
				ColorDebug: 0x7b00ab,
				SplitLines: false,
				JSON:       false,
				ThreadID:   "987654321",
			}

			url := originalConfig.GetURL()
			gomega.Expect(url.Scheme).To(gomega.Equal("discord"))
			gomega.Expect(url.Host).To(gomega.Equal("123456789"))
			gomega.Expect(url.User.Username()).To(gomega.Equal("test-token"))

			newConfig := &Config{}
			resolver := format.NewPropKeyResolver(newConfig)
			err := resolver.SetDefaultProps(newConfig)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = newConfig.SetURL(url)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(newConfig.WebhookID).To(gomega.Equal(originalConfig.WebhookID))
			gomega.Expect(newConfig.Token).To(gomega.Equal(originalConfig.Token))
			gomega.Expect(newConfig.Title).To(gomega.Equal(originalConfig.Title))
			gomega.Expect(newConfig.Username).To(gomega.Equal(originalConfig.Username))
			gomega.Expect(newConfig.Avatar).To(gomega.Equal(originalConfig.Avatar))
			gomega.Expect(newConfig.Color).To(gomega.Equal(originalConfig.Color))
			gomega.Expect(newConfig.ColorError).To(gomega.Equal(originalConfig.ColorError))
			gomega.Expect(newConfig.ColorWarn).To(gomega.Equal(originalConfig.ColorWarn))
			gomega.Expect(newConfig.ColorInfo).To(gomega.Equal(originalConfig.ColorInfo))
			gomega.Expect(newConfig.ColorDebug).To(gomega.Equal(originalConfig.ColorDebug))
			gomega.Expect(newConfig.SplitLines).To(gomega.Equal(originalConfig.SplitLines))
			gomega.Expect(newConfig.JSON).To(gomega.Equal(originalConfig.JSON))
			gomega.Expect(newConfig.ThreadID).To(gomega.Equal(originalConfig.ThreadID))
		})

		ginkgo.It("should handle JSON mode correctly", func() {
			originalConfig := &Config{
				WebhookID: "123456789",
				Token:     "test-token",
				JSON:      true,
			}

			url := originalConfig.GetURL()
			gomega.Expect(url.Path).To(gomega.Equal("/raw"))

			newConfig := &Config{}
			err := newConfig.SetURL(url)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(newConfig.JSON).To(gomega.BeTrue())
		})

		ginkgo.It("should handle thread_id with whitespace correctly", func() {
			testURL := "discord://token@channel?thread_id=%20%20123456789%20%20"
			url, err := url.Parse(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			config := &Config{}
			resolver := format.NewPropKeyResolver(config)
			err = resolver.SetDefaultProps(config)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(url)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.ThreadID).To(gomega.Equal("123456789"))

			outputURL := config.GetURL()
			gomega.Expect(outputURL.Query().Get("thread_id")).To(gomega.Equal("123456789"))
		})

		ginkgo.It("should not include thread_id in URL when empty", func() {
			config := &Config{
				WebhookID: "123456789",
				Token:     "test-token",
				ThreadID:  "",
			}

			url := config.GetURL()
			gomega.Expect(url.Query().Get("thread_id")).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle empty values gracefully", func() {
			config := &Config{}
			url := config.GetURL()
			gomega.Expect(url.Host).To(gomega.BeEmpty())
			gomega.Expect(url.User.Username()).To(gomega.BeEmpty())
		})

		ginkgo.It("should return error for missing webhook ID", func() {
			url, _ := url.Parse("discord://token@")
			config := &Config{}
			err := config.SetURL(url)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrMissingWebhookID))
		})

		ginkgo.It("should return error for missing token", func() {
			url, _ := url.Parse("discord://@webhook")
			config := &Config{}
			err := config.SetURL(url)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrMissingToken))
		})

		ginkgo.It("should return error for invalid path", func() {
			url, _ := url.Parse("discord://token@webhook/invalid")
			config := &Config{}
			err := config.SetURL(url)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrIllegalURLArgument))
		})

		ginkgo.It("should return error for invalid query parameters", func() {
			url, _ := url.Parse("discord://token@webhook?invalid_key=value")
			config := &Config{}
			resolver := format.NewPropKeyResolver(config)
			err := resolver.SetDefaultProps(config)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(url)
			gomega.Expect(err).To(gomega.HaveOccurred()) // Should error on unknown keys
		})
	})

	ginkgo.Describe("CreateAPIURLFromConfig function", func() {
		ginkgo.It("should create correct API URL without thread ID", func() {
			config := &Config{
				WebhookID: "123456789",
				Token:     "test-token",
			}
			apiURL := CreateAPIURLFromConfig(config)
			expected := testWebhookURL
			gomega.Expect(apiURL).To(gomega.Equal(expected))
		})

		ginkgo.It("should create correct API URL with thread ID", func() {
			config := &Config{
				WebhookID: "123456789",
				Token:     "test-token",
				ThreadID:  "987654321",
			}
			apiURL := CreateAPIURLFromConfig(config)
			expected := testWebhookURLWithThread
			gomega.Expect(apiURL).To(gomega.Equal(expected))
		})

		ginkgo.It("should trim whitespace from webhook ID and token", func() {
			config := &Config{
				WebhookID: "  123456789  ",
				Token:     "  test-token  ",
			}
			apiURL := CreateAPIURLFromConfig(config)
			expected := testWebhookURL
			gomega.Expect(apiURL).To(gomega.Equal(expected))
		})

		ginkgo.It("should trim whitespace from thread ID", func() {
			config := &Config{
				WebhookID: "123456789",
				Token:     "test-token",
				ThreadID:  "  987654321  ",
			}
			apiURL := CreateAPIURLFromConfig(config)
			expected := testWebhookURLWithThread
			gomega.Expect(apiURL).To(gomega.Equal(expected))
		})

		ginkgo.It("should return empty string for invalid config", func() {
			config := &Config{
				WebhookID: "",
				Token:     "",
			}
			apiURL := CreateAPIURLFromConfig(config)
			gomega.Expect(apiURL).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle empty thread ID", func() {
			config := &Config{
				WebhookID: "123456789",
				Token:     "test-token",
				ThreadID:  "",
			}
			apiURL := CreateAPIURLFromConfig(config)
			expected := testWebhookURL
			gomega.Expect(apiURL).To(gomega.Equal(expected))
		})

		ginkgo.It("should handle very long webhook ID and token", func() {
			longID := strings.Repeat("1", 100)
			longToken := strings.Repeat("a", 100)
			config := &Config{
				WebhookID: longID,
				Token:     longToken,
			}
			apiURL := CreateAPIURLFromConfig(config)
			expected := "https://discord.com/api/webhooks/" + longID + "/" + longToken
			gomega.Expect(apiURL).To(gomega.Equal(expected))
		})
	})
})
