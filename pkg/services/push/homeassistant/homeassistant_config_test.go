package homeassistant

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

var _ = ginkgo.Describe("Config", func() {
	var (
		config *Config
		pkr    format.PropKeyResolver
	)

	ginkgo.BeforeEach(func() {
		config = &Config{}
		pkr = format.NewPropKeyResolver(config)
		gomega.Expect(pkr.SetDefaultProps(config)).To(gomega.Succeed())
	})

	ginkgo.Describe("SetURL", func() {
		ginkgo.It("should parse token from userinfo and host from hostname", func() {
			err := config.SetURL(mustParseURL("homeassistant://s3cret@ha.example.com"))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Token).To(gomega.Equal("s3cret"))
			gomega.Expect(config.Host).To(gomega.Equal("ha.example.com"))
			gomega.Expect(config.Port).To(gomega.Equal(0))
			gomega.Expect(config.Path).To(gomega.BeEmpty())
			gomega.Expect(config.DisableTLS).To(gomega.BeFalse())
		})

		ginkgo.It("should parse an explicit port and path prefix", func() {
			err := config.SetURL(mustParseURL(
				"homeassistant://s3cret@ha.example.com:8123/hass/?disabletls=yes",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Port).To(gomega.Equal(8123))
			gomega.Expect(config.Path).To(gomega.Equal("/hass"))
			gomega.Expect(config.DisableTLS).To(gomega.BeTrue())
		})

		ginkgo.It("should parse title, service, targets, nid, and skiptlsverify", func() {
			err := config.SetURL(mustParseURL(
				"homeassistant://s3cret@ha.example.com" +
					"?title=Update&service=notify.mobile_app_phone&targets=device1,device2" +
					"&nid=watchtower&skiptlsverify=yes",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Title).To(gomega.Equal("Update"))
			gomega.Expect(config.Service).To(gomega.Equal("notify.mobile_app_phone"))
			gomega.Expect(config.Targets).To(gomega.Equal([]string{"device1", "device2"}))
			gomega.Expect(config.Nid).To(gomega.Equal("watchtower"))
			gomega.Expect(config.SkipTLSVerify).To(gomega.BeTrue())
		})

		ginkgo.It("should reject a missing token", func() {
			err := config.SetURL(mustParseURL("homeassistant://ha.example.com"))
			gomega.Expect(err).To(gomega.MatchError(ErrTokenMissing))
		})

		ginkgo.It("should reject a missing host", func() {
			err := config.SetURL(mustParseURL("homeassistant://s3cret@"))
			gomega.Expect(err).To(gomega.MatchError(ErrHostMissing))
		})

		ginkgo.It("should reject an invalid port", func() {
			err := config.SetURL(mustParseURL("homeassistant://s3cret@ha.example.com:99999"))
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidPort))
		})

		ginkgo.It("should reject an invalid service mapping", func() {
			err := config.SetURL(mustParseURL("homeassistant://s3cret@ha.example.com?service=notify."))
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidService))
		})

		ginkgo.It("should reject a service that would escape the API path", func() {
			err := config.SetURL(mustParseURL(
				"homeassistant://s3cret@ha.example.com?service=/api/events/foo",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidService))

			err = config.SetURL(mustParseURL(
				"homeassistant://s3cret@ha.example.com?service=../config",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidService))
		})

		ginkgo.It("should accept the docs dummy URL", func() {
			err := config.SetURL(mustParseURL(dummyServiceURL))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("GetURL", func() {
		ginkgo.It("should round-trip token, host, path, and query fields", func() {
			config.Token = "s3cret"
			config.Host = "ha.example.com"
			config.Port = 8443
			config.Path = "/hass"
			config.Title = "Update"
			config.Service = "notify.mobile_app_phone"
			config.Targets = []string{"device1"}
			config.Nid = "watchtower"
			config.DisableTLS = true

			got := config.GetURL()
			gomega.Expect(got.Scheme).To(gomega.Equal(Scheme))
			gomega.Expect(got.User.Username()).To(gomega.Equal("s3cret"))
			gomega.Expect(got.Hostname()).To(gomega.Equal("ha.example.com"))
			gomega.Expect(got.Path).To(gomega.Equal("/hass"))

			roundTrip := &Config{}
			gomega.Expect(roundTrip.SetURL(got)).To(gomega.Succeed())
			gomega.Expect(roundTrip.Token).To(gomega.Equal("s3cret"))
			gomega.Expect(roundTrip.Host).To(gomega.Equal("ha.example.com"))
			gomega.Expect(roundTrip.Port).To(gomega.Equal(8443))
			gomega.Expect(roundTrip.Path).To(gomega.Equal("/hass"))
			gomega.Expect(roundTrip.Title).To(gomega.Equal("Update"))
			gomega.Expect(roundTrip.Service).To(gomega.Equal("notify.mobile_app_phone"))
			gomega.Expect(roundTrip.Targets).To(gomega.Equal([]string{"device1"}))
			gomega.Expect(roundTrip.Nid).To(gomega.Equal("watchtower"))
			gomega.Expect(roundTrip.DisableTLS).To(gomega.BeTrue())
		})

		ginkgo.It("should omit the implied HTTPS port", func() {
			config.Token = "s3cret"
			config.Host = "ha.example.com"
			config.Port = 443

			got := config.GetURL()
			gomega.Expect(got.Host).To(gomega.Equal("ha.example.com"))
			gomega.Expect(got.Port()).To(gomega.BeEmpty())
		})

		ginkgo.It("should omit the implied HTTP port when TLS is disabled", func() {
			config.Token = "s3cret"
			config.Host = "homeassistant.local"
			config.Port = 8123
			config.DisableTLS = true

			got := config.GetURL()
			gomega.Expect(got.Host).To(gomega.Equal("homeassistant.local"))
			gomega.Expect(got.Port()).To(gomega.BeEmpty())
		})

		ginkgo.It("should keep a non-default HTTPS port", func() {
			config.Token = "s3cret"
			config.Host = "homeassistant.local"
			config.Port = 8123

			got := config.GetURL()
			gomega.Expect(got.Host).To(gomega.Equal("homeassistant.local:8123"))
		})
	})

	ginkgo.Describe("domainService", func() {
		ginkgo.It("should default to persistent_notification.create", func() {
			domain, service, err := config.domainService()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(domain).To(gomega.Equal(persistentDomain))
			gomega.Expect(service).To(gomega.Equal(persistentService))
			gomega.Expect(config.isPersistent()).To(gomega.BeTrue())
		})

		ginkgo.It("should map a bare name onto the notify domain", func() {
			config.Service = "mobile_app_phone"
			domain, service, err := config.domainService()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(domain).To(gomega.Equal(notifyDomain))
			gomega.Expect(service).To(gomega.Equal("mobile_app_phone"))
			gomega.Expect(config.isPersistent()).To(gomega.BeFalse())
		})

		ginkgo.It("should split a domain.action value", func() {
			config.Service = "notify.mobile_app_phone"
			domain, service, err := config.domainService()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(domain).To(gomega.Equal("notify"))
			gomega.Expect(service).To(gomega.Equal("mobile_app_phone"))
		})
	})

	ginkgo.Describe("apiURL", func() {
		ginkgo.It("should use HTTPS port 443 when the port is omitted", func() {
			config.Host = "ha.example.com"
			gomega.Expect(config.apiURL(persistentDomain, persistentService)).To(
				gomega.Equal("https://ha.example.com:443/api/services/persistent_notification/create"),
			)
		})

		ginkgo.It("should use HTTP port 8123 when TLS is disabled and the port is omitted", func() {
			config.Host = "homeassistant.local"
			config.DisableTLS = true
			gomega.Expect(config.apiURL(persistentDomain, persistentService)).To(
				gomega.Equal(
					"http://homeassistant.local:8123/api/services/persistent_notification/create",
				),
			)
		})

		ginkgo.It("should include a reverse-proxy prefix", func() {
			config.Host = "ha.example.com"
			config.Path = "/hass"
			gomega.Expect(config.apiURL("notify", "mobile_app_phone")).To(
				gomega.Equal("https://ha.example.com:443/hass/api/services/notify/mobile_app_phone"),
			)
		})
	})
})
