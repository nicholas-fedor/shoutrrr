# MQTT

MQTT is a lightweight messaging protocol for small sensors and mobile devices, ideal for IoT and low-bandwidth environments. Upstream docs: <https://mqtt.org/>

## Features

- __QoS Levels__: Support for Quality of Service levels 0 (at most once), 1 (at least once), and 2 (exactly once)
- __Retained Messages__: Messages can be retained by the broker for new subscribers
- __TLS/SSL Support__: Secure connections via the `mqtts://` scheme
- __Authentication__: Username/password authentication
- __Clean Session__: Control whether the broker maintains session state
- __Lazy Initialization__: The MQTT client is initialized on the first send, allowing runtime configuration changes before connection

## Getting Started

MQTT is widely used in IoT, home automation, and real-time messaging applications.
The protocol is supported by many popular brokers:

- __Mosquitto__: A popular open-source MQTT broker
- __Home Assistant__: Built-in MQTT support for smart home automation
- __EMQX__: Enterprise-grade MQTT broker
- __HiveMQ__: Scalable MQTT platform

To send notifications via MQTT, you need a running MQTT broker and a topic to publish to.
Topics use a hierarchical naming scheme with forward slashes (e.g., `home/alerts`, `sensors/temperature`).

## URL Formats

The MQTT service supports two URL schemes for connection security:

- __`mqtt://`__: Standard unencrypted connection (port 1883 by default)

!!! Info ""
    mqtt://[__`username`__[:__`password`__]@]__`host`__[:__`port`__]/__`topic`__

- __`mqtts://`__: TLS-encrypted connection (port 8883 by default)

!!! Info ""
    mqtts://[__`username`__[:__`password`__]@]__`host`__[:__`port`__]/__`topic`__

--8<-- "docs/services/push/mqtt/config.md"

!!! Warning "TLS Configuration Options"

    The following options control TLS behavior:

    - __`disabletls`__: When set to `yes`, forces an **unencrypted connection** even if the `mqtts://` scheme is used. This overrides the scheme's implicit TLS requirement.

    - __`disabletlsverification`__: When set to `yes`, disables TLS certificate verification while still using encryption. This is useful for self-signed certificates.

!!! Danger "Security Warning: Silent TLS Downgrade"

    Setting __`disabletls=yes`__ with `mqtts://` will force an unencrypted connection despite the secure scheme. This is **likely unexpected behavior** and can cause silent downgrades where you believe traffic is encrypted but it is not.

    **Recommendation**: If you intentionally want an unencrypted connection, use `mqtt://` (non-TLS scheme) instead of combining `mqtts://` with `disabletls=yes`.

!!! Tip "When to Use `disabletls=yes`"

    This option is intended for specific edge cases, such as:

    - **TLS-terminating proxy**: When connecting through a proxy that handles TLS termination, where the connection from client-to-proxy uses TLS but proxy-to-broker is plain MQTT. For example, a reverse proxy like Traefik or nginx that terminates TLS and forwards to an internal MQTT broker.
    - **Testing environments**: Local development where encryption is not required.

## Lazy Initialization

The MQTT client uses lazy initialization, meaning the connection to the broker is not established until the first message is sent. This design allows runtime configuration changes to take effect before the connection is created.

### How It Works

1. When you call `Initialize()`, the service parses the URL and stores the configuration, but does not create the MQTT client
2. On the first call to `Send()`, the client is initialized with the current configuration
3. Once initialized, the client is reused for all subsequent sends

### Error Behavior

If the connection attempt during lazy initialization fails, the following behavior applies:

- The error is returned to the caller immediately
- The internal client remains uninitialized after a failed attempt
- Subsequent `Send()` calls will retry initialization, allowing for transient failure recovery

This retry behavior means that temporary network issues or broker unavailability can be resolved on the next `Send()` call without requiring a new call to `Initialize()`.

### Runtime Configuration

This lazy approach allows you to override connection settings (Host, Port, Username, Password, TLS settings) via params on the first `Send()` call:

```go title="Example Lazy Initialization Runtime Configuration"
// Placeholder MQTT URL (will be overridden on first send)
mqttURL := "mqtt://placeholder:1883/topic"

// Create a logger for the service
logger := log.New(os.Stdout, "mqtt: ", log.LstdFlags)

// The message to send
message := "Hello from shoutrrr!"

// Initialize with a placeholder URL
service.Initialize(mqttURL, logger)

// Override connection settings on first send
params := types.Params{
    "host":     "actual-broker.example.com",
    "username": "actual-user",
    "password": "actual-password",
}
service.Send(message, &params)
```

!!! Note
    Configuration changes after the first `Send()` call will only affect message-related settings (Topic, QoS, Retained), not connection settings. The client connection cannot be reconfigured after initialization.

## Examples

!!! Example "Basic Notification"
    ```uri
    mqtt://broker.example.com/notifications
    ```

!!! Example "With Authentication"
    ```uri
    mqtt://user:pass@broker.example.com:1883/home/alerts
    ```

!!! Example "Secure Connection"
    ```uri
    mqtts://user:pass@broker.example.com:8883/home/alerts
    ```

!!! Example "With QoS and Retained Message"
    ```uri
    mqtt://broker.example.com/alerts?qos=1&retained=yes
    ```

!!! Example "Home Assistant"
    ```uri
    mqtt://homeassistant.local:1883/homeassistant/notification
    ```

!!! Example "Mosquitto broker with custom client ID"
    ```uri
    mqtt://mosquitto.example.com:1883/sensors/alerts?clientid=shoutrrr-alerts&qos=2
    ```

!!! Example "Self-signed Certificate"
    ```uri
    mqtts://broker.local:8883/secure/alerts?disabletlsverification=yes
    ```

!!! Example "Full Configuration"
    ```uri
    mqtts://admin:secret@mqtt.example.com:8883/production/alerts?clientid=prod-shoutrrr&qos=1&retained=yes&cleansession=no
    ```
