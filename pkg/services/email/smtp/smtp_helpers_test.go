package smtp

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"net/smtp"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"
	"unsafe"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	gomegaTypes "github.com/onsi/gomega/types"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
)

type mockConn struct {
	closeCount int
}

type captureWriter struct {
	bytes.Buffer
}

const (
	urlWithAllProps = "smtp://user:password@example.com:2225/?auth=None&clienthost=testhost&encryption=ExplicitTLS&fromaddress=sender%40example.com&fromname=Sender&subject=Subject&toaddresses=rec1%40example.com%2Crec2%40example.com&usehtml=Yes&usestarttls=No&timeout=10s"
	baseNoAuthURL   = "smtp://example.com:2225/?useStartTLS=no&auth=none&fromAddress=sender@example.com&toAddresses=rec1@example.com&useHTML=no&timeout=10s"
	baseAuthURL     = "smtp://user:password@example.com:2225/?useStartTLS=no&fromAddress=sender@example.com&toAddresses=rec1@example.com,rec2@example.com&useHTML=yes&timeout=10s"
	basePlusURL     = "smtp://user:password@example.com:2225/?useStartTLS=no&fromAddress=sender+tag@example.com&toAddresses=rec1+tag@example.com,rec2@example.com&useHTML=yes&timeout=10s"
)

var (
	service    *Service
	envSMTPURL string
	logger     *log.Logger
)

var _ = ginkgo.BeforeSuite(func() {
	envSMTPURL = os.Getenv("SHOUTRRR_SMTP_URL")
	logger = testutils.TestLogger()
})

func (c *captureWriter) Close() error {
	return nil
}

func (m *mockConn) Close() error {
	m.closeCount++

	_, _ = ginkgo.GinkgoWriter.Write([]byte("mockConn.Close called\n"))

	if m.closeCount > 1 {
		return errors.New("mock close error")
	}

	return nil
}

func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) Read(_ []byte) (int, error)         { return 0, nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(_ time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(_ time.Time) error { return nil }
func (m *mockConn) Write(b []byte) (int, error)        { return len(b), nil }

func injectConn(client *smtp.Client, conn net.Conn) {
	cr := reflect.ValueOf(client).Elem().FieldByName("Text").Elem().FieldByName("conn")
	cr = reflect.NewAt(cr.Type(), unsafe.Pointer(cr.UnsafeAddr())).Elem()
	cr.Set(reflect.ValueOf(conn))
}

func fakeTLSEnabled(client *smtp.Client, hostname string) {
	cr := reflect.ValueOf(client).Elem().FieldByName("tls")
	cr = reflect.NewAt(cr.Type(), unsafe.Pointer(cr.UnsafeAddr())).Elem()
	cr.SetBool(true)

	cr = reflect.ValueOf(client).Elem().FieldByName("serverName")
	cr = reflect.NewAt(cr.Type(), unsafe.Pointer(cr.UnsafeAddr())).Elem()
	cr.SetString(hostname)
}

func startGreetingServer() (string, <-chan byte, func()) {
	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	firstByte := make(chan byte, 1)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		_ = conn.SetDeadline(time.Now().Add(time.Second))
		_, _ = conn.Write([]byte("220 mock.local ESMTP\r\n"))

		received := make([]byte, 1)
		if _, err := conn.Read(received); err == nil {
			firstByte <- received[0]
		}
	}()

	return listener.Addr().String(), firstByte, func() { _ = listener.Close() }
}

func startHangingGreetingServer() (string, func()) {
	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		_, _ = conn.Write([]byte("220 mock.local ESMTP\r\n"))

		buf := make([]byte, 64)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}()

	return listener.Addr().String(), func() { _ = listener.Close() }
}

func matchFailure(id failures.FailureID) gomegaTypes.GomegaMatcher {
	return gomega.MatchError(fail(id, nil))
}

func modifyURL(base string, params map[string]string) string {
	u := testutils.URLMust(base)

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}

	u.RawQuery = q.Encode()

	return u.String()
}

func mockSMTPURL(address, extraQuery string) string {
	query := "auth=none&fromAddress=sender@example.com&toAddresses=rec1@example.com&timeout=5s"
	if extraQuery != "" {
		query += "&" + extraQuery
	}

	return "smtp://" + address + "/?" + query
}

