// Package xmpp sends fire-and-forget XMPP chat and MUC notifications.
//
// URL format: xmpp://user:pass@host[:port]/?to=a@ex.com&rooms=alerts@conf.ex.com
// URL format: xmpps://user:pass@host[:port]/?to=a@ex.com
//
// xmpp:// uses STARTTLS on port 5222. xmpps:// uses implicit TLS on port 5223.
// disabletls=yes is allowed only on xmpp://. A title is prepended to the body
// because XMPP chat has no separate title field. MUC subject is never changed.
package xmpp
