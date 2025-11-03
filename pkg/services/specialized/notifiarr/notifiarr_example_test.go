package notifiarr

import (
	"fmt"
	"io"
	"log"
	"net/url"

	"github.com/jarcoal/httpmock"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Example demonstrates basic notification sending to Notifiarr.
func Example() {
	// Activate HTTP mocking for the example
	httpmock.Activate()

	defer httpmock.DeactivateAndReset()

	// Mock a successful Notifiarr API response
	httpmock.RegisterResponder(
		"POST",
		"https://notifiarr.com/api/v1/notification/passthrough/test-api-key",
		httpmock.NewStringResponder(200, `{"success": true}`),
	)

	// Create and initialize the service
	service := &Service{}
	serviceURL, _ := url.Parse("notifiarr://test-api-key")

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification
	err = service.Send("Hello from Notifiarr!", nil)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Output: Notification sent successfully
	fmt.Println("Notification sent successfully")
}

// Example_eventID demonstrates sending a notification with an event ID for message updates.
func Example_eventID() {
	// Activate HTTP mocking for the example
	httpmock.Activate()

	defer httpmock.DeactivateAndReset()

	// Mock a successful Notifiarr API response
	httpmock.RegisterResponder(
		"POST",
		"https://notifiarr.com/api/v1/notification/passthrough/test-api-key",
		httpmock.NewStringResponder(200, `{"success": true}`),
	)

	// Create and initialize the service
	service := &Service{}
	serviceURL, _ := url.Parse("notifiarr://test-api-key")

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification with event ID and title
	params := types.Params{
		"id":    "alert-123",
		"title": "System Alert",
	}

	err = service.Send("Server CPU usage is high", &params)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Output: Notification with event ID sent successfully
	fmt.Println("Notification with event ID sent successfully")
}

// Example_discordMentions demonstrates Discord mention parsing from message content.
func Example_discordMentions() {
	// Activate HTTP mocking for the example
	httpmock.Activate()

	defer httpmock.DeactivateAndReset()

	// Mock a successful Notifiarr API response
	httpmock.RegisterResponder(
		"POST",
		"https://notifiarr.com/api/v1/notification/passthrough/test-api-key",
		httpmock.NewStringResponder(200, `{"success": true}`),
	)

	// Create and initialize the service
	service := &Service{}
	serviceURL, _ := url.Parse("notifiarr://test-api-key")

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification with Discord mentions
	message := "Alert for <@123456789> and role <@&987654321>"

	err = service.Send(message, nil)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Output: Notification with mentions sent successfully
	fmt.Println("Notification with mentions sent successfully")
}

// Example_imageThumbnail demonstrates sending a notification with a thumbnail image.
func Example_imageThumbnail() {
	// Activate HTTP mocking for the example
	httpmock.Activate()

	defer httpmock.DeactivateAndReset()

	// Mock a successful Notifiarr API response
	httpmock.RegisterResponder(
		"POST",
		"https://notifiarr.com/api/v1/notification/passthrough/test-api-key",
		httpmock.NewStringResponder(200, `{"success": true}`),
	)

	// Create and initialize the service with thumbnail
	service := &Service{}
	serviceURL, _ := url.Parse("notifiarr://test-api-key?thumbnail=https://example.com/alert.png")

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification
	err = service.Send("System alert with thumbnail", nil)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Output: Notification with thumbnail sent successfully
	fmt.Println("Notification with thumbnail sent successfully")
}

// Example_colorCustomization demonstrates sending a notification with custom color.
func Example_colorCustomization() {
	// Activate HTTP mocking for the example
	httpmock.Activate()

	defer httpmock.DeactivateAndReset()

	// Mock a successful Notifiarr API response
	httpmock.RegisterResponder(
		"POST",
		"https://notifiarr.com/api/v1/notification/passthrough/test-api-key",
		httpmock.NewStringResponder(200, `{"success": true}`),
	)

	// Create and initialize the service with color
	service := &Service{}
	serviceURL, _ := url.Parse("notifiarr://test-api-key?color=16711680") // Red color

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification
	err = service.Send("Red alert notification", nil)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Output: Colored notification sent successfully
	fmt.Println("Colored notification sent successfully")
}

// Example_urlParameterConfiguration demonstrates configuring notifications via URL parameters.
func Example_urlParameterConfiguration() {
	// Activate HTTP mocking for the example
	httpmock.Activate()

	defer httpmock.DeactivateAndReset()

	// Mock a successful Notifiarr API response
	httpmock.RegisterResponder(
		"POST",
		"https://notifiarr.com/api/v1/notification/passthrough/test-api-key",
		httpmock.NewStringResponder(200, `{"success": true}`),
	)

	// Create and initialize the service with multiple parameters
	service := &Service{}
	serviceURL, _ := url.Parse(
		"notifiarr://test-api-key?channel=123456789012345678&thumbnail=https://example.com/icon.png&image=https://example.com/image.png&color=65280",
	)

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification
	err = service.Send("Fully configured notification with image", nil)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Output: Fully configured notification with image sent successfully
	fmt.Println("Fully configured notification with image sent successfully")
}

// Example_imageAndThumbnail demonstrates sending a notification with both image and thumbnail URLs.
func Example_imageAndThumbnail() {
	// Activate HTTP mocking for the example
	httpmock.Activate()

	defer httpmock.DeactivateAndReset()

	// Mock a successful Notifiarr API response
	httpmock.RegisterResponder(
		"POST",
		"https://notifiarr.com/api/v1/notification/passthrough/test-api-key",
		httpmock.NewStringResponder(200, `{"success": true}`),
	)

	// Create and initialize the service with both image and thumbnail
	service := &Service{}
	serviceURL, _ := url.Parse(
		"notifiarr://test-api-key?thumbnail=https://example.com/thumbnail.png&image=https://example.com/full-image.png",
	)

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification
	err = service.Send("Notification with both thumbnail and full image", nil)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Output: Notification with image and thumbnail sent successfully
	fmt.Println("Notification with image and thumbnail sent successfully")
}

// Example_advancedTextFields demonstrates sending a notification with advanced text fields including icon, content, and footer.
func Example_advancedTextFields() {
	// Activate HTTP mocking for the example
	httpmock.Activate()

	defer httpmock.DeactivateAndReset()

	// Mock a successful Notifiarr API response
	httpmock.RegisterResponder(
		"POST",
		"https://notifiarr.com/api/v1/notification/passthrough/test-api-key",
		httpmock.NewStringResponder(200, `{"success": true}`),
	)

	// Create and initialize the service
	service := &Service{}
	serviceURL, _ := url.Parse("notifiarr://test-api-key")

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification with advanced text fields
	params := types.Params{
		"title":   "System Status Update",
		"icon":    "https://example.com/icon.png",
		"content": "Detailed content about the system status",
		"footer":  "Generated by Monitoring System v2.1",
	}

	err = service.Send("System is running normally", &params)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Output: Notification with advanced text fields sent successfully
	fmt.Println("Notification with advanced text fields sent successfully")
}
