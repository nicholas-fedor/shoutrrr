package zulip

import (
	"net/url"

	"github.com/onsi/gomega"

	ginkgo "github.com/onsi/ginkgo/v2"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
)

var _ = ginkgo.Describe("Config", func() {
	ginkgo.Describe("Clone", func() {
		ginkgo.It("should create an independent copy with all fields preserved", func() {
			original := &Config{
				BotMail:      "bot@example.com",
				BotKey:       "secret-key",
				Host:         "zulip.example.com",
				Type:         MessageTypeChannel,
				Stream:       "general",
				Topic:        "announcements",
				Title:        "Deployment",
				To:           "user1@example.com,user2@example.com",
				ReadBySender: true,
			}

			clone := original.Clone()

			gomega.Expect(clone).NotTo(gomega.BeNil())
			gomega.Expect(clone.BotMail).To(gomega.Equal(original.BotMail))
			gomega.Expect(clone.BotKey).To(gomega.Equal(original.BotKey))
			gomega.Expect(clone.Host).To(gomega.Equal(original.Host))
			gomega.Expect(clone.Type).To(gomega.Equal(original.Type))
			gomega.Expect(clone.Stream).To(gomega.Equal(original.Stream))
			gomega.Expect(clone.Topic).To(gomega.Equal(original.Topic))
			gomega.Expect(clone.Title).To(gomega.Equal(original.Title))
			gomega.Expect(clone.To).To(gomega.Equal(original.To))
			gomega.Expect(clone.ReadBySender).To(gomega.Equal(original.ReadBySender))
		})

		ginkgo.It("should create a distinct pointer from the original", func() {
			original := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			clone := original.Clone()

			gomega.Expect(clone).NotTo(gomega.BeIdenticalTo(original))
		})

		ginkgo.It("should handle empty config", func() {
			original := &Config{}

			clone := original.Clone()

			gomega.Expect(clone).NotTo(gomega.BeNil())
			gomega.Expect(clone.BotMail).To(gomega.BeEmpty())
			gomega.Expect(clone.BotKey).To(gomega.BeEmpty())
			gomega.Expect(clone.Host).To(gomega.BeEmpty())
			gomega.Expect(clone.Type).To(gomega.BeEmpty())
			gomega.Expect(clone.Stream).To(gomega.BeEmpty())
			gomega.Expect(clone.Topic).To(gomega.BeEmpty())
			gomega.Expect(clone.Title).To(gomega.BeEmpty())
			gomega.Expect(clone.To).To(gomega.BeEmpty())
			gomega.Expect(clone.ReadBySender).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("GetURL", func() {
		ginkgo.It("should generate URL with scheme, host, user, and password", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			resultURL := cfg.GetURL()

			gomega.Expect(resultURL.Scheme).To(gomega.Equal(Scheme))
			gomega.Expect(resultURL.Host).To(gomega.Equal("zulip.example.com"))
			gomega.Expect(resultURL.User.Username()).To(gomega.Equal("bot@example.com"))
			password, isSet := resultURL.User.Password()
			gomega.Expect(isSet).To(gomega.BeTrue())
			gomega.Expect(password).To(gomega.Equal("secret-key"))
		})

		ginkgo.It("should include stream as query parameter when set", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
				Stream:  "general",
			}

			resultURL := cfg.GetURL()

			gomega.Expect(resultURL.Query().Get("stream")).To(gomega.Equal("general"))
		})

		ginkgo.It("should include topic as query parameter when set", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
				Topic:   "announcements",
			}

			resultURL := cfg.GetURL()

			gomega.Expect(resultURL.Query().Get("topic")).To(gomega.Equal("announcements"))
		})

		ginkgo.It("should include both stream and topic when both are set", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
				Stream:  "general",
				Topic:   "announcements",
			}

			resultURL := cfg.GetURL()

			gomega.Expect(resultURL.Query().Get("stream")).To(gomega.Equal("general"))
			gomega.Expect(resultURL.Query().Get("topic")).To(gomega.Equal("announcements"))
		})

		ginkgo.It("should omit stream and topic query parameters when empty", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			resultURL := cfg.GetURL()

			gomega.Expect(resultURL.Query().Get("stream")).To(gomega.BeEmpty())
			gomega.Expect(resultURL.Query().Get("topic")).To(gomega.BeEmpty())
		})

		ginkgo.It("should include type, to, and read_by_sender when set for direct message", func() {
			cfg := &Config{
				BotMail:      "bot@example.com",
				BotKey:       "secret-key",
				Host:         "zulip.example.com",
				Type:         MessageTypeDirect,
				To:           "user1@example.com,user2@example.com",
				ReadBySender: true,
			}

			resultURL := cfg.GetURL()

			gomega.Expect(resultURL.Query().Get("type")).To(gomega.Equal("direct"))
			gomega.Expect(resultURL.Query().Get("to")).To(gomega.Equal("user1@example.com,user2@example.com"))
			gomega.Expect(resultURL.Query().Get("read_by_sender")).To(gomega.Equal("true"))
		})

		ginkgo.It("should omit read_by_sender when false", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
				Stream:  "general",
			}

			resultURL := cfg.GetURL()

			gomega.Expect(resultURL.Query().Get("read_by_sender")).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("SetURL", func() {
		ginkgo.It("should parse a valid URL with all fields", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=announcements")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cfg.BotMail).To(gomega.Equal("bot@example.com"))
			gomega.Expect(cfg.BotKey).To(gomega.Equal("secret-key"))
			gomega.Expect(cfg.Host).To(gomega.Equal("zulip.example.com"))
			gomega.Expect(cfg.Stream).To(gomega.Equal("general"))
			gomega.Expect(cfg.Topic).To(gomega.Equal("announcements"))
		})

		ginkgo.It("should parse a valid URL with only required fields", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cfg.BotMail).To(gomega.Equal("bot@example.com"))
			gomega.Expect(cfg.BotKey).To(gomega.Equal("secret-key"))
			gomega.Expect(cfg.Host).To(gomega.Equal("zulip.example.com"))
			gomega.Expect(cfg.Stream).To(gomega.BeEmpty())
			gomega.Expect(cfg.Topic).To(gomega.BeEmpty())
		})

		ginkgo.It("should parse type, to, and read_by_sender from URL for direct message", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com?type=direct&to=user1@example.com,user2@example.com&read_by_sender=true")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cfg.Type).To(gomega.Equal(MessageTypeDirect))
			gomega.Expect(cfg.To).To(gomega.Equal("user1@example.com,user2@example.com"))
			gomega.Expect(cfg.ReadBySender).To(gomega.BeTrue())
		})

		ginkgo.It("should default read_by_sender to false when not set to true", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com?type=channel&stream=general")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cfg.ReadBySender).To(gomega.BeFalse())
		})

		ginkgo.It("should return ErrMissingBotMail when bot mail is empty", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://:secret-key@zulip.example.com")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).To(gomega.MatchError(ErrMissingBotMail))
		})

		ginkgo.It("should return ErrMissingAPIKey when API key is missing", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com@zulip.example.com")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).To(gomega.MatchError(ErrMissingAPIKey))
		})

		ginkgo.It("should return ErrMissingHost when host is empty", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).To(gomega.MatchError(ErrMissingHost))
		})

		ginkgo.It("should accept dummy URL without validation errors", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://dummy@dummy.com")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("getURL", func() {
		ginkgo.It("should construct URL with scheme and host from config", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			resultURL := cfg.getURL(nil)

			gomega.Expect(resultURL.Scheme).To(gomega.Equal(Scheme))
			gomega.Expect(resultURL.Host).To(gomega.Equal("zulip.example.com"))
		})

		ginkgo.It("should set user info from config", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			resultURL := cfg.getURL(nil)

			gomega.Expect(resultURL.User.Username()).To(gomega.Equal("bot@example.com"))
			password, isSet := resultURL.User.Password()
			gomega.Expect(isSet).To(gomega.BeTrue())
			gomega.Expect(password).To(gomega.Equal("secret-key"))
		})

		ginkgo.It("should add stream query parameter when stream is set", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
				Stream:  "general",
			}

			resultURL := cfg.getURL(nil)

			gomega.Expect(resultURL.Query().Get("stream")).To(gomega.Equal("general"))
		})

		ginkgo.It("should add topic query parameter when topic is set", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
				Topic:   "announcements",
			}

			resultURL := cfg.getURL(nil)

			gomega.Expect(resultURL.Query().Get("topic")).To(gomega.Equal("announcements"))
		})

		ginkgo.It("should not add query parameters when stream and topic are empty", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			resultURL := cfg.getURL(nil)

			gomega.Expect(resultURL.RawQuery).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("setURL", func() {
		ginkgo.It("should populate all fields from a complete URL", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=announcements")

			err := cfg.setURL(nil, serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cfg.BotMail).To(gomega.Equal("bot@example.com"))
			gomega.Expect(cfg.BotKey).To(gomega.Equal("secret-key"))
			gomega.Expect(cfg.Host).To(gomega.Equal("zulip.example.com"))
			gomega.Expect(cfg.Stream).To(gomega.Equal("general"))
			gomega.Expect(cfg.Topic).To(gomega.Equal("announcements"))
			gomega.Expect(cfg.Type).To(gomega.BeEmpty())
			gomega.Expect(cfg.To).To(gomega.BeEmpty())
			gomega.Expect(cfg.ReadBySender).To(gomega.BeFalse())
		})

		ginkgo.It("should return ErrMissingBotMail when bot mail is empty", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://:secret-key@zulip.example.com")

			err := cfg.setURL(nil, serviceURL)

			gomega.Expect(err).To(gomega.MatchError(ErrMissingBotMail))
		})

		ginkgo.It("should return ErrMissingAPIKey when password is not set", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com@zulip.example.com")

			err := cfg.setURL(nil, serviceURL)

			gomega.Expect(err).To(gomega.MatchError(ErrMissingAPIKey))
		})

		ginkgo.It("should return ErrMissingHost when host is empty", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@")

			err := cfg.setURL(nil, serviceURL)

			gomega.Expect(err).To(gomega.MatchError(ErrMissingHost))
		})

		ginkgo.It("should skip validation for dummy URLs", func() {
			cfg := &Config{}
			serviceURL := &url.URL{
				Scheme: "zulip",
				Host:   "dummy.com",
				User:   url.User("dummy"),
			}

			err := cfg.setURL(nil, serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("CreateConfigFromURL", func() {
		ginkgo.It("should create a valid config from a complete URL", func() {
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=announcements")

			config, err := CreateConfigFromURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config).NotTo(gomega.BeNil())
			gomega.Expect(config.BotMail).To(gomega.Equal("bot@example.com"))
			gomega.Expect(config.BotKey).To(gomega.Equal("secret-key"))
			gomega.Expect(config.Host).To(gomega.Equal("zulip.example.com"))
			gomega.Expect(config.Stream).To(gomega.Equal("general"))
			gomega.Expect(config.Topic).To(gomega.Equal("announcements"))
			gomega.Expect(config.Type).To(gomega.BeEmpty())
			gomega.Expect(config.To).To(gomega.BeEmpty())
			gomega.Expect(config.ReadBySender).To(gomega.BeFalse())
		})

		ginkgo.It("should create a valid config from URL with direct message fields", func() {
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com?type=direct&to=user@example.com&read_by_sender=true")

			config, err := CreateConfigFromURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Type).To(gomega.Equal(MessageTypeDirect))
			gomega.Expect(config.To).To(gomega.Equal("user@example.com"))
			gomega.Expect(config.ReadBySender).To(gomega.BeTrue())
		})

		ginkgo.It("should return an error for invalid URL", func() {
			serviceURL := testutils.URLMust("zulip://:secret-key@zulip.example.com")

			config, err := CreateConfigFromURL(serviceURL)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(config).NotTo(gomega.BeNil())
		})

		ginkgo.It("should create config with only required fields", func() {
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com")

			config, err := CreateConfigFromURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Stream).To(gomega.BeEmpty())
			gomega.Expect(config.Topic).To(gomega.BeEmpty())
			gomega.Expect(config.Type).To(gomega.BeEmpty())
			gomega.Expect(config.To).To(gomega.BeEmpty())
			gomega.Expect(config.ReadBySender).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("isDummyURL", func() {
		ginkgo.It("should return true for the dummy URL", func() {
			dummyURL := &url.URL{
				Scheme: "zulip",
				Host:   "dummy.com",
				User:   url.User("dummy"),
			}

			gomega.Expect(isDummyURL(dummyURL)).To(gomega.BeTrue())
		})

		ginkgo.It("should return false for a regular URL", func() {
			regularURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com")

			gomega.Expect(isDummyURL(regularURL)).To(gomega.BeFalse())
		})

		ginkgo.It("should return false when host matches but user does not", func() {
			url1 := &url.URL{
				Scheme: "zulip",
				Host:   "dummy.com",
				User:   url.User("other"),
			}

			gomega.Expect(isDummyURL(url1)).To(gomega.BeFalse())
		})

		ginkgo.It("should return false when user matches but host does not", func() {
			url1 := &url.URL{
				Scheme: "zulip",
				Host:   "zulip.example.com",
				User:   url.User("dummy"),
			}

			gomega.Expect(isDummyURL(url1)).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("Port preservation", func() {
		ginkgo.It("should preserve non-standard port in host", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com:8443?stream=general")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cfg.Host).To(gomega.Equal("zulip.example.com:8443"))
		})

		ginkgo.It("should preserve standard port in host", func() {
			cfg := &Config{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com:443")

			err := cfg.SetURL(serviceURL)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(cfg.Host).To(gomega.Equal("zulip.example.com:443"))
		})

		ginkgo.It("should include port in GetURL round-trip", func() {
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com:8443",
				Stream:  "general",
				Topic:   "alerts",
			}

			resultURL := cfg.GetURL()

			gomega.Expect(resultURL.Host).To(gomega.Equal("zulip.example.com:8443"))
		})
	})
})
