package e2e_test

import (
	"bufio"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

// Test_Zulip_E2E runs the Zulip E2E test suite using Ginkgo.
// These tests connect to a real Zulip server and verify actual behavior.
//
// Environment variables:
//   - SHOUTRRR_ZULIP_HOST: Zulip server hostname (default: localhost)
//   - SHOUTRRR_ZULIP_BOT_MAIL: Bot email address
//   - SHOUTRRR_ZULIP_BOT_KEY: Bot API key
//   - SHOUTRRR_ZULIP_STREAM: Target stream name (optional)
//   - SHOUTRRR_ZULIP_TOPIC: Message topic (optional)
func Test_Zulip_E2E(t *testing.T) {
	t.Parallel()

	loadEnvFile(".env")

	gomega.RegisterFailHandler(ginkgo.Fail)

	ginkgo.BeforeEach(func() {
		time.Sleep(250 * time.Millisecond)
	})

	ginkgo.RunSpecs(t, "Zulip E2E Tests")
}

// buildServiceURL constructs the Zulip service URL from environment variables.
func buildServiceURL() string {
	host := os.Getenv("SHOUTRRR_ZULIP_HOST")
	if host == "" {
		host = "localhost"
	}

	botMail := os.Getenv("SHOUTRRR_ZULIP_BOT_MAIL")
	botKey := os.Getenv("SHOUTRRR_ZULIP_BOT_KEY")

	if botMail == "" || botKey == "" {
		return ""
	}

	serviceURL := &url.URL{
		Scheme: "zulip",
		User:   url.UserPassword(botMail, botKey),
		Host:   host,
	}

	q := url.Values{}
	if stream := os.Getenv("SHOUTRRR_ZULIP_STREAM"); stream != "" {
		q.Set("stream", stream)
	}

	if topic := os.Getenv("SHOUTRRR_ZULIP_TOPIC"); topic != "" {
		q.Set("topic", topic)
	}

	if len(q) > 0 {
		serviceURL.RawQuery = q.Encode()
	}

	return serviceURL.String()
}

// loadEnvFile loads environment variables from a .env file.
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
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			value = strings.Trim(value, `"'`)
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
	}

	_ = scanner.Err()
}
