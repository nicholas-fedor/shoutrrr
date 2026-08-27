# Email

Shoutrrr can send notifications as email over SMTP.
It talks to any RFC 5321 server (Gmail, Microsoft 365, self-hosted Postfix, and similar) using a single service URL.

## URL Format

!!! info ""
    smtp://__`username`__:__`password`__@__`host`__:__`port`__/?fromaddress=__`fromAddress`__&toaddresses=__`recipient1`__[,__`recipient2`__,...]&subject=__`subject`__&auth=__`auth`__&encryption=__`encryption`__&useStartTLS=__`yes/no`__&useHTML=__`yes/no`__&clientHost=__`hostname`__&requirestarttls=__`yes/no`__&skiptlsverify=__`yes/no`__&timeout=__`duration`__

--8<-- "docs/services/email/smtp/config.md"

## Getting Started

Every SMTP URL needs a `host`, a `fromaddress`, and at least one `toaddresses` recipient.

A `Username` and `Password` are required only when the server expects authentication.

!!! Example "Minimal authenticated send"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com
    ```

    Connects to `mail.example.com` on port 587, authenticates as `user`, and sends to `ops@example.com`.

## Configuration Parameters

### Host and Port

- `Host` is the SMTP server hostname or IP address and is required.
- `Port` defaults to `25`.

Common ports:

- __25__: traditional SMTP (often blocked on residential networks)
- __587__: submission with STARTTLS
- __465__: implicit TLS (SMTPS)
- __2525__: alternate submission port used by some providers

!!! Example "Submission on 587"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com
    ```

### Username and Password

- `Username` and `Password` are the SMTP credentials.
- Both default to empty.
- Leave them empty for servers that allow unauthenticated relay.
- For `auth=OAuth2`, put the access token in the password field.

!!! Example "No credentials"

    ```uri
    smtp://mail.example.com:25/?fromaddress=alerts@example.com&toaddresses=ops@example.com&auth=None
    ```

### From Address and From Name

- `fromaddress` (alias `from`) is the envelope and header From address and is required.
- `fromname` is an optional display name shown by mail clients.

!!! Example "Named sender"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&fromname=Shoutrrr&toaddresses=ops@example.com
    ```

### Recipients

- `toaddresses` (alias `to`) is a comma-separated list of recipient addresses and is required.
- Plus-tags in addresses (`user+tag@example.com`) are preserved and spaces that come from URL-decoding `+` are turned back into `+`.

!!! Example "Multiple recipients"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com,oncall@example.com
    ```

!!! Example "Plus-address recipient"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops+prod@example.com
    ```

### Subject

- `subject` (alias `title`) is the email subject.
- When the URL omits `subject`, the header is empty.

!!! Example "Custom subject"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&subject=Disk%20space%20low
    ```

### Authentication

`auth` selects the SMTP AUTH method. Default: `Unknown`.

| Value     | Behavior                                                                       |
|-----------|--------------------------------------------------------------------------------|
| `None`    | No AUTH                                                                        |
| `Plain`   | AUTH PLAIN (username and password)                                             |
| `Login`   | AUTH LOGIN, for servers that do not support PLAIN                              |
| `CRAMMD5` | AUTH CRAM-MD5                                                                  |
| `OAuth2`  | SASL XOAUTH2 with a __static__ access token in the password field (no refresh) |
| `Unknown` | If a username is set, treated as `Plain`; otherwise `None`                     |

