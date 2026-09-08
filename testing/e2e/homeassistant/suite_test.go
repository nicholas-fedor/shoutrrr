package e2e_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/homeassistant"
)

type haNotification struct {
	Message        string `json:"message"`
	NotificationID string `json:"notification_id"`
	Title          string `json:"title"`
}

const defaultHTTPTimeout = 10 * time.Second

func Test_HomeAssistant_E2E(t *testing.T) { //nolint:paralleltest // mutates process-wide env via loadEnvFile
	loadEnvFile(".env")

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Home Assistant E2E Tests")
}

func serviceURL() string {
	return os.Getenv("SHOUTRRR_HOMEASSISTANT_URL")
}

func initializeService(urlStr string) *homeassistant.Service {
	parsed, err := url.Parse(urlStr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	service := &homeassistant.Service{}
	err = service.Initialize(parsed, testutils.TestLogger())
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	service.SetHTTPClient(e2eHTTPClient())

	return service
}

func e2eTLSConfig() *tls.Config {
	pem, err := os.ReadFile("config/ssl/fullchain.pem")
	if err != nil {
		return nil
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil
	}

	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}
}

func e2eHTTPClient() *http.Client {
	tlsConfig := e2eTLSConfig()
	if tlsConfig == nil {
		return &http.Client{Timeout: defaultHTTPTimeout}
	}

	return &http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

func withNid(rawURL, nid string) string {
	parsed, err := url.Parse(rawURL)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	query := parsed.Query()
	query.Set("nid", nid)
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func isHomeAssistantAvailable() bool {
	baseURL := serviceURL()
	if baseURL == "" {
		return false
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.User == nil {
		return false
	}

	token := parsed.User.Username()

	if e2eTLSConfig() == nil {
		return false
	}

	apiURL := "https://localhost:8123/api/"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return false
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := e2eHTTPClient()
	client.Timeout = 2 * time.Second

	resp, err := client.Do(req)
	if err != nil {
		return false
	}

	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}

func fetchPersistentNotification(token, nid string) haNotification {
	dialer := websocket.Dialer{
		HandshakeTimeout: defaultHTTPTimeout,
		TLSClientConfig:  e2eTLSConfig(),
	}
	conn, _, err := dialer.Dial("wss://localhost:8123/api/websocket", nil)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	defer func() { _ = conn.Close() }()

	gomega.Expect(conn.SetReadDeadline(time.Now().Add(defaultHTTPTimeout))).To(gomega.Succeed())
	gomega.Expect(conn.SetWriteDeadline(time.Now().Add(defaultHTTPTimeout))).To(gomega.Succeed())

	var hello map[string]any
	gomega.Expect(conn.ReadJSON(&hello)).To(gomega.Succeed())
	gomega.Expect(hello["type"]).To(gomega.Equal("auth_required"))

	gomega.Expect(conn.WriteJSON(map[string]string{
		"type":         "auth",
		"access_token": token,
	})).To(gomega.Succeed())

	var auth map[string]any
	gomega.Expect(conn.ReadJSON(&auth)).To(gomega.Succeed())
	gomega.Expect(auth["type"]).To(gomega.Equal("auth_ok"))

	gomega.Expect(conn.WriteJSON(map[string]any{
		"id":   1,
		"type": "persistent_notification/get",
	})).To(gomega.Succeed())

	var result struct {
		Success bool             `json:"success"`
		Result  []haNotification `json:"result"`
	}
	gomega.Expect(conn.ReadJSON(&result)).To(gomega.Succeed())
	gomega.Expect(result.Success).To(gomega.BeTrue())

	for _, notification := range result.Result {
		if notification.NotificationID == nid {
			return notification
		}
	}

	gomega.Expect(result.Result).To(gomega.ContainElement(gomega.HaveField("NotificationID", nid)))

	return haNotification{}
}

func accessToken(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(parsed.User).NotTo(gomega.BeNil())

	return parsed.User.Username()
}

func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}

	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])

		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
