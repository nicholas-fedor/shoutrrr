package types

import (
	"context"
	"net"
)

// DialContextFunc is the signature of [net.Dialer.DialContext] and
// [http.Transport.DialContext].
//
// Implementations must be safe for concurrent use. A nil value means the
// service should use its default dialer.
type DialContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// DialContextSetter is implemented by services that accept a custom TCP dial
// function for non-HTTP connections. This enables callers to control egress
// and destination policy without global side effects.
type DialContextSetter interface {
	// SetDialContext sets the function the service uses to open TCP
	// connections.
	//
	// The router calls it after [Service.Initialize] when
	// [SenderOptions.DialContext] is set.
	//
	// Parameters:
	//   - dial: the dial function, or nil for the service's default dialer.
	SetDialContext(dial DialContextFunc)
}
