package mqtt

import (
	"bytes"
	"context"
	"math"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/eclipse/paho.golang/paho"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt/mocks"
)

// TestPublishPacketSize verifies that the computed size of a PUBLISH packet
// matches the bytes paho writes for it. The rows cover each QoS, properties,
// and payloads on both sides of each remaining-length boundary. With a
// 10-byte topic, QoS 0 and no properties, the remaining length is 13 bytes
// more than the payload, so 114 and 115 bytes straddle 127, and so on.
func TestPublishPacketSize(t *testing.T) {
	t.Parallel()

	// tests lists publishes by QoS, payload size, and properties.
	tests := []struct {
		properties *paho.PublishProperties
		name       string
		payload    int
		qos        byte
	}{
		{name: "empty", payload: 0},
		{name: "one-byte length at its end", payload: 114},
		{name: "two-byte length at its start", payload: 115},
		{name: "two-byte length at its end", payload: 16370},
		{name: "three-byte length at its start", payload: 16371},
		{name: "three-byte length at its end", payload: 2097138},
		{name: "four-byte length at its start", payload: 2097139},
		{name: "QoS 1", payload: 50, qos: 1},
		{name: "QoS 2", payload: 50, qos: 2},
		{name: "properties", payload: 50, properties: &paho.PublishProperties{ContentType: "text/plain"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			publish := &paho.Publish{
				Topic:      "test/topic",
				QoS:        tt.qos,
				Payload:    []byte(strings.Repeat("a", tt.payload)),
				Properties: tt.properties,
			}

			var encoded bytes.Buffer

			_, err := publish.Packet().ToControlPacket().WriteTo(&encoded)
			require.NoError(t, err)

			assert.Equal(t, uint64(encoded.Len()), publishPacketSize(publish))
		})
	}
}

// TestLimitedConnectionRecordConnack verifies the limit each CONNACK sets. A
// Maximum Packet Size replaces the limit. A CONNACK without one leaves the
// largest packet MQTT can encode. So does the zero MQTT forbids, or a size
// larger than MQTT can encode. Every CONNACK replaces the limit the one
// before it set.
func TestLimitedConnectionRecordConnack(t *testing.T) {
	t.Parallel()

	limit, zero, largest := uint32(1024), uint32(0), uint32(math.MaxUint32)
	connection := newLimitedConnection()

	assert.Equal(t, uint32(maxProtocolPacketSize), connection.maxPacketSize.Load())

	connection.recordConnack(&paho.Connack{Properties: &paho.ConnackProperties{MaximumPacketSize: &limit}})
	assert.Equal(t, limit, connection.maxPacketSize.Load())

	connection.recordConnack(&paho.Connack{Properties: &paho.ConnackProperties{MaximumPacketSize: &zero}})
	assert.Equal(t, uint32(maxProtocolPacketSize), connection.maxPacketSize.Load())

	connection.recordConnack(&paho.Connack{Properties: &paho.ConnackProperties{MaximumPacketSize: &largest}})
	assert.Equal(t, uint32(maxProtocolPacketSize), connection.maxPacketSize.Load())

	connection.recordConnack(&paho.Connack{Properties: &paho.ConnackProperties{MaximumPacketSize: &limit}})
	connection.recordConnack(&paho.Connack{Properties: &paho.ConnackProperties{}})
	assert.Equal(t, uint32(maxProtocolPacketSize), connection.maxPacketSize.Load())
}

// TestLimitedConnectionReconnect verifies that a send after a reconnection
// waits for the new CONNACK rather than using the limit of the connection
// that went down. A wait that outlasts its context fails with the context's
// error, and the new CONNACK ends the wait.
func TestLimitedConnectionReconnect(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connection := newLimitedConnection()
		connection.ConnectionManager = newFakeConnection(t.Context())

		connection.recordConnack(&paho.Connack{Properties: &paho.ConnackProperties{}})
		require.NoError(t, connection.AwaitConnection(t.Context()))

		connection.recordConnectionDown()

		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		require.ErrorIs(t, connection.AwaitConnection(ctx), context.DeadlineExceeded)

		limit := uint32(1024)

		go func() {
			time.Sleep(time.Second)
			connection.recordConnack(&paho.Connack{Properties: &paho.ConnackProperties{MaximumPacketSize: &limit}})
		}()

		require.NoError(t, connection.AwaitConnection(t.Context()))
		assert.Equal(t, limit, connection.maxPacketSize.Load())
	})
}

// TestLimitedConnectionDone verifies that the wrapper reports the wrapped
// connection's Done channel, and none for a connection without one, so the
// service replaces only a connection that can report it ended.
func TestLimitedConnectionDone(t *testing.T) {
	t.Parallel()

	connection := newLimitedConnection()
	connection.ConnectionManager = newFakeConnection(t.Context())

	assert.NotNil(t, connection.Done())

	connection = newLimitedConnection()
	connection.ConnectionManager = mocks.NewMockConnectionManager(t)

	assert.Nil(t, connection.Done())
}
