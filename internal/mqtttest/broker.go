package mqtttest

import (
	"net"
	"sync/atomic"
	"testing"

	"github.com/eclipse/paho.golang/packets"
)

// Broker is an in-process MQTT broker on a loopback listener. It accepts every
// connection and records the PUBLISH and DISCONNECT packets clients send.
type Broker struct {
	// listener accepts client connections.
	listener net.Listener
	// published counts PUBLISH packets.
	published atomic.Int32
	// disconnected counts DISCONNECT packets.
	disconnected atomic.Int32
}

// NewBroker starts a broker on a loopback port and stops it when the test ends.
//
// Parameters:
//   - t: the test that owns the broker.
//
// Returns:
//   - *Broker: the running broker.
func NewBroker(t *testing.T) *Broker {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting the MQTT test broker: %v", err)
	}

	broker := &Broker{listener: listener, published: atomic.Int32{}, disconnected: atomic.Int32{}}

	t.Cleanup(func() { _ = listener.Close() })

	go broker.accept()

	return broker
}

// Addr returns the broker's host and port.
//
// Returns:
//   - string: the listener address, such as 127.0.0.1:41234.
func (b *Broker) Addr() string {
	return b.listener.Addr().String()
}

// Disconnected returns how many DISCONNECT packets the broker has received.
//
// Returns:
//   - int32: the number of DISCONNECT packets.
func (b *Broker) Disconnected() int32 {
	return b.disconnected.Load()
}

// Published returns how many PUBLISH packets the broker has received.
//
// Returns:
//   - int32: the number of PUBLISH packets.
func (b *Broker) Published() int32 {
	return b.published.Load()
}

// accept serves each connection until the listener closes.
func (b *Broker) accept() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}

		go b.serve(conn)
	}
}

// serve answers CONNECT and records PUBLISH and DISCONNECT until the client
// disconnects or the connection closes.
//
// It reads PINGREQ without answering. The client pings as soon as it connects,
// and a one-shot send can close before reading a PINGRESP. On Windows, closing a
// socket with unread data resets the connection, and the reset discards the
// PUBLISH and DISCONNECT the broker has not read yet. The client's 20 second
// keepalive never expires during a test, so the unanswered ping is harmless.
//
// Parameters:
//   - conn: the client connection.
func (b *Broker) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	for {
		packet, err := packets.ReadPacket(conn)
		if err != nil {
			return
		}

		switch packet.Content.(type) {
		case *packets.Connect:
			if _, err := packets.NewControlPacket(packets.CONNACK).WriteTo(conn); err != nil {
				return
			}
		case *packets.Pingreq:
		case *packets.Publish:
			b.published.Add(1)
		case *packets.Disconnect:
			b.disconnected.Add(1)

			return
		default:
		}
	}
}
