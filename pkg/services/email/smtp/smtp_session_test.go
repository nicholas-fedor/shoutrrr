package smtp

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
)

var _ = ginkgo.Describe("session", func() {
	ginkgo.BeforeEach(func() {
		service = &Service{}
	})

	ginkgo.Describe("ehloName", func() {
		ginkgo.When("clienthost is set to auto", func() {
			ginkgo.It("should return the os hostname", func() {
				hostname, err := os.Hostname()
				gomega.Expect(err).ToNot(gomega.HaveOccurred())
				gomega.Expect((&session{config: &Config{ClientHost: "auto"}, svc: service}).ehloName()).
					To(gomega.Equal(hostname))
			})
		})

		ginkgo.When("clienthost is set to a custom value", func() {
			ginkgo.It("should return that value", func() {
				gomega.Expect((&session{config: &Config{ClientHost: "computah"}, svc: service}).ehloName()).
					To(gomega.Equal("computah"))
			})
		})
	})

	ginkgo.Describe("auth", func() {
		ginkgo.It("should succeed without AUTH when Auth is None", func() {
			sess := &session{config: &Config{Auth: AuthTypes.None}, svc: service}
			gomega.Expect(sess.auth()).To(gomega.Succeed())
		})

		ginkgo.It("should fail when Auth is Unknown", func() {
			sess := &session{config: &Config{Auth: AuthTypes.Unknown}, svc: service}
			gomega.Expect(sess.auth()).To(matchFailure(FailAuthType))
		})
	})

	ginkgo.Describe("closeIfOpen", func() {
		ginkgo.It("should not close an already closed session", func() {
			textCon, _ := testutils.CreateTextConFaker(nil, "\r\n")
			client := &smtp.Client{Text: textCon}
			mock := &mockConn{}
			injectConn(client, mock)

			sess := &session{closed: true, client: client, svc: service}
			sess.closeIfOpen()
			gomega.Expect(mock.closeCount).To(gomega.BeZero())

			sess.closed = false
			sess.closeIfOpen()
			gomega.Expect(mock.closeCount).To(gomega.Equal(1))
			gomega.Expect(sess.closed).To(gomega.BeTrue())
		})
	})

	ginkgo.When("sending an HTML message", func() {
		ginkgo.It("should use the same multipart boundary in the Content-Type header and body", func() {
			gomega.Expect(service.Initialize(testutils.URLMust(baseNoAuthURL), logger)).To(gomega.Succeed())

			textCon, tcfaker := testutils.CreateTextConFaker([]string{
				"250-mx.google.com at your service",
				"250 8BITMIME",
				"250 Sender OK",
				"250 Receiver OK",
				"354 Go ahead",
				"250 Data OK",
				"221 OK",
			}, "\r\n")
			client := &smtp.Client{Text: textCon}
			fakeTLSEnabled(client, "example.com")

			config := service.Config.Clone()
			config.UseHTML = true
			gomega.Expect(runSMTPSession(service, client, &config)).To(gomega.Succeed())

			received := tcfaker.GetClientSentences()
			boundary := ""

			for _, line := range received {
				if rest, ok := strings.CutPrefix(line, "Content-Type: multipart/alternative; boundary="); ok {
					boundary = rest

					break
				}
			}

			gomega.Expect(boundary).NotTo(gomega.BeEmpty())
			gomega.Expect(received).To(gomega.ContainElement("--" + boundary))
			gomega.Expect(received).To(gomega.ContainElement("--" + boundary + "--"))

			payload := dataPayload(received)
			msg, err := mail.ReadMessage(strings.NewReader(payload))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(msg.Header.Get("MIME-Version")).To(gomega.Equal("1.0"))
			_, parseErr := time.Parse(time.RFC1123Z, msg.Header.Get("Date"))
			gomega.Expect(parseErr).NotTo(gomega.HaveOccurred())

			mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(mediaType).To(gomega.Equal("multipart/alternative"))

			mr := multipart.NewReader(msg.Body, params["boundary"])
			plainPart, err := mr.NextPart()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			plainBody, err := io.ReadAll(plainPart)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(plainBody)).To(gomega.ContainSubstring("Test message"))

			htmlPart, err := mr.NextPart()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			htmlBody, err := io.ReadAll(htmlPart)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(htmlBody)).To(gomega.ContainSubstring("Test message"))
		})
	})

	ginkgo.Describe("message body", func() {
		ginkgo.It("should send an empty body without error", func() {
			gomega.Expect(service.Initialize(testutils.URLMust(baseNoAuthURL), logger)).To(gomega.Succeed())

			textCon, tcfaker := testutils.CreateTextConFaker([]string{
				"250-mx.google.com at your service",
				"250 8BITMIME",
				"250 Sender OK",
				"250 Receiver OK",
				"354 Go ahead",
				"250 Data OK",
				"221 OK",
			}, "\r\n")
			client := &smtp.Client{Text: textCon}
			fakeTLSEnabled(client, "example.com")
			gomega.Expect(runSMTPSessionMessage(service, client, service.Config, "")).To(gomega.Succeed())
			gomega.Expect(tcfaker.GetClientSentences()).To(gomega.ContainElement("DATA"))
		})
	})

	ginkgo.Describe("recipient headers", func() {
		ginkgo.It("should not emit Cc or Bcc headers for envelope-only recipients", func() {
			gomega.Expect(service.Initialize(testutils.URLMust(baseAuthURL), logger)).To(gomega.Succeed())

			textCon, tcfaker := testutils.CreateTextConFaker([]string{
				"250-mx.google.com at your service",
				"250-AUTH LOGIN PLAIN",
				"250 8BITMIME",
				"235 Accepted",
				"250 Sender OK",
				"250 Receiver OK",
				"354 Go ahead",
				"250 Data OK",
				"250 Reset OK",
				"250 Sender OK",
				"250 Receiver OK",
				"354 Go ahead",
				"250 Data OK",
				"221 OK",
			}, "\r\n")
			client := &smtp.Client{Text: textCon}
			fakeTLSEnabled(client, "example.com")
			gomega.Expect(runSMTPSession(service, client, service.Config)).To(gomega.Succeed())

			received := tcfaker.GetClientSentences()
			gomega.Expect(headerLines(received, "To")).To(gomega.Equal([]string{
				"To: <rec1@example.com>",
				"To: <rec2@example.com>",
			}))
			gomega.Expect(headerLines(received, "Cc")).To(gomega.BeEmpty())
			gomega.Expect(headerLines(received, "Bcc")).To(gomega.BeEmpty())
		})
	})

	ginkgo.When("running integration tests", func() {
		ginkgo.When("given a typical usage case configuration URL", func() {
			ginkgo.It("should send notifications without any errors", func() {
				err := testIntegration(baseAuthURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"235 Accepted",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"250 Reset OK",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "<pre>{{ .message }}</pre>", "{{ .message }}",
					"AUTH PLAIN AHVzZXIAcGFzc3dvcmQ=",
					"To: <rec1@example.com>",
					"To: <rec2@example.com>",
				)
				skipIfTestSetup(err)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.When("given auth=Login", func() {
			ginkgo.It("should complete the multi-step AUTH LOGIN dialog", func() {
				testURL := modifyURL(baseAuthURL, map[string]string{
					"auth":    "Login",
					"useHTML": "no",
				})

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"334 VXNlciBOYW1lAA==",
					"334 UGFzc3dvcmQ=",
					"235 Accepted",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"250 Reset OK",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "", "",
					"AUTH LOGIN",
					"dXNlcg==",
					"cGFzc3dvcmQ=",
				)
				skipIfTestSetup(err)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("should fail when the server rejects LOGIN with 535", func() {
				testURL := modifyURL(baseAuthURL, map[string]string{
					"auth":    "Login",
					"useHTML": "no",
				})

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"334 VXNlciBOYW1lAA==",
					"334 UGFzc3dvcmQ=",
					"535 5.7.8 Authentication failed",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailAuthenticating))
			})
		})

		ginkgo.When("given auth=OAuth2", func() {
			ginkgo.It("should send AUTH XOAUTH2 with the Bearer token and succeed on 235", func() {
				testURL := modifyURL(baseAuthURL, map[string]string{
					"auth":    "OAuth2",
					"useHTML": "no",
				})

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-AUTH XOAUTH2",
					"250 8BITMIME",
					"235 Accepted",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"250 Reset OK",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "", "",
					"AUTH XOAUTH2 dXNlcj11c2VyAWF1dGg9QmVhcmVyIHBhc3N3b3JkAQE=",
				)
				skipIfTestSetup(err)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("should fail AUTH when the server returns 334 then 535", func() {
				testURL := modifyURL(baseAuthURL, map[string]string{
					"auth":    "OAuth2",
					"useHTML": "no",
				})

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-AUTH XOAUTH2",
					"250 8BITMIME",
					"334 eyJzdGF0dXMiOiI0MDAifQ==",
					"535 5.7.8 Username and Password not accepted",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailAuthenticating))
				gomega.Expect(err.Error()).NotTo(gomega.ContainSubstring("password"))
			})
		})

		ginkgo.When("given auth=CRAMMD5", func() {
			ginkgo.It("should respond to the challenge with an HMAC-MD5 digest", func() {
				testURL := modifyURL(baseAuthURL, map[string]string{
					"auth":    "CRAMMD5",
					"useHTML": "no",
				})
				challenge := "<12345.67890@example.com>"
				mac := hmac.New(md5.New, []byte("password"))
				_, _ = mac.Write([]byte(challenge))
				digest := hex.EncodeToString(mac.Sum(nil))
				response := base64.StdEncoding.EncodeToString([]byte("user " + digest))

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-AUTH CRAM-MD5",
					"250 8BITMIME",
					"334 " + base64.StdEncoding.EncodeToString([]byte(challenge)),
					"235 Accepted",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"250 Reset OK",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "", "",
					"AUTH CRAM-MD5",
					response,
				)
				skipIfTestSetup(err)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.When("AUTH is not advertised", func() {
			ginkgo.It("should still attempt AUTH and fail when the server rejects it", func() {
				err := testIntegration(baseAuthURL, []string{
					"250-mx.google.com at your service",
					"250 8BITMIME",
					"503 5.5.1 AUTH not advertised",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailAuthenticating))
			})
		})

		ginkgo.When("credentials are empty", func() {
			ginkgo.It("should send AUTH PLAIN with an empty identity payload", func() {
				testURL := "smtp://example.com:2225/?auth=Plain&useStartTLS=no" +
					"&fromAddress=sender@example.com&toAddresses=rec1@example.com&timeout=10s"

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-AUTH PLAIN",
					"250 8BITMIME",
					"535 5.7.8 empty credentials",
				}, "", "",
					"AUTH PLAIN AAA=",
				)
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailAuthenticating))
			})
		})

		ginkgo.When("PLAIN is used without TLS on a remote host", func() {
			ginkgo.It("should refuse to send credentials", func() {
				err := testIntegrationWithoutTLS(baseAuthURL, []string{
					"250-mx.google.com at your service",
					"250-AUTH PLAIN",
					"250 8BITMIME",
				})
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailAuthenticating))
			})
		})

		ginkgo.When("encryption is None", func() {
			ginkgo.It("should still issue STARTTLS when usestarttls is yes", func() {
				testURL := modifyURL(baseNoAuthURL, map[string]string{
					"encryption":  "None",
					"useStartTLS": "yes",
				})

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-STARTTLS",
					"250 8BITMIME",
					"502 That's too hard",
				}, "", "",
					"STARTTLS",
				)
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailEnableStartTLS))
			})

			ginkgo.It("should not issue STARTTLS when usestarttls is no", func() {
				testURL := modifyURL(baseNoAuthURL, map[string]string{
					"encryption":  "None",
					"useStartTLS": "no",
				})

				gomega.Expect(service.Initialize(testutils.URLMust(testURL), logger)).To(gomega.Succeed())

				textCon, tcfaker := testutils.CreateTextConFaker([]string{
					"250-mx.google.com at your service",
					"250-STARTTLS",
					"250 8BITMIME",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "\r\n")
				client := &smtp.Client{Text: textCon}
				fakeTLSEnabled(client, "example.com")
				gomega.Expect(runSMTPSession(service, client, service.Config)).To(gomega.Succeed())
				gomega.Expect(tcfaker.GetClientSentences()).NotTo(gomega.ContainElement("STARTTLS"))
			})
		})

		ginkgo.When("no recipients are configured at send time", func() {
			ginkgo.It("should fail with ErrToAddressMissing", func() {
				gomega.Expect(service.Initialize(testutils.URLMust(baseNoAuthURL), logger)).To(gomega.Succeed())

				textCon, _ := testutils.CreateTextConFaker(nil, "\r\n")
				client := &smtp.Client{Text: textCon}
				config := service.Config.Clone()
				config.ToAddresses = nil
				err := runSMTPSession(service, client, &config)
				gomega.Expect(err).To(matchFailure(FailSendRecipient))
				gomega.Expect(err).To(gomega.MatchError(ErrToAddressMissing))
			})
		})

		ginkgo.When("given e-mail addresses with pluses in the configuration URL", func() {
			ginkgo.It("should send notifications without any errors", func() {
				err := testIntegration(basePlusURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"235 Accepted",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"250 Reset OK",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "<pre>{{ .message }}</pre>", "{{ .message }}",
					"RCPT TO:<rec1+tag@example.com>",
					"To: <rec1+tag@example.com>",
					"From: <sender+tag@example.com>")
				skipIfTestSetup(err)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.When("given a configuration URL with authentication disabled", func() {
			ginkgo.It("should send notifications without any errors", func() {
				err := testIntegration(baseNoAuthURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.When("given a configuration URL with StartTLS but it is not supported", func() {
			ginkgo.It("should send notifications without any errors", func() {
				testURL := modifyURL(baseNoAuthURL, map[string]string{"useStartTLS": "yes"})

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})

		ginkgo.When("StartTLS is required but unsupported", func() {
			ginkgo.It("should fail with FailEnableStartTLS", func() {
				testURL := modifyURL(baseNoAuthURL, map[string]string{
					"useStartTLS":     "yes",
					"requirestarttls": "yes",
				})

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailEnableStartTLS))
			})
		})

		ginkgo.When("server communication fails", func() {
			ginkgo.It("should fail when initial handshake is not accepted", func() {
				testURL := modifyURL(
					baseNoAuthURL,
					map[string]string{"useStartTLS": "yes", "clienthost": "spammer"},
				)

				err := testIntegration(testURL, []string{
					"421 4.7.0 Try again later, closing connection. (EHLO) r20-20020a50d694000000b004588af8956dsm771862edi.9 - gsmtp",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailHandshake))
			})

			ginkgo.It("should fail when not being able to enable StartTLS", func() {
				testURL := modifyURL(baseNoAuthURL, map[string]string{"useStartTLS": "yes"})

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-STARTTLS",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"502 That's too hard",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailEnableStartTLS))
			})

			ginkgo.It("should fail when authentication type is invalid", func() {
				testURL := modifyURL(baseNoAuthURL, map[string]string{"auth": "bad"})

				err := testIntegration(testURL, []string{}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(standard.FailServiceInit))
			})

			ginkgo.It("should fail when not being able to use authentication type", func() {
				testURL := modifyURL(baseNoAuthURL, map[string]string{"auth": "crammd5"})

				err := testIntegration(testURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"504 Liar",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailAuthenticating))
			})

			ginkgo.It("should fail MAIL FROM with FailSetSender", func() {
				err := testSendRecipient([]string{
					"250 mx.google.com at your service",
					"550 sender rejected",
				})
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailSetSender))
			})

			ginkgo.It("should reject a From address that contains CR or LF", func() {
				gomega.Expect(service.Initialize(testutils.URLMust(baseNoAuthURL), logger)).To(gomega.Succeed())

				textCon, _ := testutils.CreateTextConFaker([]string{
					"250 mx.google.com at your service",
				}, "\r\n")
				client := &smtp.Client{Text: textCon}
				config := service.Config.Clone()
				config.FromAddress = "sender@example.com\r\nBcc: evil@x.com"
				err := (&session{client: client, config: &config, svc: service}).
					sendMessage("rec1@example.com", "body", "")
				gomega.Expect(err).To(matchFailure(FailSetSender))
				gomega.Expect(err).To(gomega.MatchError(errHeaderBreak))
			})

			ginkgo.It("should reject a recipient address that contains CR or LF", func() {
				gomega.Expect(service.Initialize(testutils.URLMust(baseNoAuthURL), logger)).To(gomega.Succeed())

				textCon, _ := testutils.CreateTextConFaker([]string{
					"250 mx.google.com at your service",
				}, "\r\n")
				client := &smtp.Client{Text: textCon}
				config := service.Config.Clone()
				err := (&session{client: client, config: &config, svc: service}).
					sendMessage("rec1@example.com\r\nBcc: evil@x.com", "body", "")
				gomega.Expect(err).To(matchFailure(FailSetRecipient))
				gomega.Expect(err).To(gomega.MatchError(errHeaderBreak))
			})

			ginkgo.It("should fail when not being able to send to recipient", func() {
				err := testIntegration(baseNoAuthURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"551 I don't know you",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailSendRecipient))
			})

			ginkgo.It("should fail when the recipient is not accepted", func() {
				err := testSendRecipient([]string{
					"250 mx.google.com at your service",
					"250 Sender OK",
					"553 She doesn't want to be disturbed",
				})
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailSetRecipient))
			})

			ginkgo.It("should fail when the server does not accept the data stream", func() {
				err := testSendRecipient([]string{
					"250 mx.google.com at your service",
					"250 Sender OK",
					"250 Receiver OK",
					"554 Nah I'm fine thanks",
				})
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailOpenDataStream))
			})

			ginkgo.It("should fail when the server does not accept the data stream content", func() {
				err := testSendRecipient([]string{
					"250 mx.google.com at your service",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"554 Such garbage!",
				})
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailCloseDataStream))
			})

			ginkgo.It("should fail when the server does not close the connection gracefully", func() {
				err := testIntegration(baseNoAuthURL, []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"502 You can't quit, you're fired!",
				}, "", "")
				skipIfTestSetup(err)
				gomega.Expect(err).To(matchFailure(FailClosingSession))
			})

			ginkgo.It("should ignore short response errors on QUIT and log warning", func() {
				serviceURL, _ := url.Parse(baseNoAuthURL)
				localService := &Service{}
				gomega.Expect(localService.Initialize(serviceURL, logger)).To(gomega.Succeed())

				var buf bytes.Buffer

				testLogger := log.New(&buf, "", 0)
				localService.SetLogger(testLogger)

				responses := []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"22",
				}
				textCon, _ := testutils.CreateTextConFaker(responses, "\r\n")
				client := &smtp.Client{Text: textCon}
				fakeTLSEnabled(client, serviceURL.Hostname())

				gomega.Expect(runSMTPSession(localService, client, localService.Config)).To(gomega.Succeed())
				gomega.Expect(buf.String()).
					To(gomega.ContainSubstring("Warning: Ignoring session closure error (delivery succeeded)"))
			})

			ginkgo.It("should not close again after a successful QUIT", func() {
				serviceURL, _ := url.Parse(baseNoAuthURL)
				localService := &Service{}
				gomega.Expect(localService.Initialize(serviceURL, logger)).To(gomega.Succeed())

				var buf bytes.Buffer

				testLogger := log.New(&buf, "", 0)
				localService.SetLogger(testLogger)

				responses := []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}
				textCon, _ := testutils.CreateTextConFaker(responses, "\r\n")
				client := &smtp.Client{Text: textCon}
				fakeTLSEnabled(client, serviceURL.Hostname())

				mock := &mockConn{}
				injectConn(client, mock)

				gomega.Expect(runSMTPSession(localService, client, localService.Config)).To(gomega.Succeed())
				gomega.Expect(buf.String()).
					NotTo(gomega.ContainSubstring("Failed to close SMTP client connection"))
				gomega.Expect(mock.closeCount).To(gomega.Equal(1))
			})

			ginkgo.It("should close the client when the handshake fails", func() {
				serviceURL, _ := url.Parse(baseNoAuthURL)
				localService := &Service{}
				gomega.Expect(localService.Initialize(serviceURL, logger)).To(gomega.Succeed())

				responses := []string{
					"421 4.7.0 Try again later, closing connection. (EHLO)",
				}
				textCon, _ := testutils.CreateTextConFaker(responses, "\r\n")
				client := &smtp.Client{Text: textCon}
				fakeTLSEnabled(client, serviceURL.Hostname())

				mock := &mockConn{}
				injectConn(client, mock)

				err := runSMTPSession(localService, client, localService.Config)
				gomega.Expect(err).To(matchFailure(FailHandshake))
				gomega.Expect(mock.closeCount).To(gomega.BeNumerically(">=", 1))
			})

			ginkgo.It("should attempt all recipients and collect errors", func() {
				serviceURL, _ := url.Parse(baseAuthURL)
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				responses := []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"235 Accepted",
					"250 Sender OK",
					"553 Recipient1 not found",
					"250 Reset OK",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"221 OK",
				}
				textCon, tcfaker := testutils.CreateTextConFaker(responses, "\r\n")
				client := &smtp.Client{Text: textCon}
				fakeTLSEnabled(client, serviceURL.Hostname())

				config := service.Config
				config.ToAddresses = []string{"rec1@example.com", "rec2@example.com"}
				err := runSMTPSession(service, client, config)
				gomega.Expect(err).To(matchFailure(FailSendRecipient))
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("error sending message to recipient \"rec1@example.com\""))

				received := tcfaker.GetClientSentences()
				gomega.Expect(received).To(gomega.ContainElement("RSET"))
				gomega.Expect(received).To(gomega.ContainElement("RCPT TO:<rec2@example.com>"))
				gomega.Expect(received).To(gomega.ContainElement("QUIT"))
				logger.Printf("\n%s", tcfaker.GetConversation(false))
			})

			ginkgo.It("should fail when RSET is rejected between recipients", func() {
				serviceURL, _ := url.Parse(baseAuthURL)
				gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

				responses := []string{
					"250-mx.google.com at your service",
					"250-SIZE 35651584",
					"250-AUTH LOGIN PLAIN",
					"250 8BITMIME",
					"235 Accepted",
					"250 Sender OK",
					"250 Receiver OK",
					"354 Go ahead",
					"250 Data OK",
					"502 cannot reset",
				}
				textCon, _ := testutils.CreateTextConFaker(responses, "\r\n")
				client := &smtp.Client{Text: textCon}
				fakeTLSEnabled(client, serviceURL.Hostname())

				config := service.Config
				config.ToAddresses = []string{"rec1@example.com", "rec2@example.com"}
				err := runSMTPSession(service, client, config)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("error resetting session between recipients"))
			})
		})
	})
})
