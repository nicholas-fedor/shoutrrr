package e2e_test

import (
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

//nolint:paralleltest // Avoid using parallel test when making external calls
func Test_XMPP_E2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "XMPP E2E Tests")
}

func envOrSkip(key string) string {
	value := os.Getenv(key)
	if value == "" {
		ginkgo.Skip(key + " not set, skipping XMPP e2e test")
	}

	return value
}