func skipIfTestSetup(err error) {
	if msg, test := standard.IsTestSetupFailure(asFailure(err)); test {
		ginkgo.Skip(msg)
	}
}

func runSMTPSession(svc *Service, client *smtp.Client, config *Config) error {
	return runSMTPSessionMessage(svc, client, config, "Test message")
}

func runSMTPSessionMessage(svc *Service, client *smtp.Client, config *Config, message string) error {
	return (&session{
		client: client,
		config: config,
		svc:    svc,
	}).run(message)
}

func dataPayload(sentences []string) string {
	var builder strings.Builder

	inData := false

	for _, line := range sentences {
		if line == "DATA" {
			inData = true

			continue
		}

		if !inData {
			continue
		}

		if line == "." {
			break
		}

		builder.WriteString(line)
		builder.WriteByte('\n')
	}

	return builder.String()
}

func headerLines(sentences []string, name string) []string {
	prefix := name + ": "

	var found []string

	for _, line := range sentences {
		if strings.HasPrefix(line, prefix) {
			found = append(found, line)
		}
	}

	return found
}

func asFailure(err error) failures.Failure {
	if err == nil {
		return nil
	}

	var f failures.Failure
	if errors.As(err, &f) {
		return f
	}

	return fail(FailUnknown, err)
}

func testIntegration(
	testURL string,
	responses []string,
	htmlTemplate string,
	plainTemplate string,
	expectRec ...string,
) failures.Failure {
	return runTestIntegration(testURL, responses, htmlTemplate, plainTemplate, true, expectRec...)
}

func testIntegrationWithoutTLS(
	testURL string,
	responses []string,
	expectRec ...string,
) failures.Failure {
	return runTestIntegration(testURL, responses, "", "", false, expectRec...)
}

func runTestIntegration(
	testURL string,
	responses []string,
	htmlTemplate string,
	plainTemplate string,
	fakeTLS bool,
	expectRec ...string,
) failures.Failure {
	serviceURL, err := url.Parse(testURL)
	if err != nil {
		return standard.Failure(standard.FailParseURL, err)
	}

	if err = service.Initialize(serviceURL, logger); err != nil {
		return standard.Failure(standard.FailServiceInit, err)
	}

	if htmlTemplate != "" {
		if err := service.SetTemplateString("HTML", htmlTemplate); err != nil {
			return failures.Wrap("error setting HTML template", standard.FailTestSetup, err)
		}
	}

	if plainTemplate != "" {
		if err := service.SetTemplateString("plain", plainTemplate); err != nil {
			return failures.Wrap("error setting plain template", standard.FailTestSetup, err)
		}
	}

	textCon, tcfaker := testutils.CreateTextConFaker(responses, "\r\n")

	client := &smtp.Client{Text: textCon}
	if fakeTLS {
		fakeTLSEnabled(client, serviceURL.Hostname())
	}

	config := service.Config.Clone()
	ferr := asFailure(runSMTPSession(service, client, &config))

	received := tcfaker.GetClientSentences()
	for _, expected := range expectRec {
		gomega.Expect(received).To(gomega.ContainElement(expected))
	}

	logger.Printf("\n%s", tcfaker.GetConversation(false))

	if ferr != nil {
		return ferr
	}

	return nil
}

func testSendRecipient(responses []string) failures.Failure {
	serviceURL, err := url.Parse(baseNoAuthURL)
	if err != nil {
		return standard.Failure(standard.FailParseURL, err)
	}

	err = service.Initialize(serviceURL, logger)
	if err != nil {
		return failures.Wrap("error parsing URL", standard.FailTestSetup, err)
	}

	if err := service.SetTemplateString("plain", "{{.message}}"); err != nil {
		return failures.Wrap("error setting plain template", standard.FailTestSetup, err)
	}

	textCon, tcfaker := testutils.CreateTextConFaker(responses, "\r\n")
	client := &smtp.Client{Text: textCon}
	fakeTLSEnabled(client, serviceURL.Hostname())

	config := service.Config.Clone()
	message := "message body"
	ferr := asFailure((&session{
		client: client,
		config: &config,
		svc:    service,
	}).sendMessage(config.ToAddresses[0], message, ""))

	logger.Printf("\n%s", tcfaker.GetConversation(false))

	if ferr != nil {
		return ferr
	}

	return nil
}
