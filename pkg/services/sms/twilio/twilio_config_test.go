package twilio

import (
	"net/url"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
)

var _ = ginkgo.Describe("Config Unit Tests", func() {
	var (
		config      *Config
		keyResolver format.PropKeyResolver
	)

	ginkgo.BeforeEach(func() {
		config = &Config{}
		keyResolver = format.NewPropKeyResolver(config)
	})

	ginkgo.Describe("SetURL", func() {
		ginkgo.It("should update the account SID from the user part of the url", func() {
			testURL := createTestURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.AccountSID).To(gomega.Equal("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
		})

		ginkgo.It("should update the auth token from the password part of the url", func() {
			testURL := createTestURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "testAuthToken", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.AuthToken).To(gomega.Equal("testAuthToken"))
		})

		ginkgo.It("should update the from number from the host part of the url", func() {
			testURL := createTestURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.FromNumber).To(gomega.Equal("+15551234567"))
		})

		ginkgo.It("should update the to numbers from the path part of the url", func() {
			testURL := createTestURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.ToNumbers).To(gomega.Equal([]string{"+15559876543"}))
		})

		ginkgo.It("should parse multiple recipients from the path", func() {
			testURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543/+15551111111")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.ToNumbers).To(gomega.Equal([]string{"+15559876543", "+15551111111"}))
		})

		ginkgo.It("should error if the account SID is missing", func() {
			testURL := createTestURL("", "authToken", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.MatchError(ErrAccountSIDMissing))
		})

		ginkgo.It("should error if the auth token is missing", func() {
			testURL := createTestURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.MatchError(ErrAuthTokenMissing))
		})

		ginkgo.It("should error if the from number is missing", func() {
			testURL := createTestURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.MatchError(ErrFromNumberMissing))
		})

		ginkgo.It("should error if the to numbers are missing", func() {
			testURL := createTestURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "+15551234567", "")
			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.MatchError(ErrToNumbersMissing))
		})

		ginkgo.It("should error if to and from numbers are the same", func() {
			testURL := createTestURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "+15551234567", "+15551234567")
			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.MatchError(ErrToFromNumberSame))
		})
	})

	ginkgo.Describe("GetURL", func() {
		ginkgo.It("should return the config as a URL", func() {
			config.AccountSID = "ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
			config.AuthToken = "testAuthToken"
			config.FromNumber = "+15551234567"
			config.ToNumbers = []string{"+15559876543"}

			configURL := config.GetURL()
			password, _ := configURL.User.Password()
			gomega.Expect(configURL.Scheme).To(gomega.Equal("twilio"))
			gomega.Expect(configURL.User.Username()).To(gomega.Equal(config.AccountSID))
			gomega.Expect(password).To(gomega.Equal(config.AuthToken))
			gomega.Expect(configURL.Host).To(gomega.Equal(config.FromNumber))
		})

		ginkgo.It("should encode multiple recipients in the path", func() {
			config.AccountSID = "ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
			config.AuthToken = "testAuthToken"
			config.FromNumber = "+15551234567"
			config.ToNumbers = []string{"+15559876543", "+15551111111"}

			configURL := config.GetURL()
			gomega.Expect(configURL.Path).To(gomega.Equal("/+15559876543/+15551111111"))
		})
	})

	ginkgo.Describe("Messaging Service SID", func() {
		ginkgo.It("should accept an MG-prefixed sender", func() {
			testURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/+15559876543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.FromNumber).To(gomega.Equal("MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
		})

		ginkgo.It("should not validate to==from when using Messaging Service SID", func() {
			// MG-prefixed senders skip the to==from validation
			testURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("normalizePhoneNumber", func() {
		ginkgo.It("should strip dashes and parentheses", func() {
			testURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+1(555)123-4567/+1(555)987-6543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.FromNumber).To(gomega.Equal("+15551234567"))
			gomega.Expect(config.ToNumbers).To(gomega.Equal([]string{"+15559876543"}))
		})

		ginkgo.It("should strip spaces and dots", func() {
			testURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+1.555.123.4567/+1 555 987 6543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.FromNumber).To(gomega.Equal("+15551234567"))
			gomega.Expect(config.ToNumbers).To(gomega.Equal([]string{"+15559876543"}))
		})

		ginkgo.It("should not modify Messaging Service SIDs", func() {
			result := normalizePhoneNumber("MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
			gomega.Expect(result).To(gomega.Equal("MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
		})
	})

	ginkgo.Describe("parseToNumbers", func() {
		ginkgo.It("should return nil for an empty path", func() {
			result := parseToNumbers("")
			gomega.Expect(result).To(gomega.BeNil())
		})

		ginkgo.It("should parse a single number", func() {
			result := parseToNumbers("/+15559876543")
			gomega.Expect(result).To(gomega.Equal([]string{"+15559876543"}))
		})

		ginkgo.It("should parse multiple numbers", func() {
			result := parseToNumbers("/+15559876543/+15551111111")
			gomega.Expect(result).To(gomega.Equal([]string{"+15559876543", "+15551111111"}))
		})

		ginkgo.It("should skip empty path segments", func() {
			result := parseToNumbers("/+15559876543//+15551111111")
			gomega.Expect(result).To(gomega.Equal([]string{"+15559876543", "+15551111111"}))
		})
	})

	ginkgo.Describe("Enums", func() {
		ginkgo.It("should return an empty map", func() {
			enums := config.Enums()
			gomega.Expect(enums).To(gomega.BeEmpty())
		})
	})

	ginkgo.Describe("PropKeyResolver", func() {
		ginkgo.When("setting a config key", func() {
			ginkgo.It("should update the title when supplied", func() {
				err := keyResolver.Set("title", "new title")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(config.Title).To(gomega.Equal("new title"))
			})

			ginkgo.It("should return an error for an unrecognized key", func() {
				err := keyResolver.Set("invalidkey", "value")
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
		})

		ginkgo.When("getting a config key", func() {
			ginkgo.It("should return the title value", func() {
				config.Title = "my title"
				value, err := keyResolver.Get("title")
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(value).To(gomega.Equal("my title"))
			})

			ginkgo.It("should return an error for an unrecognized key", func() {
				_, err := keyResolver.Get("invalidkey")
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
		})

		ginkgo.When("listing query fields", func() {
			ginkgo.It("should return the key title", func() {
				fields := keyResolver.QueryFields()
				gomega.Expect(fields).To(gomega.Equal([]string{"title"}))
			})
		})
	})

	ginkgo.Describe("Service API compliance", func() {
		ginkgo.It("should pass standard config tests", func() {
			cfg := &Config{}
			enums := cfg.Enums()
			gomega.Expect(enums).To(gomega.HaveLen(0))

			resolver := format.NewPropKeyResolver(cfg)
			fields := resolver.QueryFields()
			gomega.Expect(fields).To(gomega.HaveLen(1))
		})
	})
})

// createTestURL is a helper to construct a URL for config testing.
func createTestURL(accountSID, authToken, fromNumber, toNumber string) *url.URL {
	return &url.URL{
		Scheme: "twilio",
		User:   url.UserPassword(accountSID, authToken),
		Host:   fromNumber,
		Path:   "/" + toNumber,
	}
}
