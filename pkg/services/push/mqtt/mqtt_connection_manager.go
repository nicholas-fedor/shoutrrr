package mqtt

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"
)

// ConnectionManager defines the interface for MQTT connection management.
type ConnectionManager interface {
	// AwaitConnection waits for the MQTT connection to be established.
	// It blocks until the connection is ready or the context is canceled.
	AwaitConnection(ctx context.Context) error

	// Publish sends an MQTT message to the broker.
	// Returns a PublishResponse containing the reason code and any properties,
	// or an error if the publish operation fails.
	Publish(ctx context.Context, publish *paho.Publish) (*paho.PublishResponse, error)

	// Disconnect gracefully closes the MQTT connection.
	// It sends a DISCONNECT packet to the broker and waits for the operation
	// to complete or the context to be canceled.
	Disconnect(ctx context.Context) error
}

// connectionEnder is a connection manager that reports when it has shut
// down, as autopaho's does.
type connectionEnder interface {
	// Done returns a channel that closes when the connection has shut down.
	Done() <-chan struct{}
}

// limitedConnection wraps a connection the service opened, and checks each
// publish against the largest packet the broker accepts. The broker announces
// that size in its CONNACK. paho.golang records it but never checks a publish
// against it (eclipse-paho/paho.golang#4). autopaho hands the CONNACK to
// OnConnectionUp only after AwaitConnection returns, so AwaitConnection also
// waits for it, after the first connection and after each reconnection.
type limitedConnection struct {
	// ConnectionManager is the wrapped connection.
	ConnectionManager

	// connected holds the current connection's channel, which closes once
	// its CONNACK has been recorded. A connection that goes down swaps in an
	// open channel, so a send after a reconnection waits for the new CONNACK.
	connected atomic.Pointer[chan struct{}]
	// mu serializes closing and replacing the channel in connected.
	mu sync.Mutex
	// maxPacketSize is the largest packet in bytes the broker accepts, from
	// its latest CONNACK. It starts at the largest packet MQTT can encode.
	maxPacketSize atomic.Uint32
}

// Sizes in an MQTT packet, in bytes.
const (
	// stringLengthSize is the size of the length before an MQTT string, such
	// as the topic of a PUBLISH packet.
	stringLengthSize = 2
	// packetIDSize is the size of the packet ID a PUBLISH above QoS 0 carries.
	packetIDSize = 2
	// maxProtocolPacketSize is the largest packet MQTT can encode. It holds a
	// 1-byte header and a 4-byte remaining length. That length counts up to
	// 268435455 more bytes. A broker that announces no Maximum Packet Size, or
	// a larger one, accepts packets up to this size.
	maxProtocolPacketSize = 1 + 4 + 268435455
)

// newLimitedConnection returns a wrapper that accepts packets up to the
// largest one MQTT can encode until a CONNACK arrives. The caller sets its
// connection once it exists.
//
// Returns:
//   - *limitedConnection: the wrapper.
func newLimitedConnection() *limitedConnection {
	connection := &limitedConnection{
		ConnectionManager: nil,
		connected:         atomic.Pointer[chan struct{}]{},
		mu:                sync.Mutex{},
		maxPacketSize:     atomic.Uint32{},
	}
	connection.connected.Store(new(make(chan struct{})))
	connection.maxPacketSize.Store(maxProtocolPacketSize)

	return connection
}

// AwaitConnection waits for the connection and for its CONNACK, so a publish
// is checked against the broker's limit.
//
// Parameters:
//   - ctx: cancellation and deadline for the wait.
//
// Returns:
//   - error: the wrapped connection's error as it is, or an error wrapping
//     ctx's error when ctx ends the wait for the CONNACK.
func (c *limitedConnection) AwaitConnection(ctx context.Context) error {
	if err := c.ConnectionManager.AwaitConnection(ctx); err != nil {
		return err
	}

	connected := *c.connected.Load()

	// A closed channel is received without locking ctx's channel, which every
	// send on a connection whose CONNACK arrived takes.
	select {
	case <-connected:
		return nil
	default:
	}

	select {
	case <-connected:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("awaiting the broker's CONNACK: %w", ctx.Err())
	}
}

// Done returns the wrapped connection's Done channel, which closes when the
// connection has shut down.
//
// Returns:
//   - <-chan struct{}: the channel, or nil for a connection without one,
//     which never ends on its own.
func (c *limitedConnection) Done() <-chan struct{} {
	if ender, ok := c.ConnectionManager.(connectionEnder); ok {
		return ender.Done()
	}

	return nil
}

