package mqtt

import (
	"fmt"
	"io"

	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho/session/state"
	"github.com/eclipse/paho.golang/paho/store/memory"
)

// limitedSession is the session state of a connection the service opens.
// When a reconnection resumes the session, paho resends the packets still
// waiting for an acknowledgement as soon as the CONNACK arrives, without
// checking them against the Maximum Packet Size in that CONNACK. So
// limitedSession first drops each pending packet larger than that limit.
// Otherwise a broker that announces a smaller limit than before would
// disconnect the client again on every reconnection. The send that published
// such a packet has already failed with the connection it used.
type limitedSession struct {
	*state.State

	// store holds the packets the client sent and the broker has not yet
	// acknowledged. State resends them from it.
	store *memory.Store
	// dropped is called with the size and limit in bytes of each pending
	// packet the session drops.
	dropped func(size, limit uint64)
}

// newLimitedSession returns an empty in-memory session.
//
// Parameters:
//   - dropped: called with the size and limit in bytes of each pending
//     packet the session drops.
//
// Returns:
//   - *limitedSession: the session.
func newLimitedSession(dropped func(size, limit uint64)) *limitedSession {
	store := memory.New()

	return &limitedSession{
		State:   state.New(store, memory.New()),
		store:   store,
		dropped: dropped,
	}
}

// ConAckReceived drops the pending packets larger than the CONNACK's limit
// when the broker resumed the session. paho's session state then resends the
// rest.
//
// Parameters:
//   - conn: the connection the CONNACK arrived on.
//   - connect: the CONNECT the client sent.
//   - connack: the broker's CONNACK.
//
// Returns:
//   - error: the failure to read the pending packets, or the session state's
//     error, either of which drops the connection.
func (s *limitedSession) ConAckReceived(conn io.Writer, connect *packets.Connect, connack *packets.Connack) error {
	if connack.SessionPresent {
		if err := s.dropOversizedPackets(uint64(packetSizeLimit(connack.Properties.MaximumPacketSize))); err != nil {
			return err
		}
	}

	return s.State.ConAckReceived(conn, connect, connack) //nolint:wrapcheck // paho drops the connection on any error.
}

// dropOversizedPackets removes each pending packet larger than limit from
// the store. paho resends only the packets the store lists, so a dropped
// packet is never sent again. Its packet ID stays reserved until the session
// ends.
//
// Parameters:
//   - limit: the largest packet in bytes the broker accepts.
//
// Returns:
//   - error: the failure to list, read, or remove a pending packet.
func (s *limitedSession) dropOversizedPackets(limit uint64) error {
	ids, err := s.store.List()
	if err != nil {
		return fmt.Errorf("listing the session's pending packets: %w", err)
	}

	for _, packetID := range ids {
		packet, err := s.store.Get(packetID)
		if err != nil {
			return fmt.Errorf("reading pending packet %d: %w", packetID, err)
		}

		read, err := io.Copy(io.Discard, packet)
		_ = packet.Close()

		if err != nil {
			return fmt.Errorf("reading pending packet %d: %w", packetID, err)
		}

		size := uint64(max(read, 0))
		if size <= limit {
			continue
		}

		if err := s.store.Delete(packetID); err != nil {
			return fmt.Errorf("dropping pending packet %d: %w", packetID, err)
		}

		s.dropped(size, limit)
	}

	return nil
}
