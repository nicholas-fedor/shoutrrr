# MQTT

MQTT is a lightweight messaging protocol for small sensors and mobile devices, ideal for IoT and low-bandwidth environments. Upstream docs: <https://mqtt.org/>

## Features

- __QoS Levels__: Support for Quality of Service levels 0 (at most once), 1 (at least once), and 2 (exactly once)
- __Retained Messages__: Messages can be retained by the broker for new subscribers
- __TLS/SSL Support__: Secure connections via the `mqtts://` scheme
- __Authentication__: Username/password authentication
- __Clean Session__: Control whether the broker maintains session state
- __Managed Connection__: The first send connects to the broker, later sends reuse the connection, and an idle connection is closed after 60 seconds

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

## Connection Lifecycle

The service connects to the broker lazily and keeps the connection open between sends.

### How It Works

1. `Initialize()` parses the URL and stores the configuration. It does not connect to the broker.
2. The first `Send()` opens the connection, and later sends reuse it.
3. A connection that has not been used for 60 seconds is closed. The next `Send()` opens a new one, so short-lived programs do not leave connections behind.
4. `Close()` disconnects from the broker. A later `Send()` reconnects.

A connection manager set with `SetConnectionManager()` belongs to the caller. The service never closes it for being idle, and `Close()` disconnects it.

### Error Behavior

If opening the connection fails, the error is returned to the caller and the next `Send()` tries again, so temporary network issues or broker unavailability do not require a new call to `Initialize()`. If the connection ends, for example because it was disconnected, the next `Send()` opens a new one.

### Send Params

Params apply to a single send and do not change the service configuration. The `qos` and `retained` params affect every send that uses them. The connection params (`clientid`, `cleansession`, `disabletls`, and `disabletlsverification`) apply when that send opens the connection, and are ignored while an existing connection is reused.

```go title="Example Send Params"
// Publish this message as retained with QoS 1, without changing later sends
params := types.Params{
    "qos":      "1",
    "retained": "yes",
}
service.Send(message, &params)
```

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
