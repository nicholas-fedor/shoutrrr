package smtp

import (
	"crypto/tls"
	"errors"
	"net/url"
	"time"

	"github.com/stretchr/testify/mock"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

var _ = ginkgo.Describe("Config", func() {
	ginkgo.Describe("Clone", func() {
		ginkgo.It("should be identical to the original", func() {
			config := &Config{}
			gomega.Expect(config.SetURL(testutils.URLMust(urlWithAllProps))).To(gomega.Succeed())
			gomega.Expect(config.Clone()).To(gomega.Equal(*config))
		})

		ginkgo.It("should copy ToAddresses so later mutations are independent", func() {
			config := &Config{ToAddresses: []string{"a@example.com"}}
			clone := config.Clone()
			clone.ToAddresses[0] = "b@example.com"
			gomega.Expect(config.ToAddresses[0]).To(gomega.Equal("a@example.com"))
		})
	})

	ginkgo.Describe("Enums", func() {
		ginkgo.It("should expose Auth and Encryption formatters", func() {
			enums := (&Config{}).Enums()
			gomega.Expect(enums).To(gomega.HaveLen(2))
			gomega.Expect(enums).To(gomega.HaveKey("Auth"))
			gomega.Expect(enums).To(gomega.HaveKey("Encryption"))
			gomega.Expect(enums["Auth"]).To(gomega.Equal(AuthTypes.Enum))
			gomega.Expect(enums["Encryption"]).To(gomega.Equal(EncMethods.Enum))
		})
	})

	ginkgo.Describe("GetURL and SetURL", func() {
		ginkgo.It("should be identical after de-/serialization", func() {
			url := testutils.URLMust(urlWithAllProps)
			config := &Config{}
			pkr := format.NewPropKeyResolver(config)
			gomega.Expect(config.setURL(&pkr, url)).To(gomega.Succeed())

			outputURL := config.GetURL()
			ginkgo.GinkgoT().Logf("\n\n%s\n%s\n\n-", outputURL, urlWithAllProps)
			gomega.Expect(outputURL.String()).To(gomega.Equal(urlWithAllProps))
		})

		ginkgo.When("skiptlsverify is set to yes", func() {
			ginkgo.It("should set SkipTLSVerify to true", func() {
				config := &Config{}
				gomega.Expect(config.SetURL(testutils.URLMust(
					"smtp://user:password@example.com:2225/?fromAddress=sender@example.com&toAddresses=rec1@example.com&skiptlsverify=yes",
				))).To(gomega.Succeed())
				gomega.Expect(config.SkipTLSVerify).To(gomega.BeTrue())
			})
		})

		ginkgo.When("skiptlsverify is set to no", func() {
			ginkgo.It("should set SkipTLSVerify to false", func() {
				config := &Config{}
				gomega.Expect(config.SetURL(testutils.URLMust(
					"smtp://user:password@example.com:2225/?fromAddress=sender@example.com&toAddresses=rec1@example.com&skiptlsverify=no",
				))).To(gomega.Succeed())
				gomega.Expect(config.SkipTLSVerify).To(gomega.BeFalse())
			})
		})

		ginkgo.When("fromAddress is missing", func() {
			ginkgo.It("should return ErrFromAddressMissing", func() {
				gomega.Expect((&Config{}).SetURL(testutils.URLMust(
					"smtp://user:password@example.com:2225/?toAddresses=rec1@example.com,rec2@example.com",
				))).To(gomega.MatchError(ErrFromAddressMissing))
			})
		})

		ginkgo.When("toAddresses are missing", func() {
			ginkgo.It("should return ErrToAddressMissing", func() {
				gomega.Expect((&Config{}).SetURL(testutils.URLMust(
					"smtp://user:password@example.com:2225/?fromAddress=sender@example.com",
				))).To(gomega.MatchError(ErrToAddressMissing))
			})
		})

		ginkgo.It("should not allow getting invalid query values", func() {
			testutils.TestConfigGetInvalidQueryValue(&Config{})
		})

		ginkgo.It("should not allow setting invalid query values", func() {
			testutils.TestConfigSetInvalidQueryValue(
				&Config{},
				"smtp://example.com/?fromAddress=s@example.com&toAddresses=r@example.com&foo=bar",
			)
		})

		ginkgo.It("should have the expected number of fields and enums", func() {
			config := &Config{}
			testutils.TestConfigGetEnumsCount(config, 2)
			testutils.TestConfigGetFieldsCount(config, 16)
		})

		ginkgo.It("should include requirestarttls and skiptlsverify only when true", func() {
			config := &Config{
				Host:            "example.com",
				Port:            25,
				FromAddress:     "sender@example.com",
				ToAddresses:     []string{"rec@example.com"},
				RequireStartTLS: true,
				SkipTLSVerify:   true,
			}

			query := config.GetURL().Query()
			gomega.Expect(query.Get("requirestarttls")).To(gomega.Equal("Yes"))
			gomega.Expect(query.Get("skiptlsverify")).To(gomega.Equal("Yes"))

			config.RequireStartTLS = false
			config.SkipTLSVerify = false
			query = config.GetURL().Query()
			gomega.Expect(query.Get("requirestarttls")).To(gomega.BeEmpty())
			gomega.Expect(query.Get("skiptlsverify")).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("getURL", func() {
		ginkgo.It("should skip keys the resolver cannot get", func() {
			config := &Config{Host: "example.com", Port: 25}
			resolver := typesmocks.NewMockConfigQueryResolver(ginkgo.GinkgoT())
			resolver.On("Get", mock.Anything).Return("", errors.New("missing")).Times(10)

			got := config.getURL(resolver)
			gomega.Expect(got.Scheme).To(gomega.Equal(Scheme))
			gomega.Expect(got.Host).To(gomega.Equal("example.com:25"))
			gomega.Expect(got.RawQuery).To(gomega.BeEmpty())
		})

		ginkgo.It("should serialize resolver values into the query", func() {
			config := &Config{Host: "mail.example.com", Port: 587, Username: "user", Password: "pass"}
			resolver := typesmocks.NewMockConfigQueryResolver(ginkgo.GinkgoT())
			resolver.On("Get", mock.Anything).Return("value", nil).Times(10)

			got := config.getURL(resolver)
			gomega.Expect(got.User.Username()).To(gomega.Equal("user"))
			pass, ok := got.User.Password()
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(pass).To(gomega.Equal("pass"))
			gomega.Expect(got.RawQuery).To(gomega.ContainSubstring("auth=value"))
			gomega.Expect(got.RawQuery).To(gomega.ContainSubstring("fromaddress=value"))
		})
	})

	ginkgo.Describe("setURL", func() {
		ginkgo.It("should wrap resolver Set errors", func() {
			resolver := typesmocks.NewMockConfigQueryResolver(ginkgo.GinkgoT())
			resolver.On("Set", "foo", "bar").Return(errors.New("nope"))

			err := (&Config{}).setURL(resolver, testutils.URLMust("smtp://dummy@dummy.com?foo=bar"))
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring(`setting query parameter "foo" to "bar"`))
		})

		ginkgo.It("should allow the dummy URL without from or to addresses", func() {
			gomega.Expect((&Config{}).SetURL(testutils.URLMust("smtp://dummy@dummy.com"))).To(gomega.Succeed())
		})

		ginkgo.It("should parse an IPv6 host", func() {
			config := &Config{}
			gomega.Expect(config.SetURL(testutils.URLMust(
				"smtp://[::1]:587/?fromAddress=sender@example.com&toAddresses=rec1@example.com",
			))).To(gomega.Succeed())
			gomega.Expect(config.Host).To(gomega.Equal("::1"))
			gomega.Expect(config.Port).To(gomega.Equal(uint16(587)))
			gomega.Expect(config.GetURL().Host).To(gomega.Equal("[::1]:587"))
			gomega.Expect(config.GetURL().Hostname()).To(gomega.Equal("::1"))
		})

		ginkgo.It("should keep the previous port when the URL port is out of range", func() {
			config := &Config{Port: DefaultSMTPPort}
			gomega.Expect(config.SetURL(testutils.URLMust(
				"smtp://example.com:99999/?fromAddress=sender@example.com&toAddresses=rec1@example.com",
			))).To(gomega.Succeed())
			gomega.Expect(config.Port).To(gomega.Equal(uint16(DefaultSMTPPort)))
		})

		ginkgo.It("should treat an encoded semicolon as part of a single recipient", func() {
			config := &Config{}
			gomega.Expect(config.SetURL(testutils.URLMust(
				"smtp://example.com/?fromAddress=sender@example.com&toAddresses=a@b.com%3Bc@d.com",
			))).To(gomega.Succeed())
			gomega.Expect(config.ToAddresses).To(gomega.Equal([]string{"a@b.com;c@d.com"}))
		})

		ginkgo.It("should keep display-name spaces in angle-bracket recipients", func() {
			config := &Config{}
			gomega.Expect(config.SetURL(testutils.URLMust(
				"smtp://example.com/?fromAddress=sender@example.com&toAddresses=" +
					url.QueryEscape("Name <a@b.com>"),
			))).To(gomega.Succeed())
			gomega.Expect(config.ToAddresses).To(gomega.Equal([]string{"Name <a@b.com>"}))
		})

		ginkgo.It("should decode a password that contains @ and spaces", func() {
			config := &Config{}
			gomega.Expect(config.SetURL(testutils.URLMust(
				"smtp://user:p%40ss%20word@example.com:587/?fromAddress=sender@example.com&toAddresses=rec1@example.com",
			))).To(gomega.Succeed())
			gomega.Expect(config.Username).To(gomega.Equal("user"))
			gomega.Expect(config.Password).To(gomega.Equal("p@ss word"))
		})
	})

	ginkgo.Describe("restorePlusAddresses", func() {
		ginkgo.It("should restore plus tags that URL parsing turned into spaces", func() {
			config := &Config{
				FromAddress: "sender tag@example.com",
				ToAddresses: []string{"rec1 tag@example.com", "rec2@example.com"},
			}
			config.restorePlusAddresses()
			gomega.Expect(config.FromAddress).To(gomega.Equal("sender+tag@example.com"))
			gomega.Expect(config.ToAddresses).To(gomega.Equal([]string{
				"rec1+tag@example.com",
				"rec2@example.com",
			}))
		})

		ginkgo.It("should keep display-name spaces and restore plus tags in the addr-spec", func() {
			config := &Config{
				FromAddress: "Sender Name <sender tag@example.com>",
				ToAddresses: []string{"Ops Team <rec1 tag@example.com>"},
			}
			config.restorePlusAddresses()
			gomega.Expect(config.FromAddress).To(gomega.Equal("Sender Name <sender+tag@example.com>"))
			gomega.Expect(config.ToAddresses).To(gomega.Equal([]string{
				"Ops Team <rec1+tag@example.com>",
			}))
		})
	})

	ginkgo.Describe("newTLSConfig", func() {
		ginkgo.It("should set the server name and TLS 1.2 minimum without a version ceiling", func() {
			got := newTLSConfig(&Config{Host: "mail.example.com"})
			gomega.Expect(got.ServerName).To(gomega.Equal("mail.example.com"))
			gomega.Expect(got.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS12)))
			gomega.Expect(got.MaxVersion).To(gomega.BeZero())
			gomega.Expect(got.InsecureSkipVerify).To(gomega.BeFalse())
		})

		ginkgo.It("should skip certificate verification when configured", func() {
			got := newTLSConfig(&Config{Host: "mail.example.com", SkipTLSVerify: true})
			gomega.Expect(got.InsecureSkipVerify).To(gomega.BeTrue())
		})
	})

	ginkgo.When("applying the default props", func() {
		ginkgo.It("should apply the tagged timeout default", func() {
			config := &Config{}
			resolver := format.NewPropKeyResolver(config)

			gomega.Expect(resolver.SetDefaultProps(config)).To(gomega.Succeed())
			gomega.Expect(config.Timeout).To(gomega.Equal(defaultTimeout))
			gomega.Expect(config.Subject).To(gomega.BeEmpty())
		})
	})

	ginkgo.When("serializing the timeout", func() {
		ginkgo.It("should render it as a duration rather than a nanosecond count", func() {
			config := &Config{Timeout: 10 * time.Second}
			resolver := format.NewPropKeyResolver(config)

			value, err := resolver.Get("timeout")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(value).To(gomega.Equal("10s"))
		})
	})
})
