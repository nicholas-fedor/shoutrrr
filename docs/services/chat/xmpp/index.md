# XMPP

XMPP (Extensible Messaging and Presence Protocol) is an open protocol for near-real-time messaging.
Shoutrrr's XMPP service enables sending fire-and-forget chat and multi-user chat (MUC) notifications to XMPP servers, such as [ejabberd](https://ejabberd.im/) and [Prosody](https://prosody.im/).
Upstream docs: <https://xmpp.org/>

## URL Formats

The XMPP service supports two URL schemes for connection security:

- __`xmpp://`__: STARTTLS, default port 5222

!!! Info ""
    xmpp://__`USER`__:__`PASSWORD`__@__`HOST`__[:__`PORT`__]/?to=__`JID`__[&rooms=__`ROOM_JID`__]

- __`xmpps://`__: implicit TLS, default port 5223

!!! Info ""
    xmpps://__`USER`__:__`PASSWORD`__@__`HOST`__[:__`PORT`__]/?to=__`JID`__[&rooms=__`ROOM_JID`__]

--8<-- "docs/services/chat/xmpp/config.md"

At least one of `to` or `rooms` is required.

## Authentication JID

The URL userinfo is the login identity:

- A localpart (`alice`) becomes `alice@HOST`.
- A full JID (`alice%40jabber.example.com`) is used as-is.

```
xmpp://alice%40jabber.example.com:secret@xmpp.example.com/?to=bob@jabber.example.com
```

The server's certificate is checked against the domain of the JID, as RFC 6120 requires.
In the example, that domain is `jabber.example.com`.
With a full JID, `HOST` can therefore be any name or address of the server.

## Finding the Server

Without a port, Shoutrrr finds the server through the SRV records of the JID's domain, as RFC 6120 prefers.
This applies when `HOST` is the JID's domain, which it always is for a localpart user.

- `xmpp://` reads the `_xmpp-client._tcp` records, and `xmpps://` reads the `_xmpps-client._tcp` records that XEP-0368 defines.
- Shoutrrr dials the servers they name in order of priority and weight, until one connects.
  Each server but the last gets at most 10 seconds.
- Without records, Shoutrrr dials `HOST` at port 5222 for `xmpp://`, or 5223 for `xmpps://`.
- Once the domain has records, `HOST` itself is not dialed.
- A single record whose target is `.` means the domain offers no XMPP service for the scheme, so the send fails.

Shoutrrr dials `HOST` without a lookup in three cases.

- The URL has a port, which Shoutrrr dials.
- The user is a full JID whose domain is not `HOST`.
- `HOST` is an IP address.

In the last two cases, Shoutrrr dials the scheme's default port.

A custom dial function set through the Go API also carries the DNS queries.
One that refuses them makes Shoutrrr dial `HOST` as it would without records.

## TLS

- Use `xmpp://` for STARTTLS.
- Use `xmpps://` for implicit TLS. Shoutrrr offers the `xmpp-client` ALPN protocol, as XEP-0368 asks.
- Use `disabletls=yes` only with `xmpp://` for plaintext on a trusted network. Combining it with `xmpps://` is rejected.
- Use `skiptlsverify=yes` to skip certificate verification (self-signed or internal TLS). TLS 1.2 remains the minimum.

Shoutrrr logs a warning the first time a send skips certificate verification, and the first time a send goes out without TLS.

Plaintext SASL is used only when `disabletls=yes` is set.
Encrypted connections prefer SCRAM-SHA-256, then SCRAM-SHA-1, then PLAIN.

## Recipients

- __`to`__: comma-separated JIDs sent as type=chat.
- __`rooms`__: comma-separated MUC JIDs. The service joins, sends type=groupchat, and leaves.
- __`nick`__: MUC nickname. Defaults to the auth JID localpart.
- __`roompassword`__: password applied to every room in the URL.

A `title` is prepended to the message body. The MUC subject is never changed.

## Examples

### Direct chat

```
xmpp://alice:secret@xmpp.example.com/?to=bob@example.com
```

### Multiple chat recipients and a room

```
xmpp://alice:secret@xmpp.example.com/?to=bob@example.com,carol@example.com&rooms=alerts@conference.example.com
```

### Implicit TLS with a self-signed certificate

```
xmpps://alice:secret@xmpp.example.com/?to=bob@example.com&skiptlsverify=yes
```

### Password-protected room

```
xmpp://alice:secret@xmpp.example.com/?rooms=secret@conference.example.com&nick=shoutrrr&roompassword=s3cret
```
