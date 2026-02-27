// Package mqtt provides a notification service for sending messages to MQTT brokers.
//
// MQTT (Message Queuing Telemetry Transport) is a lightweight messaging protocol
// ideal for small sensors and mobile devices, optimized for high-latency or
// unreliable networks. This service supports both standard (mqtt://) and secure
// (mqtts://) connections to any MQTT-compatible broker.
//
// # URL Format
//
// The service URL follows the format:
//
//	mqtt://[username[:password]@]host[:port]/topic[?options]
//	mqtts://[username[:password]@]host[:port]/topic[?options]
//
// Where:
//   - username: optional username for authentication
//   - password: optional password for authentication
//   - host: MQTT broker hostname (default: localhost)
//   - port: optional port number (default: 1883 for mqtt://, 8883 for mqtts://)
//   - topic: target topic name (required)
//   - options: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - clientid: MQTT client identifier (default: shoutrrr)
//   - qos: Quality of Service level (0=at most once, 1=at least once, 2=exactly once)
//   - retained: retain message on broker (default: no)
//   - cleansession: start with a clean session (default: yes)
//   - disabletls: disable TLS encryption (default: no)
//   - disabletlsverification: disable TLS certificate verification (default: no)
//
// # Quality of Service (QoS) Levels
//
// MQTT supports three QoS levels:
//   - 0 (At most once): Message delivered at most once, no acknowledgment
//   - 1 (At least once): Message delivered at least once, requires acknowledgment
//   - 2 (Exactly once): Message delivered exactly once, requires four-part handshake
//
// # Usage Examples
//
// ## Basic notification
//
//	url := "mqtt://broker.example.com/notifications"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Notification with authentication
//
//	url := "mqtt://user:password@broker.example.com/sensors/temperature"
//	err := shoutrrr.Send(url, "Temperature: 25°C")
//
// ## Secure connection with QoS
//
//	url := "mqtts://broker.example.com/alerts?qos=1&retained=yes"
//	err := shoutrrr.Send(url, "System alert: High CPU usage")
//
// ## Custom client ID and clean session
//
//	url := "mqtt://broker.example.com/events?clientid=myapp&cleansession=no"
//	err := shoutrrr.Send(url, "Event triggered")
//
// # Common Use Cases
//
// ## IoT Device Notifications
//
// Send notifications from IoT devices to central systems:
//
//	url := "mqtt://iot-broker.local/sensors/living-room?retained=yes"
//	err := shoutrrr.Send(url, "Motion detected")
//
// ## Home Automation
//
// Integrate with home automation systems like Home Assistant:
//
//	url := "mqtt://homeassistant.local/homeassistant/sensor/temperature"
//	err := shoutrrr.Send(url, `{"state": "22.5", "unit": "°C"}`)
//
// ## Real-time Monitoring
//
// Send monitoring alerts to MQTT topics:
//
//	url := "mqtts://mqtt.example.com/monitoring/alerts?qos=1"
//	err := shoutrrr.Send(url, "Server CPU usage above 90%")
//
// # TLS Configuration
//
// For secure connections, use the mqtts:// scheme (TLS enabled by default):
//
//	url := "mqtts://broker.example.com:8883/secure-topic"
//
// To explicitly disable TLS, set disabletls=true. To explicitly enable TLS when
// using the mqtt:// scheme, set disabletls=false:
//
//	url := "mqtt://broker.example.com/topic?disabletls=false"
//
// To explicitly disable TLS when using the mqtts:// scheme:
//
//	url := "mqtts://broker.example.com:8883/secure-topic?disabletls=true"
//
// To skip TLS certificate verification (useful for testing with self-signed certs):
//
//	url := "mqtts://broker.example.com/topic?disabletlsverification=yes"
//
// # Connection Behavior
//
// The service establishes a connection to the MQTT broker on each Send call
// and disconnects after the message is published. This ensures reliable
// delivery while maintaining compatibility with various broker configurations.
//
// For high-volume messaging, consider using a dedicated MQTT client library
// that maintains persistent connections.
package mqtt
