package signalgrid

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Service", func() {
	var (
		service *Service
		logger  types.StdLogger
		client  *stubHTTPClient
	)

	ginkgo.BeforeEach(func() {
		logger = &noOpLogger{}
		client = &stubHTTPClient{}
		service = &Service{}
		service.SetHTTPClient(client)
	})

	ginkgo.Describe("GetID", func() {
		ginkgo.It("should return the scheme name", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal(Scheme))
		})
	})

	ginkgo.Describe("Initialize", func() {
		ginkgo.It("should parse a valid URL and apply defaults", func() {
			err := service.Initialize(
				mustParseURL("signalgrid://abc123@channel"),
				logger,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.ClientKey).To(gomega.Equal("abc123"))
			gomega.Expect(service.Config.Channel).To(gomega.Equal("channel"))
			gomega.Expect(service.Config.Type).To(gomega.Equal(TypeINFO))
			gomega.Expect(service.httpClient).To(gomega.BeIdenticalTo(client))
		})

		ginkgo.It("should accept the docs dummy URL", func() {
			err := service.Initialize(mustParseURL(dummyServiceURL), logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return an error when the client key is missing", func() {
			err := service.Initialize(mustParseURL("signalgrid://channel"), logger)
			gomega.Expect(err).To(gomega.MatchError(ErrClientKeyMissing))
		})

		ginkgo.It("should create a default HTTP client when none is set", func() {
			service.httpClient = nil
			err := service.Initialize(
				mustParseURL("signalgrid://key@channel"),
				logger,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.httpClient).NotTo(gomega.BeNil())
		})
	})

	ginkgo.Describe("SetHTTPClient", func() {
		ginkgo.It("should replace the HTTP client", func() {
			replacement := &stubHTTPClient{}
			service.SetHTTPClient(replacement)
			gomega.Expect(service.httpClient).To(gomega.BeIdenticalTo(replacement))
		})
	})

	ginkgo.Describe("Send", func() {
		ginkgo.BeforeEach(func() {
			gomega.Expect(service.Initialize(
				mustParseURL("signalgrid://clientkey@channeltoken"),
				logger,
			)).To(gomega.Succeed())
		})

		ginkgo.It("should POST the Push API form fields", func() {
			err := service.Send("Host unreachable", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(client.req).NotTo(gomega.BeNil())
			gomega.Expect(client.req.Method).To(gomega.Equal(http.MethodPost))
			gomega.Expect(client.req.URL.String()).To(gomega.Equal(apiURL))
			gomega.Expect(client.req.Header.Get("Content-Type")).To(gomega.Equal(contentType))
			gomega.Expect(client.req.Header.Get("User-Agent")).To(
				gomega.Equal("shoutrrr/" + meta.Version),
			)

			form, err := url.ParseQuery(string(client.body))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(form.Get("client_key")).To(gomega.Equal("clientkey"))
			gomega.Expect(form.Get("channel")).To(gomega.Equal("channeltoken"))
			gomega.Expect(form.Get("body")).To(gomega.Equal("Host unreachable"))
			gomega.Expect(form.Get("type")).To(gomega.Equal("INFO"))
			gomega.Expect(form.Has("title")).To(gomega.BeFalse())
			gomega.Expect(form.Has("critical")).To(gomega.BeFalse())
		})

		ginkgo.It("should omit title when empty and omit critical when false", func() {
			err := service.Send("body only", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			form, err := url.ParseQuery(string(client.body))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(form.Has("title")).To(gomega.BeFalse())
			gomega.Expect(form.Has("critical")).To(gomega.BeFalse())
		})

		ginkgo.It("should include title and critical when set", func() {
			gomega.Expect(service.Initialize(
				mustParseURL(
					"signalgrid://clientkey@channeltoken?title=Server%20Down&type=CRIT&critical=true",
				),
				logger,
			)).To(gomega.Succeed())

			err := service.Send("Host unreachable", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			form, err := url.ParseQuery(string(client.body))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(form.Get("title")).To(gomega.Equal("Server Down"))
			gomega.Expect(form.Get("type")).To(gomega.Equal("CRIT"))
			gomega.Expect(form.Get("critical")).To(gomega.Equal("true"))
		})

		ginkgo.It("should apply runtime params", func() {
			params := &types.Params{
				"title":    "Override",
				"type":     "WARN",
				"critical": "Yes",
			}
			err := service.Send("updated", params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			form, err := url.ParseQuery(string(client.body))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(form.Get("title")).To(gomega.Equal("Override"))
			gomega.Expect(form.Get("type")).To(gomega.Equal("WARN"))
			gomega.Expect(form.Get("critical")).To(gomega.Equal("true"))
		})

		ginkgo.It("should not leak runtime params into a subsequent send", func() {
			params := &types.Params{
				"title":    "Override",
				"critical": "Yes",
			}
			err := service.Send("first", params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("second", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			form, err := url.ParseQuery(string(client.body))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(form.Get("body")).To(gomega.Equal("second"))
			gomega.Expect(form.Has("title")).To(gomega.BeFalse())
			gomega.Expect(form.Has("critical")).To(gomega.BeFalse())
		})

		ginkgo.It("should return an error for a non-success status", func() {
			client.resp = &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     "500 Internal Server Error",
				Body:       io.NopCloser(bytes.NewReader([]byte("boom"))),
				Header:     make(http.Header),
			}

			err := service.Send("fail", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(errors.Is(err, ErrSendFailed)).To(gomega.BeTrue())
			gomega.Expect(errors.Is(err, ErrUnexpectedStatus)).To(gomega.BeTrue())
		})

		ginkgo.It("should return an error for a 4xx status", func() {
			client.resp = &http.Response{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
				Body:       io.NopCloser(bytes.NewReader([]byte("denied"))),
				Header:     make(http.Header),
			}

			err := service.Send("fail", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(errors.Is(err, ErrUnexpectedStatus)).To(gomega.BeTrue())
		})

		ginkgo.It("should return an error on transport failure", func() {
			client.err = errors.New("connection refused")

			err := service.Send("fail", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(errors.Is(err, ErrSendFailed)).To(gomega.BeTrue())
		})
	})
})
