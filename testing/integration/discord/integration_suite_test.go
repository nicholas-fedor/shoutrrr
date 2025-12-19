package discord_test

import (
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func TestDiscordIntegration(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Discord Integration Tests")
}

var _ = ginkgo.BeforeSuite(func() {
	SetupTestEnvironmentOnce()
})
