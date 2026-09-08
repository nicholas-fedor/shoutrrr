package e2e_test

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Home Assistant E2E", func() {
	ginkgo.BeforeEach(func() {
		if !isHomeAssistantAvailable() {
			ginkgo.Skip("Home Assistant is not available, skipping e2e tests")
		}
	})

	ginkgo.It("should create a persistent notification", func() {
		rawURL := withNid(serviceURL(), "shoutrrr_e2e")
		service := initializeService(rawURL)

		message := "E2E Test: persistent notification"
		gomega.Expect(service.Send(message, nil)).NotTo(gomega.HaveOccurred())

		state := fetchPersistentNotification(accessToken(rawURL), "shoutrrr_e2e")
		gomega.Expect(state.Message).To(gomega.Equal(message))
	})

	ginkgo.It("should include a title", func() {
		rawURL := withNid(serviceURL(), "shoutrrr_e2e_title")
		service := initializeService(rawURL)

		params := types.Params{"title": "E2E Title"}
		gomega.Expect(service.Send("E2E Test: titled notification", &params)).
			NotTo(gomega.HaveOccurred())

		state := fetchPersistentNotification(accessToken(rawURL), "shoutrrr_e2e_title")
		gomega.Expect(state.Title).To(gomega.Equal("E2E Title"))
		gomega.Expect(state.Message).To(gomega.Equal("E2E Test: titled notification"))
	})

	ginkgo.It("should replace a persistent notification when nid is reused", func() {
		rawURL := withNid(serviceURL(), "shoutrrr_e2e_replace")
		service := initializeService(rawURL)

		gomega.Expect(service.Send("first message", nil)).NotTo(gomega.HaveOccurred())
		gomega.Expect(service.Send("second message", nil)).NotTo(gomega.HaveOccurred())

		state := fetchPersistentNotification(accessToken(rawURL), "shoutrrr_e2e_replace")
		gomega.Expect(state.Message).To(gomega.Equal("second message"))
	})

	ginkgo.It("should send over HTTP when disabletls is set", func() {
		gomega.Expect(serviceURL()).To(gomega.ContainSubstring("disabletls"))

		rawURL := withNid(serviceURL(), "shoutrrr_e2e_http")
		service := initializeService(rawURL)

		gomega.Expect(service.Send("E2E Test: HTTP", nil)).NotTo(gomega.HaveOccurred())

		state := fetchPersistentNotification(accessToken(rawURL), "shoutrrr_e2e_http")
		gomega.Expect(state.Message).To(gomega.Equal("E2E Test: HTTP"))
	})

	ginkgo.It("should reject an unauthorized token", func() {
		rawURL := "homeassistant://invalid-token@localhost:8123/?disabletls=yes"
		service := initializeService(rawURL)

		err := service.Send("should fail", nil)
		gomega.Expect(err).To(gomega.HaveOccurred())
	})
})
