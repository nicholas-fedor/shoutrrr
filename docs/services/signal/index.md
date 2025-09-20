# Signal

## URL Format

!!! info ""
    signal://[__`user`__[:__`password`__]@]__`host`__[:__`port`__]/__`source_phone`__/__`recipient1`__[,__`recipient2`__,...]

    signals://[__`user`__[:__`password`__]@]__`host`__[:__`port`__]/__`source_phone`__/__`recipient1`__[,__`recipient2`__,...]

## Setting up Signal API Server

Signal notifications require a Signal API server that can send messages on behalf of a registered Signal account. These implementations are built on top of __[signal-cli](https://github.com/AsamK/signal-cli)__, the unofficial command-line interface for Signal (3.8k+ stars).

Popular open-source implementations include:

- __[signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api)__: Dockerized REST API wrapper for signal-cli (2.1k+ stars)
- __[secured-signal-api](https://github.com/codeshelldev/secured-signal-api)__: Security proxy for signal-cli-rest-api with authentication and access control

Common setup involves:

1. __Phone Number__: A dedicated phone number registered with Signal
2. __API Server__: A server running signal-cli with REST API capabilities
3. __Account Linking__: Linking the server as a secondary device to your Signal account
4. __Optional Security Layer__: Authentication and endpoint restrictions via a proxy

The server must be able to receive SMS verification codes during initial setup and maintain a persistent connection to Signal's servers.

!!! tip "Setup Resources"
    See the [signal-cli-rest-api documentation](https://github.com/bbernhard/signal-cli-rest-api) and [secured-signal-api documentation](https://github.com/codeshelldev/secured-signal-api) for detailed setup instructions.

## URL Parameters

### Host and Port

- `host`: The hostname or IP address of your Signal API server (default: localhost)
- `port`: The port number (default: 8080)

### Authentication

The Signal service supports multiple authentication methods:

- `user`: Username for HTTP Basic Authentication (optional)
- `password`: Password for HTTP Basic Authentication (optional)
- `token` or `apikey`: API token for Bearer authentication (optional)

!!! tip "Authentication Priority"
    If both token and user/password are provided, the API token takes precedence and uses Bearer authentication. This is useful for [secured-signal-api](https://github.com/codeshelldev/secured-signal-api) which prefers Bearer tokens.

### Source Phone Number

The `source_phone` is your Signal phone number with country code (e.g., +1234567890) that is registered with the API server.

### Recipients

Recipients can be:

- __Phone numbers__: With country code (e.g., +0987654321)
- __Group IDs__: In the format `group.groupId`

### TLS Configuration

- Use `signal://` for HTTPS (default, recommended)
- Use `signals://` for HTTP (insecure, for local testing only)

## Examples

### Send to a single phone number

```
signal://localhost:8080/+1234567890/+0987654321
```

### Send to multiple recipients

```
signal://localhost:8080/+1234567890/+0987654321/+1123456789/group.testgroup
```

### Send to a group

```
signal://localhost:8080/+1234567890/group.abcdefghijklmnop=
```

### With authentication

```
signal://user:password@localhost:8080/+1234567890/+0987654321
```

### With API token (Bearer auth)

```
signal://localhost:8080/+1234567890/+0987654321?token=YOUR_API_TOKEN
```

### Using HTTP instead of HTTPS

```
signals://localhost:8080/+1234567890/+0987654321
```

## Attachments

The Signal service supports sending base64-encoded attachments. Use the `attachments` parameter with comma-separated base64 data:

```bash
# Send with attachments via CLI
shoutrrr send "signal://localhost:8080/+1234567890/+0987654321" \
  "Message with attachment" \
  --attachments "base64data1,base64data2"
```

!!! note "Attachment Format"
    Attachments must be provided as base64-encoded data. The API server handles the MIME type detection and file handling.

## Optional Parameters

You can specify additional parameters in the URL query string:

- `disabletls=yes`: Force HTTP instead of HTTPS (same as using `signals://`)

## Implementation Notes

The Signal service sends messages using HTTP POST requests to the API server's send endpoint with JSON payloads containing the message, source number, and recipient list. The server handles the actual Signal protocol communication.
