package twilio_test

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio"
)

const hookURL = "https://api.twilio.com/2010-04-01/Accounts/ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/Messages.json"

func TestTwilio(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Twilio Suite")
}

var (
	service      *twilio.Service
	config       *twilio.Config
	keyResolver  format.PropKeyResolver
	envTwilioURL *url.URL
	logger       *log.Logger
	_            = ginkgo.BeforeSuite(func() {
		service = &twilio.Service{}
		logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)
		envTwilioURL, _ = url.Parse(os.Getenv("SHOUTRRR_TWILIO_URL"))
	})
)

var _ = ginkgo.Describe("the twilio service", func() {
	ginkgo.When("running integration tests", func() {
		ginkgo.It("should work", func() {
			if envTwilioURL.String() == "" {
				return
			}
			serviceURL, _ := url.Parse(envTwilioURL.String())
			err := service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = service.Send("this is an integration test", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("returns the correct service identifier", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal("twilio"))
		})
	})
})

var _ = ginkgo.Describe("the twilio config", func() {
	ginkgo.BeforeEach(func() {
		config = &twilio.Config{}
		keyResolver = format.NewPropKeyResolver(config)
	})
	ginkgo.When("updating it using a url", func() {
		ginkgo.It("should update the account SID from the user part of the url", func() {
			testURL := createURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.AccountSID).To(gomega.Equal("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
		})
		ginkgo.It("should update the auth token from the password part of the url", func() {
			testURL := createURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "testAuthToken", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.AuthToken).To(gomega.Equal("testAuthToken"))
		})
		ginkgo.It("should update the from number from the host part of the url", func() {
			testURL := createURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.FromNumber).To(gomega.Equal("+15551234567"))
		})
		ginkgo.It("should update the to numbers from the path part of the url", func() {
			testURL := createURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "+15551234567", "+15559876543")
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
			testURL := createURL("", "authToken", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("should error if the auth token is missing", func() {
			testURL := createURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "", "+15551234567", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("should error if the from number is missing", func() {
			testURL := createURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "", "+15559876543")
			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("should error if the to numbers are missing", func() {
			testURL := createURL("ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "authToken", "+15551234567", "")
			err := config.SetURL(testURL)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})
	ginkgo.When("getting the current config", func() {
		ginkgo.It("should return the config that is currently set as a url", func() {
			config.AccountSID = "ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
			config.AuthToken = "testAuthToken"
			config.FromNumber = "+15551234567"
			config.ToNumbers = []string{"+15559876543"}

			configURL := config.GetURL()
			password, _ := configURL.User.Password()
			gomega.Expect(configURL.User.Username()).To(gomega.Equal(config.AccountSID))
			gomega.Expect(password).To(gomega.Equal(config.AuthToken))
			gomega.Expect(configURL.Host).To(gomega.Equal(config.FromNumber))
			gomega.Expect(configURL.Scheme).To(gomega.Equal("twilio"))
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
	ginkgo.When("using a Messaging Service SID as the sender", func() {
		ginkgo.It("should accept an MG-prefixed sender", func() {
			testURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/+15559876543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = config.SetURL(testURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.FromNumber).To(gomega.Equal("MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
		})
	})
	ginkgo.When("normalizing phone numbers", func() {
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
	})
	ginkgo.When("setting a config key", func() {
		ginkgo.It("should update the title when it is supplied", func() {
			err := keyResolver.Set("title", "new title")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Title).To(gomega.Equal("new title"))
		})
		ginkgo.It("should return an error if the key is not recognized", func() {
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
		ginkgo.It("should return an error if the key is not recognized", func() {
			_, err := keyResolver.Get("invalidkey")
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})
	ginkgo.When("listing the query fields", func() {
		ginkgo.It("should return the key title", func() {
			fields := keyResolver.QueryFields()
			gomega.Expect(fields).To(gomega.Equal([]string{"title"}))
		})
	})

	ginkgo.Describe("sending the payload", func() {
		ginkgo.BeforeEach(func() {
			httpmock.Activate()
		})
		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})
		ginkgo.It("should not report an error if the server accepts the payload", func() {
			serviceURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder("POST", hookURL, httpmock.NewStringResponder(201, ""))

			err = service.Send("Message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("should send to multiple recipients", func() {
			serviceURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543/+15551111111")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder("POST", hookURL, httpmock.NewStringResponder(201, ""))

			err = service.Send("Message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(httpmock.GetTotalCallCount()).To(gomega.Equal(2))
		})
		ginkgo.It("should use MessagingServiceSid for MG-prefixed senders", func() {
			serviceURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/+15559876543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder("POST", hookURL,
				func(req *http.Request) (*http.Response, error) {
					err := req.ParseForm()
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(req.Form.Get("MessagingServiceSid")).To(gomega.Equal("MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"))
					gomega.Expect(req.Form.Get("From")).To(gomega.BeEmpty())
					return httpmock.NewStringResponse(201, ""), nil
				})

			err = service.Send("Message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("should not panic if an error occurs when sending the payload", func() {
			serviceURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				hookURL,
				httpmock.NewErrorResponder(errors.New("dummy error")),
			)

			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("should return a descriptive error from the Twilio API response", func() {
			serviceURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder("POST", hookURL,
				httpmock.NewStringResponder(400, `{"code": 21211, "message": "The 'To' number is not a valid phone number.", "status": 400}`))

			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("The 'To' number is not a valid phone number."))
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("21211"))
		})
		ginkgo.It("should return an error if the server returns a non-2xx status", func() {
			serviceURL, err := url.Parse("twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder("POST", hookURL, httpmock.NewStringResponder(401, ""))

			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("the basic service API", func() {
		ginkgo.It("should implement correctly", func() {
			testutils.TestConfigGetInvalidQueryValue(&twilio.Config{})
			testutils.TestConfigSetInvalidQueryValue(&twilio.Config{}, "twilio://user:pass@host/path?foo=bar")
			testutils.TestConfigSetDefaultValues(&twilio.Config{})
			testutils.TestConfigGetEnumsCount(&twilio.Config{}, 0)
			testutils.TestConfigGetFieldsCount(&twilio.Config{}, 1)
		})
	})
})

func createURL(accountSID string, authToken string, fromNumber string, toNumber string) *url.URL {
	return &url.URL{
		Scheme: "twilio",
		User:   url.UserPassword(accountSID, authToken),
		Host:   fromNumber,
		Path:   "/" + toNumber,
	}
}
