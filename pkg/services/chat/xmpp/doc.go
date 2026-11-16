// Package xmpp sends fire-and-forget XMPP chat and MUC notifications.
//
// URL format: xmpp://user:pass@host[:port]/?to=a@ex.com&rooms=alerts@conf.ex.com
// URL format: xmpps://user:pass@host[:port]/?to=a@ex.com
//
// xmpp:// uses STARTTLS on port 5222. xmpps:// uses implicit TLS on port 5223.
// The server's certificate is verified against the domain of the auth JID,
// as RFC 6120 section 13.7.2.1 requires. With a full JID, the host can
// therefore be any name or address of the server. xmpps:// offers the
// xmpp-client ALPN protocol, as XEP-0368 asks. The service dials the host and
// does not look up SRV records.
// disabletls=yes is allowed only on xmpp://. The first send without TLS, or
// without certificate verification, logs a warning. A title is prepended to
// the body because XMPP chat has no separate title field. MUC subject is never
// changed.
package xmpp
