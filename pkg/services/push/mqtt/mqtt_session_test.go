package mqtt

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/eclipse/paho.golang/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// droppedPacket is a pending packet a session dropped, by its size and the
// limit it exceeded.
type droppedPacket struct {
	size  uint64
	limit uint64
}

// TestLimitedSessionReconnect verifies which pending packets paho resends
// when a reconnection resumes the session. Two QoS 1 messages, of 10 and 150
// bytes, wait for their acknowledgements when the connection is lost. A
// resumed session resends each packet within the new CONNACK's limit and
// drops the larger ones, so they are never sent again. A broker that kept no
// session gets neither packet, and the session drops nothing.
func TestLimitedSessionReconnect(t *testing.T) {
	t.Parallel()

	// tests lists reconnections by the CONNACK's Session Present flag and
	// Maximum Packet Size.
	tests := []struct {
		maxPacketSize  *uint32
		name           string
		wantResent     []string
		sessionPresent bool
		wantDropped    bool
	}{
		{name: "smaller limit", sessionPresent: true, maxPacketSize: new(uint32(100)), wantResent: []string{"small"}, wantDropped: true},
		{name: "no limit", sessionPresent: true, wantResent: []string{"small", "large"}},
		{name: "no session", maxPacketSize: new(uint32(100))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var dropped []droppedPacket

			session := newLimitedSession(func(size, limit uint64) {
				dropped = append(dropped, droppedPacket{size: size, limit: limit})
			})

			// The session outlasts a lost connection only when the CONNECT asks
			// for a Session Expiry Interval, as the service's does.
			connect := packets.NewControlPacket(packets.CONNECT).Content.(*packets.Connect)
			connect.Properties.SessionExpiryInterval = new(uint32(sessionExpiryInterval))

			require.NoError(t, session.ConAckReceived(io.Discard, connect, newConnack(false, nil)))

			small, large := pendingPublish(t, session, "small", 10), pendingPublish(t, session, "large", 150)

			require.NoError(t, session.ConnectionLost(nil))

			var resent bytes.Buffer

			require.NoError(t, session.ConAckReceived(&resent, connect, newConnack(tt.sessionPresent, tt.maxPacketSize)))

			assert.Equal(t, tt.wantResent, resentPayloads(t, &resent))

			if tt.wantDropped {
				assert.Equal(t, []droppedPacket{{size: encodedSize(t, large), limit: 100}}, dropped)
				assert.Less(t, encodedSize(t, small), uint64(100))
			} else {
				assert.Empty(t, dropped)
			}
		})
	}
}

// newConnack returns a successful CONNACK.
//
// Parameters:
//   - sessionPresent: whether the broker resumed the session.
//   - maxPacketSize: the Maximum Packet Size it announces, or nil for none.
//
// Returns:
//   - *packets.Connack: the CONNACK.
func newConnack(sessionPresent bool, maxPacketSize *uint32) *packets.Connack {
	connack := packets.NewControlPacket(packets.CONNACK).Content.(*packets.Connack)
	connack.SessionPresent = sessionPresent
	connack.Properties.MaximumPacketSize = maxPacketSize

	return connack
}

// pendingPublish adds a QoS 1 PUBLISH to the session, as paho does before it
// writes one, and leaves it unacknowledged.
//
// Parameters:
//   - t: the test.
//   - session: the session to add it to.
//   - prefix: the start of the payload.
//   - size: the payload size in bytes.
//
// Returns:
//   - *packets.Publish: the packet, with its packet ID.
func pendingPublish(t *testing.T, session *limitedSession, prefix string, size int) *packets.Publish {
	t.Helper()

	publish := packets.NewControlPacket(packets.PUBLISH).Content.(*packets.Publish)
	publish.Topic = "test/topic"
	publish.QoS = 1
	publish.Payload = []byte(prefix + strings.Repeat("x", size-len(prefix)))

	require.NoError(t, session.AddToSession(t.Context(), publish, make(chan packets.ControlPacket, 1)))

	return publish
}

// resentPayloads reads the packets a session resent and returns the start of
// each payload, up to its first "x".
//
// Parameters:
//   - t: the test.
//   - resent: the bytes the session wrote.
//
// Returns:
//   - []string: the payload prefixes in the order they were resent, or nil
//     when nothing was resent.
func resentPayloads(t *testing.T, resent io.Reader) []string {
	t.Helper()

	var prefixes []string

	for {
		packet, err := packets.ReadPacket(resent)
		if errors.Is(err, io.EOF) {
			return prefixes
		}

		require.NoError(t, err)

		publish, ok := packet.Content.(*packets.Publish)
		require.True(t, ok, "resent a packet that is not a PUBLISH")
		assert.True(t, publish.Duplicate)

		prefix, _, _ := strings.Cut(string(publish.Payload), "x")
		prefixes = append(prefixes, prefix)
	}
}

// encodedSize returns the size of a packet as paho writes it.
//
// Parameters:
//   - t: the test.
//   - publish: the packet.
//
// Returns:
//   - uint64: its size in bytes.
func encodedSize(t *testing.T, publish *packets.Publish) uint64 {
	t.Helper()

	size, err := publish.WriteTo(io.Discard)
	require.NoError(t, err)

	return uint64(size)
}
