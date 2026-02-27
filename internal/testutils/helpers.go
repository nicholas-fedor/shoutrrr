package testutils

import (
	"log"
	"net/url"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestLogger returns a log.Logger that writes to ginkgo.GinkgoWriter for use in tests.
func TestLogger() *log.Logger {
	return log.New(ginkgo.GinkgoWriter, "[Test] ", 0)
}

// URLMust creates a url.URL from the given rawURL and fails the test if it cannot be parsed.
func URLMust(rawURL string) *url.URL {
	parsed, err := url.Parse(rawURL)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

	return parsed
}

// JSONRespondMust creates a httpmock.Responder with the given response
// as the body, and fails the test if it cannot be created.
func JSONRespondMust(code int, response any) httpmock.Responder {
	responder, err := httpmock.NewJsonResponder(code, response)
	gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred(), "invalid test response struct")

	return responder
}

// TestServiceSetInvalidParamValue tests whether the service returns an error
// when an invalid param key/value is passed through Send.
func TestServiceSetInvalidParamValue(service types.Service, key, value string) {
	err := service.Send("TestMessage", &types.Params{key: value})
	gomega.ExpectWithOffset(1, err).To(gomega.HaveOccurred())
}
