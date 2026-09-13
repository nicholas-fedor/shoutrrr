package xmpp

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
)

var _ = ginkgo.Describe("config", func() {
	ginkgo.Describe("SetURL", func() {
		ginkgo.It("should parse a localpart user, host, and to JID", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/?to=bob@example.com",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.User).To(gomega.Equal("alice"))
			gomega.Expect(config.Password).To(gomega.Equal("secret"))
			gomega.Expect(config.Host).To(gomega.Equal("xmpp.example.com"))
			gomega.Expect(config.Port).To(gomega.Equal(DefaultPort))
			gomega.Expect(config.To).To(gomega.Equal([]string{"bob@example.com"}))
			gomega.Expect(config.scheme).To(gomega.Equal(Scheme))
			gomega.Expect(config.authJIDString()).To(gomega.Equal("alice@xmpp.example.com"))
		})

		ginkgo.It("should parse a full auth JID distinct from the dial host", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice%40jabber.example.com:secret@xmpp.example.com/?to=bob@jabber.example.com",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.User).To(gomega.Equal("alice@jabber.example.com"))
			gomega.Expect(config.Host).To(gomega.Equal("xmpp.example.com"))
			gomega.Expect(config.authJIDString()).To(gomega.Equal("alice@jabber.example.com"))
		})

		ginkgo.It("should default xmpps to port 5223", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpps://alice:secret@xmpp.example.com/?to=bob@example.com",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Port).To(gomega.Equal(DefaultTLSPort))
			gomega.Expect(config.scheme).To(gomega.Equal(SchemeTLS))
			gomega.Expect(config.implicitTLS()).To(gomega.BeTrue())
		})

		ginkgo.It("should parse rooms, nick, and roompassword", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/" +
					"?rooms=alerts@conference.example.com&nick=bot&roompassword=s3cret",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Rooms).To(gomega.Equal([]string{"alerts@conference.example.com"}))
			gomega.Expect(config.Nick).To(gomega.Equal("bot"))
			gomega.Expect(config.RoomPassword).To(gomega.Equal("s3cret"))
			gomega.Expect(config.mucNick()).To(gomega.Equal("bot"))
		})

		ginkgo.It("should default nick to the auth JID localpart", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/?rooms=alerts@conference.example.com",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.mucNick()).To(gomega.Equal("alice"))
		})

		ginkgo.It("should reject disabletls on xmpps", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpps://alice:secret@xmpp.example.com/?to=bob@example.com&disabletls=yes",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrDisableTLSOnXMPPS))
		})

		ginkgo.It("should allow disabletls on xmpp", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/?to=bob@example.com&disabletls=yes",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.DisableTLS).To(gomega.BeTrue())
			gomega.Expect(config.useTLS()).To(gomega.BeFalse())
		})

		ginkgo.It("should reject missing targets", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust("xmpp://alice:secret@xmpp.example.com/"))
			gomega.Expect(err).To(gomega.MatchError(ErrMissingTargets))
		})

		ginkgo.It("should reject missing password", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice@xmpp.example.com/?to=bob@example.com",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrMissingPassword))
		})

		ginkgo.It("should reject an invalid recipient JID", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/?to=not-a-jid",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidRecipientJID))
		})

		ginkgo.It("should reject an invalid room JID", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/?rooms=not-a-jid",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidRoomJID))
		})

		ginkgo.It("should reject an invalid auth JID", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice%40:secret@xmpp.example.com/?to=bob@example.com",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidUserJID))
		})

		ginkgo.It("should reject a missing host", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@/?to=bob@example.com",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrMissingHost))
		})

		ginkgo.It("should reject a missing user", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://:secret@xmpp.example.com/?to=bob@example.com",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrMissingUser))
		})

		ginkgo.It("should default an empty scheme to xmpp", func() {
			config := &Config{}
			serviceURL := testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/?to=bob@example.com",
			)
			serviceURL.Scheme = ""
			gomega.Expect(config.SetURL(serviceURL)).To(gomega.Succeed())
			gomega.Expect(config.scheme).To(gomega.Equal(Scheme))
		})

		ginkgo.It("should reject an unsupported scheme", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"http://alice:secret@xmpp.example.com/?to=bob@example.com",
			))
			gomega.Expect(err).To(gomega.MatchError(ErrUnsupportedScheme))
		})

		ginkgo.It("should reject a port that does not fit in uint16", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com:70000/?to=bob@example.com",
			))
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("parsing port"))
		})

		ginkgo.It("should reject an unknown query key", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com/?to=bob@example.com&notakey=1",
			))
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("setting query parameter"))
		})

		ginkgo.It("should parse comma-separated targets and a custom port", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com:5225/" +
					"?to=bob@example.com,carol@example.com&rooms=a@conf.example.com,b@conf.example.com",
			))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.Port).To(gomega.Equal(uint16(5225)))
			gomega.Expect(config.To).To(gomega.Equal([]string{"bob@example.com", "carol@example.com"}))
			gomega.Expect(config.Rooms).To(gomega.Equal([]string{"a@conf.example.com", "b@conf.example.com"}))
		})

		ginkgo.It("should accept the dummy docs URL", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust("xmpp://dummy@dummy.com"))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should accept the dummy xmpps docs URL", func() {
			config := &Config{}
			err := config.SetURL(testutils.URLMust("xmpps://dummy@dummy.com"))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(config.scheme).To(gomega.Equal(SchemeTLS))
			gomega.Expect(config.Port).To(gomega.Equal(DefaultTLSPort))
		})
	})

	ginkgo.Describe("GetURL", func() {
		ginkgo.It("should round-trip a localpart URL", func() {
			original := testutils.URLMust(
				"xmpp://alice:secret@xmpp.example.com:5222/?to=bob@example.com",
			)
			config := &Config{}
			gomega.Expect(config.SetURL(original)).To(gomega.Succeed())

			parsed := &Config{}
			gomega.Expect(parsed.SetURL(config.GetURL())).To(gomega.Succeed())
			gomega.Expect(parsed.User).To(gomega.Equal(config.User))
			gomega.Expect(parsed.Password).To(gomega.Equal(config.Password))
			gomega.Expect(parsed.Host).To(gomega.Equal(config.Host))
			gomega.Expect(parsed.Port).To(gomega.Equal(config.Port))
			gomega.Expect(parsed.To).To(gomega.Equal(config.To))
			gomega.Expect(parsed.scheme).To(gomega.Equal(Scheme))
		})

		ginkgo.It("should preserve the xmpps scheme", func() {
			config := &Config{}
			gomega.Expect(config.SetURL(testutils.URLMust(
				"xmpps://alice:secret@xmpp.example.com/?to=bob@example.com",
			))).To(gomega.Succeed())
			gomega.Expect(config.GetURL().Scheme).To(gomega.Equal(SchemeTLS))
		})

		ginkgo.It("should default scheme and port when they are unset", func() {
			config := &Config{Host: "xmpp.example.com", User: "alice", Password: "secret"}
			parsed := config.GetURL()
			gomega.Expect(parsed.Scheme).To(gomega.Equal(Scheme))
			gomega.Expect(parsed.Port()).To(gomega.Equal("5222"))
			gomega.Expect(parsed.User.Username()).To(gomega.Equal("alice"))
		})

		ginkgo.It("should omit userinfo when the user is empty", func() {
			config := &Config{Host: "xmpp.example.com", scheme: Scheme, Port: DefaultPort}
			gomega.Expect(config.GetURL().User).To(gomega.BeNil())
		})
	})

	ginkgo.Describe("mucNick", func() {
		ginkgo.It("should fall back to the resource name when the auth JID is invalid", func() {
			config := &Config{User: "alice"}
			gomega.Expect(config.mucNick()).To(gomega.Equal(Resource))
		})
	})

	ginkgo.Describe("splitJID", func() {
		ginkgo.It("should strip a resource and reject empty or nested domains", func() {
			local, domain, err := splitJID("alice@jabber.example.com/phone")
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(local).To(gomega.Equal("alice"))
			gomega.Expect(domain).To(gomega.Equal("jabber.example.com"))

			_, _, err = splitJID("alice@")
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidUserJID))

			_, _, err = splitJID("alice@jabber@example.com")
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidUserJID))
		})
	})

	ginkgo.Describe("composeMessage", func() {
		ginkgo.It("should return the body when title is empty", func() {
			gomega.Expect(composeMessage("", "hello")).To(gomega.Equal("hello"))
		})

		ginkgo.It("should prepend the title", func() {
			gomega.Expect(composeMessage("Alert", "hello")).To(gomega.Equal("Alert\nhello"))
		})

		ginkgo.It("should return the title when the body is empty", func() {
			gomega.Expect(composeMessage("Alert", "")).To(gomega.Equal("Alert"))
		})
	})
})
