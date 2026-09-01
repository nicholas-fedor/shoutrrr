package smtp

import (
	"mime"
	"net/mail"
	"strings"
	"text/template"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

var _ = ginkgo.Describe("message", func() {
	ginkgo.Describe("headers", func() {
		ginkgo.When("the subject contains non-ASCII characters", func() {
			ginkgo.It("should encode it as an RFC 2047 encoded-word", func() {
				headerMap := headers(
					&Config{FromAddress: "sender@example.com", Subject: "Plan überschritten"},
					"rec1@example.com",
					"",
				)

				gomega.Expect(headerMap["Subject"]).
					To(gomega.Equal("=?UTF-8?q?Plan_=C3=BCberschritten?="))
				gomega.Expect(new(mime.WordDecoder).DecodeHeader(headerMap["Subject"])).
					To(gomega.Equal("Plan überschritten"))
			})
		})

		ginkgo.When("the subject is pure ASCII", func() {
			ginkgo.It("should leave it unencoded", func() {
				headerMap := headers(
					&Config{FromAddress: "sender@example.com", Subject: "Plan exceeded"},
					"rec1@example.com",
					"",
				)

				gomega.Expect(headerMap["Subject"]).To(gomega.Equal("Plan exceeded"))
			})
		})

		ginkgo.When("the subject contains CR or LF", func() {
			ginkgo.It("should encode them so they cannot inject a Bcc header", func() {
				headerMap := headers(
					&Config{FromAddress: "sender@example.com", Subject: "Hello\r\nBcc: evil@x.com"},
					"rec1@example.com",
					"",
				)

				gomega.Expect(headerMap["Subject"]).NotTo(gomega.ContainSubstring("\r"))
				gomega.Expect(headerMap["Subject"]).NotTo(gomega.ContainSubstring("\n"))
				decoded, err := new(mime.WordDecoder).DecodeHeader(headerMap["Subject"])
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(decoded).To(gomega.Equal("Hello\r\nBcc: evil@x.com"))

				writer := &captureWriter{}
				gomega.Expect(writeHeaders(writer, map[string]string{
					"Subject": headerMap["Subject"],
					"From":    headerMap["From"],
				})).To(gomega.Succeed())
				gomega.Expect(writer.String()).NotTo(gomega.ContainSubstring("\nBcc:"))
			})
		})

		ginkgo.When("the sender name contains CR or LF", func() {
			ginkgo.It("should encode them out of the From header value", func() {
				headerMap := headers(&Config{
					FromName:    "Eve\r\nBcc: evil@x.com",
					FromAddress: "sender@example.com",
					Subject:     "Subject",
				}, "rec1@example.com", "")

				gomega.Expect(headerMap["From"]).NotTo(gomega.ContainSubstring("\r"))
				gomega.Expect(headerMap["From"]).NotTo(gomega.ContainSubstring("\n"))
			})
		})

		ginkgo.When("an identifying header is already an encoded-word", func() {
			ginkgo.It("should not wrap an ASCII encoded-word again", func() {
				headerMap := headers(
					&Config{FromAddress: "sender@example.com", Subject: "=?UTF-8?q?Hello?="},
					"rec1@example.com",
					"",
				)

				gomega.Expect(headerMap["Subject"]).To(gomega.Equal("=?UTF-8?q?Hello?="))
			})
		})

		ginkgo.When("the subject is longer than 998 octets", func() {
			ginkgo.It("should fold the value so no physical line exceeds the RFC 5322 limit", func() {
				subject := strings.Repeat("A", 1200)
				headerMap := headers(
					&Config{FromAddress: "sender@example.com", Subject: subject},
					"rec1@example.com",
					"",
				)

				gomega.Expect(strings.ReplaceAll(headerMap["Subject"], "\n ", "")).
					To(gomega.Equal(subject))
				gomega.Expect(headerMap["Subject"]).NotTo(gomega.ContainSubstring("\r"))

				for line := range strings.SplitSeq("Subject: "+headerMap["Subject"], "\n") {
					gomega.Expect(len(line)).To(gomega.BeNumerically("<=", maxHeaderLineOctets))
				}
			})
		})

		ginkgo.When("the sender name contains non-ASCII characters", func() {
			ginkgo.It("should encode it and keep the address parseable", func() {
				headerMap := headers(&Config{
					FromName:    "Grüßer",
					FromAddress: "sender@example.com",
					Subject:     "Subject",
				}, "rec1@example.com", "")

				gomega.Expect(headerMap["From"]).
					To(gomega.Equal("=?utf-8?q?Gr=C3=BC=C3=9Fer?= <sender@example.com>"))

				address, err := mail.ParseAddress(headerMap["From"])
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(address.Name).To(gomega.Equal("Grüßer"))
				gomega.Expect(address.Address).To(gomega.Equal("sender@example.com"))
			})
		})

		ginkgo.When("the sender name or address requires quoting", func() {
			ginkgo.It("should quote them", func() {
				headerMap := headers(&Config{
					FromName:    "Doe, John",
					FromAddress: "odd name@example.com",
					Subject:     "Subject",
				}, "rec1@example.com", "")

				gomega.Expect(headerMap["From"]).
					To(gomega.Equal(`"Doe, John" <"odd name"@example.com>`))
				gomega.Expect(mail.ParseAddress(headerMap["From"])).
					To(gomega.Equal(&mail.Address{
						Name:    "Doe, John",
						Address: `odd name@example.com`,
					}))
			})
		})

		ginkgo.When("no sender name is configured", func() {
			ginkgo.It("should only emit the address", func() {
				headerMap := headers(
					&Config{FromAddress: "sender+tag@example.com", Subject: "Subject"},
					"rec1@example.com",
					"",
				)

				gomega.Expect(headerMap["From"]).To(gomega.Equal("<sender+tag@example.com>"))
			})
		})

		ginkgo.When("the message is HTML", func() {
			ginkgo.It("should put the supplied boundary in the Content-Type header", func() {
				headerMap := headers(&Config{
					FromAddress: "sender@example.com",
					UseHTML:     true,
				}, "rec1@example.com", "abc123")

				gomega.Expect(headerMap["Content-Type"]).
					To(gomega.Equal(`multipart/alternative; boundary=abc123`))
				_, hasCTE := headerMap["Content-Transfer-Encoding"]
				gomega.Expect(hasCTE).To(gomega.BeFalse())
			})
		})

		ginkgo.When("the message is plain text", func() {
			ginkgo.It("should declare 8bit content-transfer-encoding", func() {
				headerMap := headers(
					&Config{FromAddress: "sender@example.com"},
					"rec1@example.com",
					"",
				)

				gomega.Expect(headerMap["Content-Transfer-Encoding"]).To(gomega.Equal("8bit"))
			})
		})

		ginkgo.When("constructing identifying headers", func() {
			ginkgo.It("should generate a Message-ID using the sender domain", func() {
				headerMap := headers(
					&Config{FromAddress: "sender@example.com"},
					"rec1@example.com",
					"",
				)

				gomega.Expect(headerMap["Message-ID"]).
					To(gomega.MatchRegexp(`^<[0-9a-f]+@example\.com>$`))
			})

			ginkgo.It("should format the To address like From", func() {
				headerMap := headers(
					&Config{FromAddress: "sender@example.com"},
					"rec1+tag@example.com",
					"",
				)

				gomega.Expect(headerMap["To"]).To(gomega.Equal("<rec1+tag@example.com>"))
			})

			ginkgo.It("should include a parseable Date and MIME-version", func() {
				headerMap := headers(
					&Config{FromAddress: "sender@example.com"},
					"rec1@example.com",
					"",
				)

				gomega.Expect(headerMap["MIME-version"]).To(gomega.Equal("1.0"))
				_, err := time.Parse(time.RFC1123Z, headerMap["Date"])
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("should encode a non-ASCII recipient local-part", func() {
				headerMap := headers(
					&Config{FromAddress: "sender@example.com"},
					"grüß@example.com",
					"",
				)

				gomega.Expect(headerMap["To"]).NotTo(gomega.Equal("grüß@example.com"))
				address, err := mail.ParseAddress(headerMap["To"])
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(address.Address).To(gomega.Equal("grüß@example.com"))
			})
		})
	})

	ginkgo.Describe("generateMessageID", func() {
		ginkgo.It("should use the sender domain", func() {
			gomega.Expect(generateMessageID("sender@example.com")).
				To(gomega.MatchRegexp(`^<[0-9a-f]{16}@example\.com>$`))
		})

		ginkgo.It("should fall back to localhost when the address has no host", func() {
			gomega.Expect(generateMessageID("nodomain")).
				To(gomega.MatchRegexp(`^<[0-9a-f]{16}@localhost>$`))
			gomega.Expect(generateMessageID("user@")).
				To(gomega.MatchRegexp(`^<[0-9a-f]{16}@localhost>$`))
		})
	})

	ginkgo.Describe("generateBoundary", func() {
		ginkgo.It("should return a unique hex token", func() {
			first, err := generateBoundary()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			second, err := generateBoundary()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(first).To(gomega.MatchRegexp(`^[0-9a-f]{16}$`))
			gomega.Expect(second).To(gomega.MatchRegexp(`^[0-9a-f]{16}$`))
			gomega.Expect(first).NotTo(gomega.Equal(second))
		})
	})

	ginkgo.Describe("writeHeaders", func() {
		ginkgo.It("should write each header and a blank line", func() {
			writer := &captureWriter{}
			gomega.Expect(writeHeaders(writer, map[string]string{"Subject": "Hello"})).To(gomega.Succeed())
			gomega.Expect(writer.String()).To(gomega.Equal("Subject: Hello\n\n"))
		})

		ginkgo.It("should write headers in sorted key order", func() {
			writer := &captureWriter{}
			gomega.Expect(writeHeaders(writer, map[string]string{
				"Subject": "Hello",
				"From":    "<a@b.com>",
			})).To(gomega.Succeed())
			gomega.Expect(writer.String()).To(gomega.Equal("From: <a@b.com>\nSubject: Hello\n\n"))
		})

		ginkgo.It("should fold a long header value to the RFC 5322 line limit", func() {
			writer := &captureWriter{}
			gomega.Expect(writeHeaders(writer, map[string]string{
				"Subject": strings.Repeat("B", 1200),
			})).To(gomega.Succeed())

			body := strings.TrimSuffix(writer.String(), "\n")
			for line := range strings.SplitSeq(body, "\n") {
				gomega.Expect(len(line)).To(gomega.BeNumerically("<=", maxHeaderLineOctets))
			}

			unfolded := strings.ReplaceAll(strings.TrimPrefix(strings.ReplaceAll(body, "\n ", ""), "Subject: "), "\n", "")
			gomega.Expect(unfolded).To(gomega.Equal(strings.Repeat("B", 1200)))
		})

		ginkgo.When("the output stream is closed during header content", func() {
			ginkgo.It("should fail with FailWriteHeaders", func() {
				gomega.Expect(writeHeaders(testutils.CreateFailWriter(0), map[string]string{"key": "value"})).
					To(matchFailure(FailWriteHeaders))
			})
		})

		ginkgo.When("the output stream is closed after header content", func() {
			ginkgo.It("should fail with FailWriteHeaders", func() {
				gomega.Expect(writeHeaders(testutils.CreateFailWriter(1), map[string]string{"key": "value"})).
					To(matchFailure(FailWriteHeaders))
			})
		})
	})

	ginkgo.Describe("writeAlternative", func() {
		ginkgo.It("should write plain and HTML parts with the closing boundary", func() {
			templater := typesmocks.NewMockTemplater(ginkgo.GinkgoT())
			templater.On("GetTemplate", templatePlain).Return(nil, false)
			templater.On("GetTemplate", templateHTML).Return(nil, false)

			writer := &captureWriter{}
			gomega.Expect(writeAlternative(writer, "hello", "abc", templater)).To(gomega.Succeed())

			body := writer.String()
			gomega.Expect(body).To(gomega.ContainSubstring("--abc\n"))
			gomega.Expect(body).To(gomega.ContainSubstring("Content-Type: " + contentPlain))
			gomega.Expect(body).To(gomega.ContainSubstring("Content-Type: " + contentHTML))
			gomega.Expect(strings.Count(body, "hello")).To(gomega.Equal(2))
			gomega.Expect(body).To(gomega.ContainSubstring("--abc--"))
		})

		ginkgo.When("the underlying stream stops working", func() {
			ginkgo.It("should fail when writing multipart plain header", func() {
				err := writeAlternative(testutils.CreateFailWriter(1), "", "boundary", &Service{})
				gomega.Expect(err).To(matchFailure(FailPlainHeader))
			})

			ginkgo.It("should fail when writing multipart plain message", func() {
				err := writeAlternative(testutils.CreateFailWriter(2), "", "boundary", &Service{})
				gomega.Expect(err).To(matchFailure(FailMessageRaw))
			})

			ginkgo.It("should fail when writing multipart HTML header", func() {
				err := writeAlternative(testutils.CreateFailWriter(4), "", "boundary", &Service{})
				gomega.Expect(err).To(matchFailure(FailHTMLHeader))
			})

			ginkgo.It("should fail when writing multipart HTML message", func() {
				err := writeAlternative(testutils.CreateFailWriter(5), "", "boundary", &Service{})
				gomega.Expect(err).To(matchFailure(FailMessageRaw))
			})

			ginkgo.It("should fail when writing multipart end header", func() {
				err := writeAlternative(testutils.CreateFailWriter(6), "", "boundary", &Service{})
				gomega.Expect(err).To(matchFailure(FailMultiEndHeader))
			})
		})
	})

	ginkgo.Describe("writePart", func() {
		ginkgo.It("should write the raw message when no template is registered", func() {
			templater := typesmocks.NewMockTemplater(ginkgo.GinkgoT())
			templater.On("GetTemplate", "HTML").Return(nil, false)

			writer := &captureWriter{}
			gomega.Expect(writePart(writer, "<p>styled</p>", "HTML", templater)).To(gomega.Succeed())
			gomega.Expect(writer.String()).To(gomega.Equal("<p>styled</p>"))
		})

		ginkgo.It("should execute a registered template", func() {
			tpl := template.Must(template.New("plain").Parse("wrapped {{.message}}"))
			templater := typesmocks.NewMockTemplater(ginkgo.GinkgoT())
			templater.On("GetTemplate", templatePlain).Return(tpl, true)

			writer := &captureWriter{}
			gomega.Expect(writePart(writer, "hello", templatePlain, templater)).To(gomega.Succeed())
			gomega.Expect(writer.String()).To(gomega.Equal("wrapped hello"))
		})

		ginkgo.It("should fail when the template cannot be executed", func() {
			tpl := template.Must(template.New("dummy").Parse("{{.message.Bogus}}"))
			templater := typesmocks.NewMockTemplater(ginkgo.GinkgoT())
			templater.On("GetTemplate", "dummy").Return(tpl, true)

			err := writePart(&captureWriter{}, "hello", "dummy", templater)
			gomega.Expect(err).To(matchFailure(FailMessageTemplate))
		})

		ginkgo.It("should fail when writing a template to a closed stream", func() {
			service := Service{}
			gomega.Expect(service.SetTemplateString("dummy", "dummy template content")).To(gomega.Succeed())

			err := writePart(testutils.CreateFailWriter(0), "", "dummy", &service)
			gomega.Expect(err).To(matchFailure(FailMessageTemplate))
		})
	})

	ginkgo.Describe("writePartHeader", func() {
		ginkgo.It("should write a part header with content type", func() {
			writer := &captureWriter{}
			gomega.Expect(writePartHeader(writer, "abc", contentPlain)).To(gomega.Succeed())
			gomega.Expect(writer.String()).To(gomega.HavePrefix("\n\n--abc\n"))
			gomega.Expect(writer.String()).To(gomega.ContainSubstring("Content-Type: " + contentPlain))
			gomega.Expect(writer.String()).To(gomega.ContainSubstring("Content-Transfer-Encoding: 8bit"))
		})

		ginkgo.It("should write the closing boundary when content type is empty", func() {
			writer := &captureWriter{}
			gomega.Expect(writePartHeader(writer, "abc", "")).To(gomega.Succeed())
			gomega.Expect(writer.String()).To(gomega.Equal("\n\n--abc--"))
		})

		ginkgo.It("should fail when the writer rejects the boundary", func() {
			err := writePartHeader(testutils.CreateFailWriter(0), "abc", contentPlain)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("writing multipart boundary"))
		})
	})
})
