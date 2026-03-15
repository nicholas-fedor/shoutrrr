package mqtt

import (
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

func TestMQTT(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "MQTT Suite")
}