// Publish sends publish when its packet fits the broker's limit.
//
// Parameters:
//   - ctx: cancellation and deadline for the publish.
//   - publish: the message to send.
//
// Returns:
//   - *paho.PublishResponse: the broker's answer.
//   - error: an error wrapping [ErrMessageTooLarge] for a packet over the
//     limit, or paho's error for properties it cannot encode, without
//     sending it. Otherwise the wrapped connection's error as it is.
func (c *limitedConnection) Publish(ctx context.Context, publish *paho.Publish) (*paho.PublishResponse, error) {
	size, err := publishPacketSize(publish)
	if err != nil {
		return nil, err
	}

	limit := uint64(c.maxPacketSize.Load())
	if size > limit {
		return nil, fmt.Errorf("%w: the packet would be %d bytes, and the broker accepts at most %d",
			ErrMessageTooLarge, size, limit)
	}

	return c.ConnectionManager.Publish(ctx, publish)
}

// recordConnack records the broker's limit from a CONNACK, which arrives on
// every connection and reconnection, and releases the sends waiting for it.
// paho always fills in its properties.
//
// Parameters:
//   - connack: the broker's CONNACK.
func (c *limitedConnection) recordConnack(connack *paho.Connack) {
	c.maxPacketSize.Store(packetSizeLimit(connack.Properties.MaximumPacketSize))

	c.mu.Lock()
	defer c.mu.Unlock()

	connected := *c.connected.Load()
	select {
	case <-connected:
	default:
		close(connected)
	}
}

// recordConnectionDown makes sends wait for the CONNACK of the next
// connection, whose broker may announce another limit.
func (c *limitedConnection) recordConnectionDown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-*c.connected.Load():
		c.connected.Store(new(make(chan struct{})))
	default:
	}
}

// packetSizeLimit returns the largest packet a broker accepts, from the
// Maximum Packet Size in its CONNACK. A CONNACK without one, or with the zero
// that MQTT forbids, leaves packets limited only by what MQTT can encode. So
// does a limit larger than MQTT can encode.
//
// Parameters:
//   - maximumPacketSize: the CONNACK's Maximum Packet Size, or nil when it
//     has none.
//
// Returns:
//   - uint32: the limit in bytes.
func packetSizeLimit(maximumPacketSize *uint32) uint32 {
	if maximumPacketSize == nil || *maximumPacketSize == 0 {
		return maxProtocolPacketSize
	}

	return min(*maximumPacketSize, maxProtocolPacketSize)
}

// publishPacketSize returns the size of the PUBLISH packet paho encodes for
// publish. The packet holds these parts in order:
//   - a 1-byte header
//   - the remaining length
//   - the topic with its 2-byte length
//   - a 2-byte packet ID above QoS 0
//   - the properties with their length
//   - the payload
//
// Parameters:
//   - publish: the message to send.
//
// Returns:
//   - uint64: the packet size in bytes.
//   - error: paho's error for properties it cannot encode, such as a
//     Subscription Identifier outside 0 to 268435455.
func publishPacketSize(publish *paho.Publish) (uint64, error) {
	var properties uint64

	if publish.Properties != nil {
		packed, err := publish.Packet().Properties.Pack(packets.PUBLISH)
		if err != nil {
			return 0, fmt.Errorf("packing PUBLISH properties: %w", err)
		}

		properties = uint64(len(packed))
	}

	remaining := stringLengthSize + uint64(len(publish.Topic)) + uint64(len(publish.Payload)) +
		variableByteIntegerLength(properties) + properties
	if publish.QoS > 0 {
		remaining += packetIDSize
	}

	return 1 + variableByteIntegerLength(remaining) + remaining, nil
}

// variableByteIntegerLength returns how many bytes MQTT's variable byte
// integer takes for value, 7 bits per byte. A value over 268435455 needs more
// than the 4 bytes MQTT allows, which makes its packet too large for any
// broker.
//
// Parameters:
//   - value: the integer.
//
// Returns:
//   - uint64: the encoded length in bytes.
func variableByteIntegerLength(value uint64) uint64 {
	length := uint64(1)
	for ; value >= 128; value >>= 7 {
		length++
	}

	return length
}
