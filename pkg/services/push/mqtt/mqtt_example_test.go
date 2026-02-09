package mqtt_test

import (
	"fmt"
	"io"
	"log"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ExampleService_basic demonstrates basic MQTT usage without authentication.
func ExampleService_basic() {
	// Create a new MQTT service
	service := &mqtt.Service{}

	// Parse the MQTT URL for a local broker
	// Format: mqtt://host:port/topic
	serviceURL, err := url.Parse("mqtt://localhost:1883/notifications")
	if err != nil {
		log.Printf("Failed to parse URL: %v", err)

		return
	}

	// Initialize the service with a discard logger for this example
	err = service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		log.Printf("Failed to initialize service: %v", err)

		return
	}

	// The service is now configured and ready to send messages
	// Note: Actual sending requires a running MQTT broker
	fmt.Println("MQTT service initialized successfully")
	// Output: MQTT service initialized successfully
}

// ExampleService_withAuth demonstrates MQTT with username/password authentication.
func ExampleService_withAuth() {
	// Create a new MQTT service
	service := &mqtt.Service{}

	// Parse the MQTT URL with authentication credentials
	// Format: mqtt://username:password@host:port/topic
	serviceURL, err := url.Parse("mqtt://myuser:mypassword@broker.example.com:1883/sensors/data")
	if err != nil {
		log.Printf("Failed to parse URL: %v", err)

		return
	}

	// Initialize the service
	err = service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		log.Printf("Failed to initialize service: %v", err)

		return
	}

	// The service is configured with authentication
	fmt.Println("MQTT service with auth initialized successfully")
	// Output: MQTT service with auth initialized successfully
}

// ExampleService_withTLS demonstrates secure MQTT connection using TLS.
func ExampleService_withTLS() {
	// Create a new MQTT service
	service := &mqtt.Service{}

	// Parse the MQTTS URL for secure connection
	// Format: mqtts://host:port/topic
	// The mqtts scheme automatically enables TLS
	serviceURL, err := url.Parse("mqtts://broker.example.com:8883/secure-topic")
	if err != nil {
		log.Printf("Failed to parse URL: %v", err)

		return
	}

	// Initialize the service
	err = service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		log.Printf("Failed to initialize service: %v", err)

		return
	}

	// The service is configured for secure TLS connection
	fmt.Println("MQTTS service with TLS initialized successfully")
	// Output: MQTTS service with TLS initialized successfully
}

// ExampleService_withQoS demonstrates MQTT with Quality of Service settings.
func ExampleService_withQoS() {
	// Create a new MQTT service
	service := &mqtt.Service{}

	// Parse the MQTT URL with QoS and retained message options
	// qos=1 ensures at-least-once delivery
	// retained=yes keeps the message on the broker for new subscribers
	serviceURL, err := url.Parse(
		"mqtt://broker.example.com/alerts?qos=1&retained=yes&clientid=myapp-client",
	)
	if err != nil {
		log.Printf("Failed to parse URL: %v", err)

		return
	}

	// Initialize the service
	err = service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		log.Printf("Failed to initialize service: %v", err)

		return
	}

	// The service is configured with QoS 1 and retained messages
	fmt.Println("MQTT service with QoS settings initialized successfully")
	// Output: MQTT service with QoS settings initialized successfully
}

// ExampleService_withCustomConfig demonstrates MQTT with custom configuration.
func ExampleService_withCustomConfig() {
	// Create a new MQTT service
	service := &mqtt.Service{}

	// Parse the MQTTS URL with various configuration options
	// - cleansession=no: persist session across connections
	// - disabletlsverification=yes: skip TLS cert verification
	//
	// WARNING: disabletlsverification=yes bypasses certificate validation and
	// should ONLY be used for testing with self-signed certificates.
	// NEVER use this option in production environments as it exposes
	// connections to man-in-the-middle attacks.
	serviceURL, err := url.Parse(
		"mqtts://broker.example.com/events?cleansession=no&disabletlsverification=yes",
	)
	if err != nil {
		log.Printf("Failed to parse URL: %v", err)

		return
	}

	// Initialize the service
	err = service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		log.Printf("Failed to initialize service: %v", err)

		return
	}

	// The service is configured with custom settings including TLS verification bypass
	// Note: disabletlsverification is only applicable to mqtts:// (TLS) connections
	fmt.Println("MQTTS service with custom config initialized successfully")
	// Output: MQTTS service with custom config initialized successfully
}

// ExampleService_withParams demonstrates sending a message with parameters.
func ExampleService_withParams() {
	// Create a new MQTT service
	service := &mqtt.Service{}

	// Parse the MQTT URL
	serviceURL, err := url.Parse("mqtt://broker.example.com/notifications")
	if err != nil {
		log.Printf("Failed to parse URL: %v", err)

		return
	}

	// Initialize the service
	err = service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		log.Printf("Failed to initialize service: %v", err)

		return
	}

	// Create optional parameters for the message
	params := types.Params{
		"title": "Alert Title",
	}

	// The service is ready to send messages with parameters.
	// Note: Actual sending requires a running MQTT broker.
	// Example usage (uncomment to send with params, requires a running broker):
	//
	//	err = service.Send("Your message payload", &params)
	//	if err != nil {
	//		log.Printf("Failed to send message: %v", err)
	//
	//		return
	//	}
	_ = params // Placeholder to prevent unused variable error in this example

	fmt.Println("Service ready for parameterized messages")
	// Output: Service ready for parameterized messages
}

// ExampleService_configURL demonstrates accessing the service configuration
// after initialization.
func ExampleService_configURL() {
	// Create a new MQTT service
	service := &mqtt.Service{}

	// Parse and initialize with a complex URL
	serviceURL, err := url.Parse(
		"mqtt://user:pass@broker.example.com:1883/test/topic?qos=2&retained=yes",
	)
	if err != nil {
		log.Printf("Failed to parse URL: %v", err)

		return
	}

	err = service.Initialize(serviceURL, log.New(io.Discard, "", 0))
	if err != nil {
		log.Printf("Failed to initialize service: %v", err)

		return
	}

	// Access the service configuration fields after initialization
	// The Config struct contains all parsed settings from the URL
	fmt.Printf("Topic: %s\n", service.Config.Topic)
	fmt.Printf("QoS: %d\n", service.Config.QoS)
	fmt.Printf("Retained: %v\n", service.Config.Retained)
	// Output:
	// Topic: test/topic
	// QoS: 2
	// Retained: true
}
