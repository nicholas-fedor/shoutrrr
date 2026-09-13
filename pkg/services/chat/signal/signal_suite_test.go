package signal

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

type capturedRequest struct {
	userAgent string
	payload   sendMessagePayload
}

type stubHTTPClient struct {
	req  *http.Request
	resp *http.Response
	err  error
}

var (
	logger *log.Logger

	_ types.HTTPClient = (*stubHTTPClient)(nil)

	_ = ginkgo.BeforeSuite(func() {
		logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)
	})
)

func (c *stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.req = req

	if c.err != nil {
		return nil, c.err
	}

	if c.resp != nil {
		return c.resp, nil
	}

	return &http.Response{
		StatusCode: http.StatusCreated,
		Status:     "201 Created",
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"timestamp":1}`))),
		Header:     make(http.Header),
	}, nil
}

func TestSignal(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Signal Suite")
}

func initMocked(svc *Service, rawURL string) {
	serviceURL, err := url.Parse(rawURL)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(svc.Initialize(serviceURL, logger)).NotTo(gomega.HaveOccurred())

	client, ok := svc.httpClient.(*http.Client)
	gomega.Expect(ok).To(gomega.BeTrue())
	httpmock.ActivateNonDefault(client)
}

func setupCapture(code int, body string, captured *capturedRequest) {
	httpmock.RegisterResponder(
		"POST",
		"https://localhost:8080/v2/send",
		func(req *http.Request) (*http.Response, error) {
			captured.userAgent = req.Header.Get("User-Agent")

			reqBody, err := io.ReadAll(req.Body)
			if err == nil {
				_ = json.Unmarshal(reqBody, &captured.payload)
			}

			return httpmock.NewStringResponse(code, body), nil
		},
	)
}
