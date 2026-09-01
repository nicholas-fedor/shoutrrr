package smtp

import (
	"context"
	"net"
	"strconv"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("dialClient", func() {
	ginkgo.It("should fail when the context is already canceled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := dialClient(ctx, &Config{
			Host:        "example.com",
			Port:        25,
			FromAddress: "sender@example.com",
			ToAddresses: []string{"rec@example.com"},
		})
		gomega.Expect(err).To(matchFailure(FailConnectToServer))
		gomega.Expect(err).To(gomega.MatchError(context.Canceled))
	})

	ginkgo.It("should fail when the host cannot be reached", func() {
		_, err := dialClient(context.Background(), &Config{
			Host: "127.0.0.1",
			Port: 1,
		})
		gomega.Expect(err).To(matchFailure(FailConnectToServer))
	})

	ginkgo.It("should create a client against a greeting server", func() {
		address, _, stop := startGreetingServer()
		defer stop()

		host, portStr, err := net.SplitHostPort(address)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		port, err := strconv.ParseUint(portStr, 10, 16)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		client, err := dialClient(context.Background(), &Config{Host: host, Port: uint16(port)})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client).NotTo(gomega.BeNil())
		gomega.Expect(client.Close()).To(gomega.Succeed())
	})
})
