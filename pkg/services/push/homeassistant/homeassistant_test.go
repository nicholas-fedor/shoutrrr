package homeassistant

import (
	"bytes"
	"crypto/tls"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"

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
		ginkgo.It("should parse a valid URL and keep an injected HTTP client", func() {
			err := service.Initialize(
				mustParseURL("homeassistant://s3cret@ha.example.com"),
				logger,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Token).To(gomega.Equal("s3cret"))
			gomega.Expect(service.Config.Host).To(gomega.Equal("ha.example.com"))
			gomega.Expect(service.httpClient).To(gomega.BeIdenticalTo(client))
		})

		ginkgo.It("should accept the docs dummy URL", func() {
			err := service.Initialize(mustParseURL(dummyServiceURL), logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return an error when the token is missing", func() {
			err := service.Initialize(mustParseURL("homeassistant://ha.example.com"), logger)
			gomega.Expect(err).To(gomega.MatchError(ErrTokenMissing))
		})

		ginkgo.It("should create a default HTTP client when none is set", func() {
			service.httpClient = nil
			err := service.Initialize(
				mustParseURL("homeassistant://s3cret@ha.example.com"),
				logger,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.httpClient).NotTo(gomega.BeNil())
		})

		ginkgo.It("should skip TLS verification when skiptlsverify is set", func() {
			service.httpClient = nil
			err := service.Initialize(
				mustParseURL("homeassistant://s3cret@ha.example.com?skiptlsverify=yes"),
				logger,
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpClient, ok := service.httpClient.(*http.Client)
			gomega.Expect(ok).To(gomega.BeTrue())

			transport, ok := httpClient.Transport.(*http.Transport)
			gomega.Expect(ok).To(gomega.BeTrue())
			gomega.Expect(transport.TLSClientConfig).NotTo(gomega.BeNil())
			gomega.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(gomega.BeTrue())
			gomega.Expect(transport.TLSClientConfig.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS12)))
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
				mustParseURL("homeassistant://s3cret@ha.example.com"),
				logger,
			)).To(gomega.Succeed())
		})

		ginkgo.It("should POST persistent_notification.create with Bearer auth", func() {
			err := service.Send("Host unreachable", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(client.req).NotTo(gomega.BeNil())
			gomega.Expect(client.req.Method).To(gomega.Equal(http.MethodPost))
			gomega.Expect(client.req.URL.String()).To(gomega.Equal(
				"https://ha.example.com:443/api/services/persistent_notification/create",
			))
			gomega.Expect(client.req.Header.Get("Content-Type")).To(gomega.Equal(contentType))
			gomega.Expect(client.req.Header.Get("Authorization")).To(gomega.Equal("Bearer s3cret"))
			gomega.Expect(client.req.Header.Get("User-Agent")).To(
				gomega.Equal("shoutrrr/" + meta.Version),
			)

			var payload requestPayload
			gomega.Expect(json.Unmarshal(client.body, &payload)).To(gomega.Succeed())
			gomega.Expect(payload.Message).To(gomega.Equal("Host unreachable"))
			gomega.Expect(payload.Title).To(gomega.BeEmpty())
			gomega.Expect(payload.NotificationID).To(gomega.BeEmpty())
			gomega.Expect(payload.Targets).To(gomega.BeEmpty())
			gomega.Expect(bytes.Contains(client.body, []byte(`"title"`))).To(gomega.BeFalse())
			gomega.Expect(bytes.Contains(client.body, []byte(`"notification_id"`))).To(gomega.BeFalse())
			gomega.Expect(bytes.Contains(client.body, []byte(`"target"`))).To(gomega.BeFalse())
		})

		ginkgo.It("should include title and nid for persistent notifications", func() {
			gomega.Expect(service.Initialize(
				mustParseURL(
					"homeassistant://s3cret@ha.example.com?title=Server%20Down&nid=watchtower",
				),
				logger,
			)).To(gomega.Succeed())

			err := service.Send("Host unreachable", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			var payload requestPayload
			gomega.Expect(json.Unmarshal(client.body, &payload)).To(gomega.Succeed())
			gomega.Expect(payload.Title).To(gomega.Equal("Server Down"))
			gomega.Expect(payload.NotificationID).To(gomega.Equal("watchtower"))
			gomega.Expect(payload.Targets).To(gomega.BeEmpty())
		})

		ginkgo.It("should call notify services without notification_id", func() {
			gomega.Expect(service.Initialize(
				mustParseURL(
					"homeassistant://s3cret@ha.example.com"+
						"?service=notify.mobile_app_phone&targets=device1,device2&nid=ignored",
				),
				logger,
			)).To(gomega.Succeed())

			err := service.Send("ping", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(client.req.URL.String()).To(gomega.Equal(
				"https://ha.example.com:443/api/services/notify/mobile_app_phone",
			))

			var payload requestPayload
			gomega.Expect(json.Unmarshal(client.body, &payload)).To(gomega.Succeed())
			gomega.Expect(payload.Message).To(gomega.Equal("ping"))
			gomega.Expect(payload.NotificationID).To(gomega.BeEmpty())
			gomega.Expect(payload.Targets).To(gomega.Equal([]string{"device1", "device2"}))
			gomega.Expect(bytes.Contains(client.body, []byte(`"notification_id"`))).To(gomega.BeFalse())
			gomega.Expect(bytes.Contains(client.body, []byte(`"targets"`))).To(gomega.BeFalse())
			gomega.Expect(bytes.Contains(client.body, []byte(`"target"`))).To(gomega.BeTrue())
		})

		ginkgo.It("should apply runtime params without leaking into a later send", func() {
			params := &types.Params{
				"title": "Override",
				"nid":   "one",
			}
			err := service.Send("first", params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			var first requestPayload
			gomega.Expect(json.Unmarshal(client.body, &first)).To(gomega.Succeed())
			gomega.Expect(first.Title).To(gomega.Equal("Override"))
			gomega.Expect(first.NotificationID).To(gomega.Equal("one"))

			err = service.Send("second", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			var second requestPayload
			gomega.Expect(json.Unmarshal(client.body, &second)).To(gomega.Succeed())
			gomega.Expect(second.Message).To(gomega.Equal("second"))
			gomega.Expect(second.Title).To(gomega.BeEmpty())
			gomega.Expect(second.NotificationID).To(gomega.BeEmpty())
		})

		ginkgo.It("should accept HTTP 201 as success", func() {
			client.resp = &http.Response{
				StatusCode: http.StatusCreated,
				Status:     "201 Created",
				Body:       io.NopCloser(bytes.NewReader([]byte(`[]`))),
				Header:     make(http.Header),
			}

			err := service.Send("created", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should reject an empty message", func() {
			err := service.Send("", nil)
			gomega.Expect(err).To(gomega.MatchError(ErrMessageEmpty))
			gomega.Expect(client.req).To(gomega.BeNil())
		})

		ginkgo.It("should return an error for a 401 status", func() {
			client.resp = &http.Response{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
				Body:       io.NopCloser(bytes.NewReader([]byte("denied"))),
				Header:     make(http.Header),
			}

			err := service.Send("fail", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(errors.Is(err, ErrSendFailed)).To(gomega.BeTrue())
			gomega.Expect(errors.Is(err, ErrUnexpectedStatus)).To(gomega.BeTrue())
		})

		ginkgo.It("should return an error for a 400 status", func() {
			client.resp = &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(bytes.NewReader([]byte("bad"))),
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
