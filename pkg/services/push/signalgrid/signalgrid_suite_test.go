package signalgrid

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

type noOpLogger struct{}

type stubHTTPClient struct {
	req  *http.Request
	body []byte
	resp *http.Response
	err  error
}

var _ types.StdLogger = (*noOpLogger)(nil)

func (*noOpLogger) Print(_ ...any)            {}
func (*noOpLogger) Printf(_ string, _ ...any) {}
func (*noOpLogger) Println(_ ...any)          {}

func (c *stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.req = req

	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err == nil {
			c.body = body
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	if c.err != nil {
		return nil, c.err
	}

	if c.resp != nil {
		return c.resp, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"code":"200","text":"OK"}`))),
		Header:     make(http.Header),
	}, nil
}

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return parsed
}

func TestSignalgrid(t *testing.T) { //nolint:paralleltest // Ginkgo manages its own parallelization
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Signalgrid Suite")
}
