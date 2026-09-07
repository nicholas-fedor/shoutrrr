package smtp

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
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
		}, nil)
		gomega.Expect(err).To(matchFailure(FailConnectToServer))
		gomega.Expect(err).To(gomega.MatchError(context.Canceled))
	})

	ginkgo.It("should fail when the host cannot be reached", func() {
		_, err := dialClient(context.Background(), &Config{
			Host: "127.0.0.1",
			Port: 1,
		}, nil)
		gomega.Expect(err).To(matchFailure(FailConnectToServer))
	})

	ginkgo.It("should create a client against a greeting server", func() {
		address, _, stop := startGreetingServer()
		defer stop()

		host, portStr, err := net.SplitHostPort(address)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		port, err := strconv.ParseUint(portStr, 10, 16)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		client, err := dialClient(context.Background(), &Config{Host: host, Port: uint16(port)}, nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client).NotTo(gomega.BeNil())
		gomega.Expect(client.Close()).To(gomega.Succeed())
	})

	ginkgo.It("should use a custom dialer", func() {
		address, _, stop := startGreetingServer()
		defer stop()

		host, portStr, err := net.SplitHostPort(address)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		port, err := strconv.ParseUint(portStr, 10, 16)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		var gotNetwork, gotAddr string

		dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
			gotNetwork = network
			gotAddr = addr

			return (&net.Dialer{}).DialContext(ctx, network, addr)
		}

		client, err := dialClient(
			context.Background(),
			&Config{Host: host, Port: uint16(port)},
			dial,
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(client).NotTo(gomega.BeNil())
		gomega.Expect(gotNetwork).To(gomega.Equal("tcp"))
		gomega.Expect(gotAddr).To(gomega.Equal(net.JoinHostPort(host, portStr)))
		gomega.Expect(client.Close()).To(gomega.Succeed())
	})

	ginkgo.It("should surface a custom dialer error as FailConnectToServer", func() {
		blocked := errors.New("destination blocked")
		_, err := dialClient(
			context.Background(),
			&Config{Host: "mail.example.com", Port: 587},
			func(_ context.Context, _, _ string) (net.Conn, error) {
				return nil, blocked
			},
		)
		gomega.Expect(err).To(matchFailure(FailConnectToServer))
		gomega.Expect(err).To(gomega.MatchError(blocked))
	})
})

var _ = ginkgo.Describe("SetDialContext", func() {
	ginkgo.It("should fail Send when the custom dialer blocks the destination", func() {
		svc := &Service{}
		serviceURL, err := url.Parse(
			"smtp://mail.example.com:587/?fromaddress=from@example.com&toaddresses=to@example.com",
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(svc.Initialize(serviceURL, logger)).To(gomega.Succeed())

		blocked := errors.New("destination blocked")

		svc.SetDialContext(func(_ context.Context, _, _ string) (net.Conn, error) {
			return nil, blocked
		})

		err = svc.Send("hello", nil)
		gomega.Expect(err).To(matchFailure(FailGetSMTPClient))
		gomega.Expect(err).To(gomega.MatchError(blocked))
	})

	ginkgo.It("should satisfy DialContextSetter", func() {
		var setter types.DialContextSetter = &Service{}
		gomega.Expect(setter).NotTo(gomega.BeNil())
	})
})
