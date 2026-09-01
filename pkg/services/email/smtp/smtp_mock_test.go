package smtp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"
)

// mockSMTP is a scripted SMTP listener used to exercise TLS, greetings, and DATA I/O.
type mockSMTP struct {
	implicitTLS       bool
	advertiseStartTLS bool
	acceptStartTLS    bool
	authMechs         string
	failGreeting      bool
	closeAfter354     bool
	hangAfter         string

	tlsConfig *tls.Config
	firstByte chan byte
	done      chan struct{}
	closeDone sync.Once

	mu        sync.Mutex
	commands  []string
	data      bytes.Buffer
	tlsActive bool
}

type firstByteConn struct {
	net.Conn

	first chan byte
	once  sync.Once
}

func newMockSMTP() *mockSMTP {
	return &mockSMTP{
		firstByte: make(chan byte, 1),
		done:      make(chan struct{}),
	}
}

func (s *mockSMTP) commandsCopy() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.commands...)
}

func (s *mockSMTP) dataString() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.data.String()
}

func (s *mockSMTP) hang() {
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
	}
}

func (s *mockSMTP) record(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.commands = append(s.commands, line)
}

func mustSelfSignedCert(ip net.IP) tls.Certificate {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		panic(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "mock.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{ip},
		DNSNames:     []string{"localhost", "mock.local"},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		panic(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}

	return cert
}

func (c *firstByteConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.once.Do(func() {
			select {
			case c.first <- p[0]:
			default:
			}
		})
	}

	return n, err
}

func startMockSMTP(cfg *mockSMTP) (string, func()) {
	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	host, _, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		panic(err)
	}

	ip := net.ParseIP(host)
	cert := mustSelfSignedCert(ip)
	cfg.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		cfg.serve(&firstByteConn{Conn: conn, first: cfg.firstByte})
	}()

	return listener.Addr().String(), func() {
		cfg.closeDone.Do(func() {
			close(cfg.done)
		})

		_ = listener.Close()
	}
}

func (s *mockSMTP) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	if s.hangAfter == "" {
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	}

	if s.implicitTLS {
		tlsConn := tls.Server(conn, s.tlsConfig)
		if err := tlsConn.HandshakeContext(context.Background()); err != nil {
			return
		}

		conn = tlsConn
		s.tlsActive = true
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	write := func(line string) {
		_, _ = writer.WriteString(line + "\r\n")
		_ = writer.Flush()
	}

	if s.failGreeting {
		write("500 not a greeting")

		return
	}

	write("220 mock.local ESMTP")

	if s.hangAfter == "220" {
		s.hang()

		return
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimRight(line, "\r\n")
		s.record(line)

		cmd, _, _ := strings.Cut(line, " ")
		cmd = strings.ToUpper(cmd)

		switch cmd {
		case "EHLO", "HELO":
			write("250-mock.local")

			if s.advertiseStartTLS && !s.tlsActive {
				write("250-STARTTLS")
			}

			if s.authMechs != "" {
				write("250-AUTH " + s.authMechs)
			}

			write("250 8BITMIME")

			if s.hangAfter == "ehlo" {
				s.hang()

				return
			}
		case "STARTTLS":
			if !s.acceptStartTLS || s.tlsActive {
				write("502 STARTTLS not available")

				continue
			}

			write("220 Ready to start TLS")

			tlsConn := tls.Server(conn, s.tlsConfig)
			if err := tlsConn.HandshakeContext(context.Background()); err != nil {
				return
			}

			conn = tlsConn
			s.tlsActive = true
			reader = bufio.NewReader(conn)
			writer = bufio.NewWriter(conn)
		case "AUTH":
			write("235 2.7.0 Accepted")

			if s.hangAfter == "auth" {
				s.hang()

				return
			}
		case "MAIL":
			write("250 2.1.0 Sender OK")
		case "RCPT":
			write("250 2.1.5 Recipient OK")
		case "DATA":
			write("354 Go ahead")

			if s.closeAfter354 {
				return
			}

			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}

				s.mu.Lock()
				s.data.WriteString(dataLine)
				s.mu.Unlock()

				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
			}

			write("250 2.0.0 Message accepted")
		case "RSET":
			write("250 2.0.0 Reset OK")
		case "QUIT":
			write("221 2.0.0 Bye")

			return
		default:
			write("502 5.5.1 Unrecognized")
		}
	}
}
