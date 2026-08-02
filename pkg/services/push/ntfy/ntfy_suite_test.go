package ntfy

import (
	"net/url"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	jsonclientmocks "github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient/mocks"
)

type noOpLogger struct{}

var _ types.StdLogger = (*noOpLogger)(nil)

func (n *noOpLogger) Print(_ ...any)            {}
func (n *noOpLogger) Printf(_ string, _ ...any) {}
func (n *noOpLogger) Println(_ ...any)          {}

func TestNtfy(t *testing.T) { //nolint:paralleltest // Ginkgo manages its own parallelization
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Ntfy Suite")
}

func newMockJSONClient() *jsonclientmocks.MockClient {
	return jsonclientmocks.NewMockClient(ginkgo.GinkgoT())
}

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return parsed
}