!!! Example "AUTH LOGIN"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&auth=Login
    ```

!!! Example "OAuth2 access token"

    ```uri
    smtp://user:ya29.token@smtp.gmail.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&auth=OAuth2&requirestarttls=yes
    ```

    The token is not refreshed.
    Supply a current access token for each send.

### Encryption

- `encryption` selects how TLS is applied. Default: `Auto`.
- `usestarttls` defaults to yes and is independent of `encryption`.
- `encryption=None` still attempts STARTTLS unless you also set `usestarttls=no`.

| Value         | Behavior                                                                     |
|---------------|------------------------------------------------------------------------------|
| `None`        | No implicit TLS; STARTTLS is still attempted unless `usestarttls=no`         |
| `ExplicitTLS` | STARTTLS after connect                                                       |
| `ImplicitTLS` | TLS from the first byte (typical for port 465)                               |
| `Auto`        | Implicit TLS on port 465; otherwise explicit TLS when the server supports it |

!!! Example "Implicit TLS on 465"

    ```uri
    smtp://user:pass@mail.example.com:465/?fromaddress=alerts@example.com&toaddresses=ops@example.com&encryption=ImplicitTLS
    ```

### StartTLS

- `usestarttls` (alias `starttls`) defaults to yes.
- When enabled and the server does not advertise STARTTLS, Shoutrrr logs a warning and continues unencrypted unless `requirestarttls` is set.

!!! Example "Disable STARTTLS"

    ```uri
    smtp://user:pass@mail.example.com:25/?fromaddress=alerts@example.com&toaddresses=ops@example.com&usestarttls=no
    ```

### Require StartTLS

- `requirestarttls` defaults to no.
- When yes, send fails if STARTTLS is enabled but the server does not support it.

!!! Example "Fail closed without STARTTLS"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&requirestarttls=yes
    ```

### Skip TLS Certificate Verification

- `skiptlsverify` defaults to no.
- When TLS is negotiated, `yes` disables server certificate verification; however, it does not enable TLS.

!!! danger "Security Risk"
    Skipping certificate verification makes the connection vulnerable to man-in-the-middle attacks.
    Only use this on networks you trust, such as a lab or an internal mail relay with a private CA.

!!! Example "Skip verify on an internal relay"

    ```uri
    smtp://user:pass@mail.internal:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&skiptlsverify=yes
    ```

### HTML Body

- `usehtml` defaults to no.
- When yes, the message is sent as `multipart/alternative` with a `text/plain` part and a `text/html` part.
- The same message string is used for both parts.
- The HTML part is sent __as-is__.

!!! Example "HTML notification"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&usehtml=yes
    ```

    Pass HTML as the message body, for example `<p>Disk usage is <strong>92%</strong>.</p>`.

### Client Host

- `clienthost` is the hostname sent in the SMTP `EHLO`/`HELO` handshake.
- Default: `localhost`.
- Set it to `auto` to use the operating system hostname (falling back to `localhost` if lookup fails).

!!! Example "Auto client host"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&clienthost=auto
    ```

### Timeout

- `timeout` is a Go duration applied when establishing the SMTP connection.
- Default: `10s`.
- Later commands (EHLO, AUTH, DATA) are not covered by this deadline.

!!! Example "Thirty-second timeout"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&timeout=30s
    ```

## Message Templates

- SMTP templates are only available via the __Shoutrrr API__.
- They are not URL or CLI parameters.
- IDs are `"plain"` and `"HTML"`.
- The template data map has a `message` key.

!!! Example "`<div>`-wrapped message body"

    ```go
    err := service.SetTemplateString("HTML", `<div>{{ .message }}</div>`)
    ```

!!! Example "If you want a `<pre>` wrapper around an HTML part, include it in the message or set an `"HTML"` template"

    ```go
    err := service.SetTemplateString("HTML", `<pre>{{ .message }}</pre>`)
    ```

## Examples

!!! Example "STARTTLS on port 587"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com
    ```

!!! Example "Implicit TLS on port 465"

    ```uri
    smtp://user:pass@mail.example.com:465/?fromaddress=alerts@example.com&toaddresses=ops@example.com&encryption=ImplicitTLS
    ```

!!! Example "HTML body"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops@example.com&usehtml=yes&subject=Alert
    ```

!!! Example "Plus-address recipient"

    ```uri
    smtp://user:pass@mail.example.com:587/?fromaddress=alerts@example.com&toaddresses=ops+prod@example.com
    ```

!!! Example "Unauthenticated local relay"

    ```uri
    smtp://127.0.0.1:25/?fromaddress=alerts@example.com&toaddresses=ops@example.com&auth=None&usestarttls=no
    ```
