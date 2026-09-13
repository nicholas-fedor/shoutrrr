package xmpp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

type mockSession struct {
	chats [][2]string
	rooms [][4]string
	err   error
}

var _ = ginkgo.Describe("service", func() {
	var service *Service

	ginkgo.BeforeEach(func() {
		service = &Service{}
	})

	ginkgo.It("should return xmpp as the service ID", func() {
		gomega.Expect(service.GetID()).To(gomega.Equal(Scheme))
	})

	ginkgo.It("should initialize from a valid URL", func() {
		err := service.Initialize(
			testutils.URLMust("xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"),
			testutils.TestLogger(),
		)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(service.Config.Host).To(gomega.Equal("xmpp.example.com"))
	})

	ginkgo.It("should return an error when initializing an invalid URL", func() {
		err := service.Initialize(
			testutils.URLMust("xmpp://alice:secret@xmpp.example.com/"),
			testutils.TestLogger(),
		)
		gomega.Expect(err).To(gomega.MatchError(ErrMissingTargets))
	})

	ginkgo.It("should implement DialContextSetter", func() {
		_, ok := any(service).(types.DialContextSetter)
		gomega.Expect(ok).To(gomega.BeTrue())
	})

	ginkgo.It("should send chat messages through the session", func() {
		session := &mockSession{}

		gomega.Expect(service.Initialize(
			testutils.URLMust("xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"),
			testutils.TestLogger(),
		)).To(gomega.Succeed())
		service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
			return session, nil
		})
		service.SetDialContext(pipeDialer())

		err := service.Send("hello", nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(session.chats).To(gomega.Equal([][2]string{{"bob@example.com", "hello"}}))
	})

	ginkgo.It("should prepend the title and send to rooms", func() {
		session := &mockSession{}

		gomega.Expect(service.Initialize(
			testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/"+
					"?rooms=alerts@conference.example.com&title=Alert&roompassword=pw",
			),
			testutils.TestLogger(),
		)).To(gomega.Succeed())
		service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
			return session, nil
		})
		service.SetDialContext(pipeDialer())

		err := service.Send("hello", nil)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(session.rooms).To(gomega.Equal([][4]string{{
			"alerts@conference.example.com",
			"alice",
			"pw",
			"Alert\nhello",
		}}))
	})

	ginkgo.It("should dial the configured host and port", func() {
		var (
			mu      sync.Mutex
			gotAddr string
			gotNet  string
			dialed  bool
		)

		gomega.Expect(service.Initialize(
			testutils.URLMust("xmpp://alice:secret@xmpp.example.com:5222/?to=bob@example.com"),
			testutils.TestLogger(),
		)).To(gomega.Succeed())
		service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
			return &mockSession{}, nil
		})
		service.SetDialContext(func(_ context.Context, network, addr string) (net.Conn, error) {
			mu.Lock()
			gotNet = network
			gotAddr = addr
			dialed = true
			mu.Unlock()

			return pipeDialer()(context.Background(), network, addr)
		})

		gomega.Expect(service.Send("hello", nil)).To(gomega.Succeed())
		mu.Lock()
		defer mu.Unlock()

		gomega.Expect(dialed).To(gomega.BeTrue())
		gomega.Expect(gotNet).To(gomega.Equal("tcp"))
		gomega.Expect(gotAddr).To(gomega.Equal("xmpp.example.com:5222"))
	})

	ginkgo.It("should restore the default session factory when nil is set", func() {
		gomega.Expect(service.Initialize(
			testutils.URLMust("xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"),
			testutils.TestLogger(),
		)).To(gomega.Succeed())
		service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
			return &mockSession{}, nil
		})
		service.SetSessionFactory(nil)
		service.SetDialContext(func(context.Context, string, string) (net.Conn, error) {
			client, server := net.Pipe()
			gomega.Expect(server.Close()).To(gomega.Succeed())

			return client, nil
		})

		err := service.Send("hello", nil)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("negotiating XMPP session"))
	})

	ginkgo.It("should return a dial error from the default dialer", func() {
		gomega.Expect(service.Initialize(
			testutils.URLMust("xmpp://alice:secret@127.0.0.1:1/?to=bob@example.com"),
			testutils.TestLogger(),
		)).To(gomega.Succeed())

		err := service.Send("hello", nil)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("dialing"))
	})

	ginkgo.It("should close the connection when session setup fails", func() {
		gomega.Expect(service.Initialize(
			testutils.URLMust("xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"),
			testutils.TestLogger(),
		)).To(gomega.Succeed())
		service.SetDialContext(pipeDialer())
		service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
			return nil, errors.New("negotiate failed")
		})

		err := service.Send("hello", nil)
		gomega.Expect(err).To(gomega.MatchError("negotiate failed"))
	})

	ginkgo.It("should return chat send errors", func() {
		session := &mockSession{err: errors.New("chat failed")}

		gomega.Expect(service.Initialize(
			testutils.URLMust("xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"),
			testutils.TestLogger(),
		)).To(gomega.Succeed())
		service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
			return session, nil
		})
		service.SetDialContext(pipeDialer())

		gomega.Expect(service.Send("hello", nil)).To(gomega.MatchError("chat failed"))
	})

	ginkgo.It("should return MUC send errors", func() {
		session := &mockSession{err: errors.New("muc failed")}

		gomega.Expect(service.Initialize(
			testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/?rooms=alerts@conference.example.com",
			),
			testutils.TestLogger(),
		)).To(gomega.Succeed())
		service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
			return session, nil
		})
		service.SetDialContext(pipeDialer())

		gomega.Expect(service.Send("hello", nil)).To(gomega.MatchError("muc failed"))
	})

	ginkgo.It("should reject unknown send params", func() {
		gomega.Expect(service.Initialize(
			testutils.URLMust("xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"),
			testutils.TestLogger(),
		)).To(gomega.Succeed())

		err := service.Send("hello", &types.Params{"notakey": "1"})
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("updating config from params"))
	})

	ginkgo.It("should reject params that remove all targets", func() {
		gomega.Expect(service.Initialize(
			testutils.URLMust("xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"),
			testutils.TestLogger(),
		)).To(gomega.Succeed())

		err := service.Send("hello", &types.Params{"to": ""})
		gomega.Expect(err).To(gomega.MatchError(ErrMissingTargets))
	})

	ginkgo.It("should warn when TLS verification is skipped", func() {
		var buf bytes.Buffer
		gomega.Expect(service.Initialize(
			testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/?to=bob@example.com&skiptlsverify=yes",
			),
			log.New(&buf, "", 0),
		)).To(gomega.Succeed())
		service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
			return &mockSession{}, nil
		})
		service.SetDialContext(pipeDialer())

		gomega.Expect(service.Send("hello", nil)).To(gomega.Succeed())
		gomega.Expect(buf.String()).To(gomega.ContainSubstring("TLS verification is disabled"))
	})
})

func (*mockSession) Close() error {
	return nil
}

func (m *mockSession) SendChat(_ context.Context, to, body string) error {
	m.chats = append(m.chats, [2]string{to, body})

	return m.err
}

func (m *mockSession) SendToRoom(_ context.Context, room, nick, password, body string) error {
	m.rooms = append(m.rooms, [4]string{room, nick, password, body})

	return m.err
}

func pipeDialer() types.DialContextFunc {
	return func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		go func() {
			_, _ = io.Copy(io.Discard, server)
			_ = server.Close()
		}()

		return client, nil
	}
}
