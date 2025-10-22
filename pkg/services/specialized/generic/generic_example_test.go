package generic

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/jarcoal/httpmock"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Example demonstrates basic webhook sending to a generic endpoint.
func Example() {
	// Activate HTTP mocking for the example
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Mock a successful webhook response
	httpmock.RegisterResponder("POST", "https://api.example.com/webhook",
		httpmock.NewStringResponder(200, "OK"))

	// Create and initialize the service
	service := &Service{}
	serviceURL, _ := url.Parse("generic://api.example.com/webhook")

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification
	err = service.Send("Hello, webhook!", nil)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	fmt.Println("Notification sent successfully")
	// Output: Notification sent successfully
}

// ExampleJSONTemplate demonstrates sending a JSON-formatted payload.
func ExampleJSONTemplate() {
	// Activate HTTP mocking for the example
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Mock a successful webhook response that accepts any JSON payload
	httpmock.RegisterResponder("POST", "https://api.example.com/webhook",
		httpmock.NewStringResponder(200, "OK"))

	// Create and initialize the service with JSON template
	service := &Service{}
	serviceURL, _ := url.Parse("generic://api.example.com/webhook?template=json")

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification with title
	params := types.Params{"title": "Alert"}

	err = service.Send("System is down", &params)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	fmt.Println("JSON notification sent successfully")
	// Output: JSON notification sent successfully
}

// Example_customHeaders demonstrates sending with custom HTTP headers.
func Example_customHeaders() {
	// Activate HTTP mocking for the example
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Mock a successful webhook response that checks for custom header
	httpmock.RegisterResponder("POST", "https://api.example.com/webhook",
		func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("Authorization") == "Bearer token123" &&
				req.Header.Get("X-Custom") == "value" {
				return httpmock.NewStringResponse(200, "OK"), nil
			}

			return httpmock.NewStringResponse(401, "Unauthorized"), nil
		})

	// Create and initialize the service with custom headers
	service := &Service{}
	serviceURL, _ := url.Parse(
		"generic://api.example.com/webhook?@Authorization=Bearer%20token123&@X-Custom=value",
	)

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Send a notification
	err = service.Send("Authenticated message", nil)
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	fmt.Println("Notification with custom headers sent successfully")
	// Output: Notification with custom headers sent successfully
}

// Example_configurationParsing demonstrates parsing various configuration options from URL.
func Example_configurationParsing() {
	// Create a service URL with various configuration options
	serviceURL, _ := url.Parse(
		"generic://webhook.example.com/alert?template=json&disabletls=yes&method=POST&titlekey=alert_title&messagekey=alert_message&$source=shoutrrr&@Authorization=Bearer%20token",
	)

	// Initialize the service
	service := &Service{}

	err := service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		fmt.Println("Error:", err)

		return
	}

	// Display parsed configuration
	fmt.Printf("Webhook URL: %s\n", service.Config.WebhookURL().String())
	fmt.Printf("Template: %s\n", service.Config.Template)
	fmt.Printf("TLS Disabled: %t\n", service.Config.DisableTLS)
	fmt.Printf("Method: %s\n", service.Config.RequestMethod)
	fmt.Printf("Title Key: %s\n", service.Config.TitleKey)
	fmt.Printf("Message Key: %s\n", service.Config.MessageKey)
	fmt.Printf("Extra Data: %v\n", service.Config.extraData)
	fmt.Printf("Headers: %v\n", service.Config.headers)

	// Output:
	// Webhook URL: http://webhook.example.com/alert
	// Template: json
	// TLS Disabled: true
	// Method: POST
	// Title Key: alert_title
	// Message Key: alert_message
	// Extra Data: map[source:shoutrrr]
	// Headers: map[Authorization:Bearer token]
}
