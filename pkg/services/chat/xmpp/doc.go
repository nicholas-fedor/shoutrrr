// Package xmpp sends fire-and-forget XMPP chat and MUC notifications.
//
// URL format: xmpp://user:pass@host[:port]/?to=a@ex.com&rooms=alerts@conf.ex.com
// URL format: xmpps://user:pass@host[:port]/?to=a@ex.com
//
// xmpp:// uses STARTTLS and xmpps:// uses implicit TLS. A URL without a port,
// whose host is the auth JID's domain, finds the server through the domain's
// SRV records (RFC 6120 section 3.2.1 and XEP-0368). Without records, and for
// any other URL without a port, the host is dialed at 5222 for xmpp:// or
// 5223 for xmpps://.
// The server's certificate is verified against the domain of the auth JID,
// as RFC 6120 section 13.7.2.1 requires. With a full JID, the host can
// therefore be any name or address of the server. xmpps:// offers the
// xmpp-client ALPN protocol, as XEP-0368 asks.
// disabletls=yes is allowed only on xmpp://. The first send without TLS, or
// without certificate verification, logs a warning. A title is prepended to
// the body because XMPP chat has no separate title field. MUC subject is never
// changed.
package xmpp
