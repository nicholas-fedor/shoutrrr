# Signal

## URL Format

!!! Info ""
    signal://[__`user`__[:__`password`__]@]__`host`__[:__`port`__]/__`source_phone`__/__`recipient1`__[,__`recipient2`__,...]

--8<-- "docs/services/chat/signal/config.md"

!!! Note "Authentication Priority"
    If both token and user/password are provided, the API token takes precedence and uses Bearer authentication.
    This is useful for [secured-signal-api](https://github.com/codeshelldev/secured-signal-api) which prefers Bearer tokens.

## Setting up Signal API Server

Signal notifications require a Signal API server that can send messages on behalf of a registered Signal account. These implementations are built on top of __[signal-cli](https://github.com/AsamK/signal-cli)__, the unofficial command-line interface for Signal.

Popular open-source implementations include:

- __[signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api)__: Dockerized REST API wrapper for signal-cli
- __[secured-signal-api](https://github.com/codeshelldev/secured-signal-api)__: Security proxy for signal-cli-rest-api with authentication and access control

Common setup involves:

1. __Phone Number__: A dedicated phone number registered with Signal
2. __API Server__: A server running signal-cli with REST API capabilities
3. __Account Linking__: Linking the server as a secondary device to your Signal account
4. __Optional Security Layer__: Authentication and endpoint restrictions via a proxy

The server must be able to receive SMS verification codes during initial setup and maintain a persistent connection to Signal's servers.

!!! Note "Setup Resources"
    See the [signal-cli-rest-api documentation](https://github.com/bbernhard/signal-cli-rest-api) and [secured-signal-api documentation](https://github.com/codeshelldev/secured-signal-api) for detailed setup instructions.

## Recipients

Recipients can be:

- __Phone numbers__: With country code (e.g., +0987654321)
- __Group IDs__: In the format `group.groupId`
- __Usernames__: In the format `u:nickname.123`

!!! Important
    The Signal API rejects mixed recipient types in a single `/v2/send` request.
    Shoutrrr splits them: all phone numbers go in one request, all usernames in another, and each group in its own request.
    A URL that lists phones and a group therefore results in more than one POST.
    If any of those requests fail, the others may already have been sent.

## TLS Configuration

- Use `signal://` for HTTPS (default, recommended)
- Use `signal://...?disabletls=yes` for HTTP (insecure, for local testing only)
- Use `skiptlsverify=yes` to skip certificate-chain trust and hostname validation (expired, untrusted, and hostname-mismatched certificates). TLS 1.2 remains the minimum.

## Examples

### Send to a single phone number

```
signal://localhost:8080/+1234567890/+0987654321
```

### Send to multiple recipients

```
signal://localhost:8080/+1234567890/+0987654321/+1123456789/group.testgroup
```

The two phone numbers are sent together. The group is a second `/v2/send` call.

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

### Send to a username

```
signal://localhost:8080/+1234567890/u:someuser.123
```

### Using HTTP instead of HTTPS

```
signal://localhost:8080/+1234567890/+0987654321?disabletls=yes
```

### Styled message

```
signal://localhost:8080/+1234567890/+0987654321?textmode=styled
```

!!! Note
    - When `textmode=styled`, the message body may include `*italic*`, `**bold**`, `~strikethrough~`, `||spoiler||`, and `` `monospace` ``.
    - Escape a formatting character with two backslashes.
    - If `textmode` is omitted, the JSON `text_mode` field is omitted and the API server default applies.

### Title

Signal has no separate title field.
A non-empty `title` (URL query, or the send `title` param) is prepended as the first line of the message.
With `textmode=styled` the title is wrapped in `**...**`.

```
signal://localhost:8080/+1234567890/+0987654321?title=Alert&textmode=styled
```

## Attachments

Use the `attachments` query parameter or send param with comma-separated raw base64 values.
If the value contains `data:`, it is sent as a single data URI (data URIs contain commas, so they are not split).

```
signal://localhost:8080/+1234567890/+0987654321?attachments=base64data1,base64data2
```

!!! Note "Attachment Format"
    Raw base64 entries may be comma-separated.
    A `data:` URI must be sent as a single value.

## Implementation Notes

Shoutrrr's Signal service sends messages using POST requests to the Signal API server's `/v2/send` endpoint.
Requests use HTTPS by default and HTTP only when TLS is disabled.
The JSON payload contains the message, source number, recipient list, and optional `text_mode`, `notify_self`, and `base64_attachments`.
The API server handles the actual Signal protocol communication.
