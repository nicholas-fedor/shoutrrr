package ntfy

import (
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

var _ = ginkgo.Describe("Config", func() {
	var config *Config

	ginkgo.BeforeEach(func() {
		config = &Config{}
	})

	ginkgo.Describe("Enums", func() {
		ginkgo.It("should return a map with Priority enum formatter", func() {
			enums := config.Enums()
			gomega.Expect(enums).To(gomega.HaveKey("Priority"))
			gomega.Expect(enums["Priority"]).NotTo(gomega.BeNil())
		})
	})

	ginkgo.Describe("GetAPIURL", func() {
		ginkgo.It("should construct URL with https scheme by default", func() {
			config.Scheme = "https"
			config.Host = "ntfy.sh"
			config.Topic = "mytopic"

			result := config.GetAPIURL()
			gomega.Expect(result).To(gomega.Equal("https://ntfy.sh/mytopic"))
		})

		ginkgo.It("should not double-slash when topic already has leading slash", func() {
			config.Scheme = "https"
			config.Host = "ntfy.sh"
			config.Topic = "/mytopic"

			result := config.GetAPIURL()
			gomega.Expect(result).To(gomega.Equal("https://ntfy.sh/mytopic"))
		})

		ginkgo.It("should handle nested topic paths", func() {
			config.Scheme = "https"
			config.Host = "ntfy.sh"
			config.Topic = "notifications/alerts"

			result := config.GetAPIURL()
			gomega.Expect(result).To(gomega.Equal("https://ntfy.sh/notifications/alerts"))
		})

		ginkgo.It("should use http scheme when configured", func() {
			config.Scheme = "http"
			config.Host = "ntfy.local"
			config.Topic = "mytopic"

			result := config.GetAPIURL()
			gomega.Expect(result).To(gomega.Equal("http://ntfy.local/mytopic"))
		})
	})

	ginkgo.Describe("GetURL", func() {
		ginkgo.It("should build URL with all fields populated", func() {
			config.Scheme = "https"
			config.Host = "ntfy.sh"
			config.Topic = "mytopic"
			config.Username = "user"
			config.Password = "pass"
			config.Title = "Alert"
			config.Priority = PriorityHigh

			result := config.GetURL()
			gomega.Expect(result).NotTo(gomega.BeNil())
			gomega.Expect(result.Scheme).To(gomega.Equal(Scheme))
			gomega.Expect(result.Host).To(gomega.Equal("ntfy.sh"))
			gomega.Expect(result.Path).To(gomega.Equal("/mytopic"))
			gomega.Expect(result.User.Username()).To(gomega.Equal("user"))
			pass, hasPass := result.User.Password()
			gomega.Expect(hasPass).To(gomega.BeTrue())
			gomega.Expect(pass).To(gomega.Equal("pass"))
			gomega.Expect(result.Query().Get("title")).To(gomega.Equal("Alert"))
			gomega.Expect(result.Query().Get("priority")).To(gomega.Equal("High"))
		})

		ginkgo.It("should build URL without credentials when empty", func() {
			config.Scheme = "https"
			config.Host = "ntfy.sh"
			config.Topic = "mytopic"

			result := config.GetURL()
			gomega.Expect(result).NotTo(gomega.BeNil())
			gomega.Expect(result.User).To(gomega.BeNil())
		})

		ginkgo.It("should build URL with username only", func() {
			config.Scheme = "https"
			config.Host = "ntfy.sh"
			config.Topic = "mytopic"
			config.Username = "user"

			result := config.GetURL()
			gomega.Expect(result).NotTo(gomega.BeNil())
			gomega.Expect(result.User.Username()).To(gomega.Equal("user"))
			_, hasPass := result.User.Password()
			gomega.Expect(hasPass).To(gomega.BeFalse())
		})

		ginkgo.It("should force query string even with no params", func() {
			config.Scheme = "https"
			config.Host = "ntfy.sh"
			config.Topic = "mytopic"

			result := config.GetURL()
			gomega.Expect(result).NotTo(gomega.BeNil())
			gomega.Expect(result.ForceQuery).To(gomega.BeTrue())
		})
	})

	ginkgo.Describe("QueryFields", func() {
		ginkgo.It("should return non-empty list of query fields", func() {
			fields := config.QueryFields()
			gomega.Expect(fields).NotTo(gomega.BeEmpty())
		})

		ginkgo.It("should contain priority field", func() {
			fields := config.QueryFields()
			gomega.Expect(fields).To(gomega.ContainElement("priority"))
		})

		ginkgo.It("should contain title field", func() {
			fields := config.QueryFields()
			gomega.Expect(fields).To(gomega.ContainElement("title"))
		})
	})

	ginkgo.Describe("SetURL", func() {
		ginkgo.It("should parse URL components correctly", func() {
			testURL := mustParseURL("ntfy://user:pass@ntfy.example.com/mytopic?priority=5")

			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Host).To(gomega.Equal("ntfy.example.com"))
			gomega.Expect(config.Topic).To(gomega.Equal("mytopic"))
			gomega.Expect(config.Username).To(gomega.Equal("user"))
			gomega.Expect(config.Password).To(gomega.Equal("pass"))
			gomega.Expect(config.Priority).To(gomega.Equal(PriorityMax))
		})

		ginkgo.It("should parse URL without credentials", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/mytopic")

			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Username).To(gomega.Equal(""))
			gomega.Expect(config.Password).To(gomega.Equal(""))
			gomega.Expect(config.Host).To(gomega.Equal("ntfy.example.com"))
			gomega.Expect(config.Topic).To(gomega.Equal("mytopic"))
		})

		ginkgo.It("should parse URL with username only", func() {
			testURL := mustParseURL("ntfy://user@ntfy.example.com/mytopic")

			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Username).To(gomega.Equal("user"))
			gomega.Expect(config.Password).To(gomega.Equal(""))
		})

		ginkgo.It("should return ErrTopicRequired for empty topic", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/")

			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.Equal(ErrTopicRequired))
		})

		ginkgo.It("should skip topic validation for dummy URL", func() {
			testURL := mustParseURL("ntfy://dummy@dummy.com")

			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should parse nested topic paths", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/level1/level2/level3")

			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Topic).To(gomega.Equal("level1/level2/level3"))
		})

		ginkgo.It("should parse query parameters", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/mytopic?title=Alert&priority=High&tags=warning,skull")

			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Title).To(gomega.Equal("Alert"))
			gomega.Expect(config.Priority).To(gomega.Equal(PriorityHigh))
			gomega.Expect(config.Tags).To(gomega.Equal([]string{"warning", "skull"}))
		})

		ginkgo.It("should handle semicolons in query by splitting actions", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/mytopic?actions=view;open")

			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Actions).To(gomega.Equal([]string{"view", "open"}))
		})

		ginkgo.It("should parse markdown query parameter", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/mytopic?markdown=yes")

			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Markdown).To(gomega.BeTrue())
		})

		ginkgo.It("should parse cache query parameter as No", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/mytopic?cache=no")

			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Cache).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("getURL", func() {
		ginkgo.It("should construct URL with ntfy scheme", func() {
			config.Scheme = "https"
			config.Host = "ntfy.sh"
			config.Topic = "mytopic"
			config.Username = "user"
			config.Password = "pass"

			resolver := format.NewPropKeyResolver(config)
			result := config.getURL(&resolver)
			gomega.Expect(result).NotTo(gomega.BeNil())
			gomega.Expect(result.Scheme).To(gomega.Equal(Scheme))
			gomega.Expect(result.Host).To(gomega.Equal("ntfy.sh"))
			gomega.Expect(result.Path).To(gomega.Equal("/mytopic"))
			gomega.Expect(result.User.Username()).To(gomega.Equal("user"))
			pass, hasPass := result.User.Password()
			gomega.Expect(hasPass).To(gomega.BeTrue())
			gomega.Expect(pass).To(gomega.Equal("pass"))
		})

		ginkgo.It("should include username without password", func() {
			config.Scheme = "https"
			config.Host = "ntfy.sh"
			config.Topic = "mytopic"
			config.Username = "user"

			resolver := format.NewPropKeyResolver(config)
			result := config.getURL(&resolver)
			gomega.Expect(result).NotTo(gomega.BeNil())
			gomega.Expect(result.User.Username()).To(gomega.Equal("user"))
			_, hasPass := result.User.Password()
			gomega.Expect(hasPass).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("setURL", func() {
		ginkgo.It("should set topic from URL path without leading slash", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/mytopic")

			resolver := format.NewPropKeyResolver(config)
			err := config.setURL(&resolver, testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Topic).To(gomega.Equal("mytopic"))
		})

		ginkgo.It("should set topic from URL path with leading slash", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com//mytopic")

			resolver := format.NewPropKeyResolver(config)
			err := config.setURL(&resolver, testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Topic).To(gomega.Equal("/mytopic"))
		})

		ginkgo.It("should clear credentials when URL has no user info", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/mytopic")

			config.Username = "olduser"
			config.Password = "oldpass"

			resolver := format.NewPropKeyResolver(config)
			err := config.setURL(&resolver, testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Username).To(gomega.Equal(""))
			gomega.Expect(config.Password).To(gomega.Equal(""))
		})

		ginkgo.It("should set host from URL", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com:8080/mytopic")

			resolver := format.NewPropKeyResolver(config)
			err := config.setURL(&resolver, testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Host).To(gomega.Equal("ntfy.example.com:8080"))
		})

		ginkgo.It("should split semicolons in actions query", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/mytopic?actions=a;b;c")
			gomega.Expect(strings.Contains(testURL.RawQuery, ";")).To(gomega.BeTrue())

			resolver := format.NewPropKeyResolver(config)
			err := config.setURL(&resolver, testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Actions).To(gomega.Equal([]string{"a", "b", "c"}))
		})

		ginkgo.It("should return ErrTopicRequired for empty topic except dummy URL", func() {
			testURL := mustParseURL("ntfy://ntfy.example.com/")

			resolver := format.NewPropKeyResolver(config)
			err := config.setURL(&resolver, testURL)
			gomega.Expect(err).To(gomega.Equal(ErrTopicRequired))
		})

		ginkgo.It("should skip topic validation for dummy URL", func() {
			testURL := mustParseURL("ntfy://dummy@dummy.com")

			resolver := format.NewPropKeyResolver(config)
			err := config.setURL(&resolver, testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("default values", func() {
		ginkgo.It("should apply defaults from struct tags when only required fields are set", func() {
			svc := &Service{}
			serviceURL := mustParseURL("ntfy://hostname/topic")
			gomega.Expect(svc.Initialize(serviceURL, &noOpLogger{})).To(gomega.Succeed())
			gomega.Expect(svc.Config.Host).To(gomega.Equal("hostname"))
			gomega.Expect(svc.Config.Topic).To(gomega.Equal("topic"))
			gomega.Expect(svc.Config.Scheme).To(gomega.Equal("https"))
			gomega.Expect(svc.Config.Tags).To(gomega.Equal([]string{""}))
			gomega.Expect(svc.Config.Actions).To(gomega.Equal([]string{""}))
			gomega.Expect(svc.Config.Priority).To(gomega.Equal(PriorityDefault))
			gomega.Expect(svc.Config.Markdown).To(gomega.BeFalse())
			gomega.Expect(svc.Config.Firebase).To(gomega.BeTrue())
			gomega.Expect(svc.Config.Cache).To(gomega.BeTrue())
			gomega.Expect(svc.Config.DisableTLSVerification).To(gomega.BeFalse())
			gomega.Expect(svc.Config.DisableTLS).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("URL round-trip", func() {
		ginkgo.It("should be identical after de-/serialization", func() {
			testURL := "ntfy://user:pass@example.com:2225/topic?cache=No&click=CLICK&disabletls=No&disabletlsverification=No&firebase=No&icon=ICON&markdown=No&priority=Max&scheme=http&title=TITLE"
			cfg := &Config{}
			pkr := format.NewPropKeyResolver(cfg)
			gomega.Expect(cfg.setURL(&pkr, mustParseURL(testURL))).To(gomega.Succeed())
			gomega.Expect(cfg.GetURL().String()).To(gomega.Equal(testURL))
		})
	})

	ginkgo.Describe("Service API compliance", func() {
		ginkgo.It("should pass standard config tests", func() {
			cfg := &Config{}
			enums := cfg.Enums()
			gomega.Expect(enums).To(gomega.HaveKey("Priority"))

			resolver := format.NewPropKeyResolver(cfg)
			fields := resolver.QueryFields()
			gomega.Expect(fields).To(gomega.HaveLen(18))
		})
	})
})
