package xmpp

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("client", func() {
	ginkgo.Describe("newTLSConfig", func() {
		ginkgo.It("should copy host and skip-verify onto the TLS config", func() {
			cfg := &Config{Host: "xmpp.example.com", SkipTLSVerify: true}
			tlsCfg := newTLSConfig(cfg)
			gomega.Expect(tlsCfg.ServerName).To(gomega.Equal("xmpp.example.com"))
			gomega.Expect(tlsCfg.MinVersion).To(gomega.Equal(uint16(tls.VersionTLS12)))
			gomega.Expect(tlsCfg.InsecureSkipVerify).To(gomega.BeTrue())
		})

		ginkgo.It("should verify certificates by default", func() {
			cfg := &Config{Host: "xmpp.example.com"}
			gomega.Expect(newTLSConfig(cfg).InsecureSkipVerify).To(gomega.BeFalse())
		})
	})

	ginkgo.Describe("melliumSession", func() {
		ginkgo.It("should no-op Close when the session is unset", func() {
			session := &melliumSession{}
			gomega.Expect(session.Close()).To(gomega.Succeed())
		})

		ginkgo.It("should reject an invalid chat JID before encoding", func() {
			session := &melliumSession{}
			err := session.SendChat(context.Background(), "alice@", "hello")
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidRecipientJID))
		})

		ginkgo.It("should reject an invalid room JID before joining", func() {
			session := &melliumSession{}
			err := session.SendToRoom(context.Background(), "alice@", "nick", "", "hello")
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidRoomJID))
		})
	})

	ginkgo.Describe("newMelliumSession", func() {
		ginkgo.It("should reject an invalid auth JID", func() {
			cfg := &Config{User: "alice@", Host: "xmpp.example.com", Password: "secret"}
			_, err := newMelliumSession(context.Background(), cfg, nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should fail implicit TLS handshake on a closed pipe", func() {
			client, server := net.Pipe()
			gomega.Expect(server.Close()).To(gomega.Succeed())

			cfg := &Config{
				User:          "alice",
				Host:          "localhost",
				Password:      "secret",
				scheme:        SchemeTLS,
				SkipTLSVerify: true,
			}
			_, err := newMelliumSession(context.Background(), cfg, client)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("TLS handshake"))
		})

		ginkgo.It("should fail plaintext negotiation on a closed pipe", func() {
			client, server := net.Pipe()
			gomega.Expect(server.Close()).To(gomega.Succeed())

			cfg := &Config{
				User:       "alice",
				Host:       "localhost",
				Password:   "secret",
				scheme:     Scheme,
				DisableTLS: true,
			}
			_, err := newMelliumSession(context.Background(), cfg, client)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("negotiating XMPP session"))
		})

		ginkgo.It("should fail STARTTLS negotiation on a closed pipe", func() {
			client, server := net.Pipe()
			gomega.Expect(server.Close()).To(gomega.Succeed())

			cfg := &Config{
				User:     "alice",
				Host:     "localhost",
				Password: "secret",
				scheme:   Scheme,
			}
			_, err := newMelliumSession(context.Background(), cfg, client)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("negotiating XMPP session"))
		})
	})
})
