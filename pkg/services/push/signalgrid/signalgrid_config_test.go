package signalgrid

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
		ginkgo.It("should parse client key from userinfo and channel from host", func() {
			err := config.SetURL(mustParseURL("signalgrid://abc123@channel"))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.ClientKey).To(gomega.Equal("abc123"))
			gomega.Expect(config.Channel).To(gomega.Equal("channel"))
		})

		ginkgo.It("should parse title, type, and critical query parameters", func() {
			err := config.SetURL(mustParseURL(
				"signalgrid://key@channel?title=Server%20Down&type=CRIT&critical=true",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Title).To(gomega.Equal("Server Down"))
			gomega.Expect(config.Type).To(gomega.Equal(TypeCRIT))
			gomega.Expect(config.Critical).To(gomega.BeTrue())
		})

		ginkgo.It("should accept lowercase type aliases", func() {
			err := config.SetURL(mustParseURL("signalgrid://key@channel?type=info"))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Type).To(gomega.Equal(TypeINFO))
		})

		ginkgo.It("should reject an invalid type", func() {
			err := config.SetURL(mustParseURL("signalgrid://key@channel?type=nope"))
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should reject a missing client key", func() {
			err := config.SetURL(mustParseURL("signalgrid://channel"))
			gomega.Expect(err).To(gomega.MatchError(ErrClientKeyMissing))
		})

		ginkgo.It("should reject a missing channel", func() {
			err := config.SetURL(mustParseURL("signalgrid://key@"))
			gomega.Expect(err).To(gomega.MatchError(ErrChannelMissing))
		})

		ginkgo.It("should accept the docs dummy URL", func() {
			err := config.SetURL(mustParseURL(dummyServiceURL))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("GetURL", func() {
		ginkgo.It("should round-trip client key and channel", func() {
			config.ClientKey = "abc123"
			config.Channel = "channel"
			config.Title = "Alert"
			config.Type = TypeCRIT
			config.Critical = true

			got := config.GetURL()
			gomega.Expect(got.Scheme).To(gomega.Equal(Scheme))
			gomega.Expect(got.User.Username()).To(gomega.Equal("abc123"))
			gomega.Expect(got.Host).To(gomega.Equal("channel"))

			roundTrip := &Config{}
			gomega.Expect(roundTrip.SetURL(got)).To(gomega.Succeed())
			gomega.Expect(roundTrip.ClientKey).To(gomega.Equal("abc123"))
			gomega.Expect(roundTrip.Channel).To(gomega.Equal("channel"))
			gomega.Expect(roundTrip.Title).To(gomega.Equal("Alert"))
			gomega.Expect(roundTrip.Type).To(gomega.Equal(TypeCRIT))
			gomega.Expect(roundTrip.Critical).To(gomega.BeTrue())
		})

		ginkgo.It("should omit default type from the query", func() {
			config.ClientKey = "key"
			config.Channel = "channel"

			got := config.GetURL()
			gomega.Expect(got.Query().Get("type")).To(gomega.BeEmpty())
			gomega.Expect(got.Query().Get("critical")).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("SetDefaultProps", func() {
		ginkgo.It("should default type to INFO", func() {
			gomega.Expect(config.Type).To(gomega.Equal(TypeINFO))
			gomega.Expect(config.Type.String()).To(gomega.Equal("INFO"))
		})
	})
})
