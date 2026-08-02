package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

type ntfyMessage struct {
	ID          string   `json:"id"`
	Event       string   `json:"event"`
	Topic       string   `json:"topic"`
	Message     string   `json:"message"`
	Title       string   `json:"title"`
	Priority    int      `json:"priority"`
	Tags        []string `json:"tags"`
	ContentType string   `json:"content_type"`
}

const defaultMessageTimeout = 5 * time.Second

var _ = ginkgo.Describe("ntfy E2E Basic Tests", func() {
	ginkgo.When("running e2e tests against a real ntfy server", func() {
		ginkgo.BeforeEach(func() {
			if !isNtfyServerAvailable() {
				ginkgo.Skip("ntfy server not available, skipping e2e tests")
			}
		})

		ginkgo.It("should send basic text content", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("ntfy server not configured, skipping basic text test")
			}

			service := initializeService(serviceURLStr)

			message := "E2E Test: Basic text content notification"

			gomega.Expect(service.Send(message, nil)).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(service.Config.Topic, message)
		})

		ginkgo.It("should send an empty message", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("ntfy server not configured, skipping empty message test")
			}

			service := initializeService(serviceURLStr)

			gomega.Expect(service.Send("", nil)).NotTo(gomega.HaveOccurred())

			ginkgo.GinkgoWriter.Write([]byte("Empty message sent successfully\n"))
		})

		ginkgo.It("should send a message with a title via params", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("ntfy server not configured, skipping title test")
			}

			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Message with title"
			expectedTitle := "Test Title"

			gomega.Expect(service.Send(message, &types.Params{
				"title": expectedTitle,
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceivedWithTitle(topic, message, expectedTitle)
		})

		ginkgo.It("should send a message with priority via params", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("ntfy server not configured, skipping priority test")
			}

			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: High priority message"

			gomega.Expect(service.Send(message, &types.Params{
				"priority": "5",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceivedWithPriority(topic, message, 5)
		})

		ginkgo.It("should send a message with tags", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("ntfy server not configured, skipping tags test")
			}

			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Tagged message"

			gomega.Expect(service.Send(message, &types.Params{
				"tags": "warning,skull",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceivedWithTags(topic, message, []string{"warning", "skull"})
		})

		ginkgo.It("should send a message with markdown", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("ntfy server not configured, skipping markdown test")
			}

			serviceURLStr += "&markdown=yes"
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: **bold** and *italic* markdown"

			gomega.Expect(service.Send(message, nil)).NotTo(gomega.HaveOccurred())

			msg, err := pollForMessage(topic, message)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "message should be received by ntfy server")
			gomega.Expect(msg).NotTo(gomega.BeNil())
			gomega.Expect(msg.Event).To(gomega.Equal("message"))
			gomega.Expect(msg.Topic).To(gomega.ContainSubstring(topic))
			gomega.Expect(msg.ContentType).To(gomega.Equal("text/markdown"), "received event should reflect Markdown Content-Type")
		})

		ginkgo.It("should send a message with special characters and unicode", func() {
			serviceURLStr := buildServiceURL()
			if serviceURLStr == "" {
				ginkgo.Skip("ntfy server not configured, skipping unicode test")
			}

			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Special chars <>@#$% and unicode 世界 🌍"

			gomega.Expect(service.Send(message, nil)).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})
	})

	ginkgo.When("no server is configured", func() {
		ginkgo.It("should return the correct service ID", func() {
			service := &ntfy.Service{}
			gomega.Expect(service.GetID()).To(gomega.Equal("ntfy"))
		})
	})
})

func pollForMessage(topic, expectedMessage string) (*ntfyMessage, error) {
	baseURL := getNtfyBaseURL()
	apiURL := fmt.Sprintf("%s/%s/json?poll=1&message=%s", baseURL, topic, url.QueryEscape(expectedMessage))

	ctx, cancel := context.WithTimeout(context.Background(), defaultMessageTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	client := &http.Client{Timeout: defaultMessageTimeout}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	lines := strings.SplitSeq(strings.TrimSpace(string(body)), "\n")
	for line := range lines {
		if line == "" {
			continue
		}

		var msg ntfyMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msg.Message == expectedMessage {
			return &msg, nil
		}
	}

	return nil, errors.New("message not found in response")
}

func verifyMessageReceived(topic, message string) {
	msg, err := pollForMessage(topic, message)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "message should be received by ntfy server")
	gomega.Expect(msg).NotTo(gomega.BeNil())
	gomega.Expect(msg.Event).To(gomega.Equal("message"))
	gomega.Expect(msg.Topic).To(gomega.ContainSubstring(topic))
}

func verifyMessageReceivedWithTitle(topic, message, expectedTitle string) {
	msg, err := pollForMessage(topic, message)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "message should be received by ntfy server")
	gomega.Expect(msg).NotTo(gomega.BeNil())
	gomega.Expect(msg.Event).To(gomega.Equal("message"))
	gomega.Expect(msg.Title).To(gomega.Equal(expectedTitle))
}

func verifyMessageReceivedWithPriority(topic, message string, expectedPriority int) {
	msg, err := pollForMessage(topic, message)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "message should be received by ntfy server")
	gomega.Expect(msg).NotTo(gomega.BeNil())
	gomega.Expect(msg.Event).To(gomega.Equal("message"))
	gomega.Expect(msg.Priority).To(gomega.Equal(expectedPriority))
}

func verifyMessageReceivedWithTags(topic, message string, expectedTags []string) {
	msg, err := pollForMessage(topic, message)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "message should be received by ntfy server")
	gomega.Expect(msg).NotTo(gomega.BeNil())
	gomega.Expect(msg.Event).To(gomega.Equal("message"))
	gomega.Expect(msg.Tags).To(gomega.ConsistOf(expectedTags))
}
