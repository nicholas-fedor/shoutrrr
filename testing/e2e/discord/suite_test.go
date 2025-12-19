package e2e_test

import (
	"bufio"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestDiscordE2E(t *testing.T) {
	// Load .env file if it exists
	loadEnvFile(".env")

	gomega.RegisterFailHandler(ginkgo.Fail)

	// Add delay between tests to respect rate limits
	ginkgo.BeforeEach(func() {
		// Add a small delay between tests to avoid hitting Discord rate limits
		time.Sleep(100 * time.Millisecond)
	})

	ginkgo.RunSpecs(t, "Discord E2E Tests")
}

// loadEnvFile loads environment variables from a .env file.
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		// .env file doesn't exist, skip loading
		return
	}
	defer file.Close()

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
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			os.Setenv(key, value)
		}
	}
}
