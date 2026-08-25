package signalgrid_test

import (
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/signalgrid"
)

const hookURL = "https://api.signalgrid.co/v1/push"

var (
	service     *signalgrid.Service
	config      *signalgrid.Config
	keyResolver format.PropKeyResolver
	logger      *log.Logger

	_ = ginkgo.BeforeSuite(func() {
		service = &signalgrid.Service{}
		logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)
	})
)

var _ = ginkgo.Describe("the Signalgrid service", func() {
	ginkgo.It("returns the correct service identifier", func() {
		gomega.Expect(service.GetID()).To(gomega.Equal("signalgrid"))
	})
})

var _ = ginkgo.Describe("the Signalgrid config", func() {
	ginkgo.BeforeEach(func() {
		config = &signalgrid.Config{}
		keyResolver = format.NewPropKeyResolver(config)
	})

	ginkgo.When("updating it using a URL", func() {
		ginkgo.It("should update the client key", func() {
			serviceURL, err := url.Parse(
				"signalgrid://clientkey@channel",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.ClientKey).To(
				gomega.Equal("clientkey"),
			)
		})

		ginkgo.It("should update the channel", func() {
			serviceURL, err := url.Parse(
				"signalgrid://clientkey@channel",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Channel).To(
				gomega.Equal("channel"),
			)
		})

		ginkgo.It("should update title and type", func() {
			serviceURL, err := url.Parse(
				"signalgrid://clientkey@channel" +
					"?title=Server%20Down&type=CRIT",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Title).To(
				gomega.Equal("Server Down"),
			)
			gomega.Expect(config.Type).To(
				gomega.Equal("CRIT"),
			)
		})

		ginkgo.It("should update critical", func() {
			serviceURL, err := url.Parse(
				"signalgrid://clientkey@channel?critical=true",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(serviceURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(config.Critical).To(gomega.BeTrue())
		})

		ginkgo.It("should reject a missing client key", func() {
			serviceURL, err := url.Parse(
				"signalgrid://channel",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(serviceURL)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should reject a missing channel", func() {
			serviceURL, err := url.Parse(
				"signalgrid://clientkey@",
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = config.SetURL(serviceURL)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.When("getting the current config", func() {
		ginkgo.It("should return a Signalgrid URL", func() {
			config.ClientKey = "clientkey"
			config.Channel = "channel"
			config.Title = "Test"
			config.Type = "CRIT"
			config.Critical = true

			serviceURL := config.GetURL()

			gomega.Expect(serviceURL.Scheme).To(
				gomega.Equal("signalgrid"),
			)
			gomega.Expect(serviceURL.User.Username()).To(
				gomega.Equal("clientkey"),
			)
			gomega.Expect(serviceURL.Host).To(
				gomega.Equal("channel"),
			)
		})
	})

	ginkgo.When("setting config parameters", func() {
		ginkgo.It("should set the title", func() {
			err := keyResolver.Set("title", "Server Down")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Title).To(
				gomega.Equal("Server Down"),
			)
		})

		ginkgo.It("should set the type", func() {
			err := keyResolver.Set("type", "CRIT")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Type).To(
				gomega.Equal("CRIT"),
			)
		})

		ginkgo.It("should set critical", func() {
			err := keyResolver.Set("critical", "true")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Critical).To(gomega.BeTrue())
		})

		ginkgo.It("should reject an unknown parameter", func() {
			err := keyResolver.Set("invalid", "value")

			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})
})

var _ = ginkgo.Describe("sending Signalgrid notifications", func() {
	ginkgo.BeforeEach(func() {
		httpmock.Activate()
	})

	ginkgo.AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	ginkgo.It("should send a notification successfully", func() {
		serviceURL, err := url.Parse(
			"signalgrid://clientkey@channel" +
				"?title=Test&type=INFO",
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = service.Initialize(serviceURL, logger)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		httpmock.RegisterResponder(
			"POST",
			hookURL,
			httpmock.NewStringResponder(
				200,
				`{"code":"200","text":"OK"}`,
			),
		)

		err = service.Send("Test message", nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		gomega.Expect(
			httpmock.GetTotalCallCount(),
		).To(gomega.Equal(1))
	})

	ginkgo.It("should send the correct payload", func() {
		serviceURL, err := url.Parse(
			"signalgrid://clientkey@channel" +
				"?title=Server%20Down&type=CRIT&critical=true",
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = service.Initialize(serviceURL, logger)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		httpmock.RegisterResponder(
			"POST",
			hookURL,
			func(req *http.Request) (*http.Response, error) {
				err := req.ParseForm()
				gomega.Expect(err).NotTo(
					gomega.HaveOccurred(),
				)

				gomega.Expect(
					req.Form.Get("client_key"),
				).To(gomega.Equal("clientkey"))

				gomega.Expect(
					req.Form.Get("channel"),
				).To(gomega.Equal("channel"))

				gomega.Expect(
					req.Form.Get("title"),
				).To(gomega.Equal("Server Down"))

				gomega.Expect(
					req.Form.Get("body"),
				).To(gomega.Equal("Host unreachable"))

				gomega.Expect(
					req.Form.Get("type"),
				).To(gomega.Equal("CRIT"))

				gomega.Expect(
					req.Form.Get("critical"),
				).To(gomega.Equal("true"))

				return httpmock.NewStringResponse(
					200,
					`{"code":"200","text":"OK"}`,
				), nil
			},
		)

		err = service.Send("Host unreachable", nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	})

	ginkgo.It("should return an error for an HTTP error", func() {
		serviceURL, err := url.Parse(
			"signalgrid://clientkey@channel",
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		err = service.Initialize(serviceURL, logger)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		httpmock.RegisterResponder(
			"POST",
			hookURL,
			httpmock.NewStringResponder(
				500,
				"Internal Server Error",
			),
		)

		err = service.Send("Test message", nil)
		gomega.Expect(err).To(gomega.HaveOccurred())
	})
})

func TestSignalgrid(t *testing.T) {
	t.Parallel()

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Signalgrid Suite")
}
