// Package peer defines the PeerTransport interface used by the conversation
// broker. Concrete implementations live in subpackages:
//
//	internal/peer/provider     - local persistent PTY peer (Plan 2)
//	internal/peer/clankremote  - remote vitis peer over the bus (Plan 4)
//	internal/peer/stdio        - this process's stdin/stdout (Plan 5)
//	internal/peer/mock         - scripted in-memory transport for tests
//
// The broker only ever talks to PeerTransport. It never imports a concrete
// transport package; CLI wiring builds the transport and passes it in.
package peer

import (
	"context"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// PeerTransport is the broker's view of a participant in a conversation.
//
// Lifecycle:
//  1. Start brings the peer online and returns when it is ready to receive
//     its first envelope. For provider transports this means spawning a
//     PTY and waiting for the adapter's ready signal. For network transports
//     it means handshaking over the bus. Idempotent within a single
//     conversation; calling Start twice is a programming error.
//  2. Deliver hands one envelope to the peer and blocks until either the
//     response turn is captured (success) or an error occurs. Deliver is
//     called serially by the broker — at most one Deliver in flight at a
//     time per peer per conversation.
//  3. Stop terminates the peer with a grace period. After Stop, neither
//     Deliver nor Start may be called.
type PeerTransport interface {
	Start(ctx context.Context, spec model.PeerSpec, b bus.Bus, conversationID string, slot model.PeerSlot) error
	Deliver(ctx context.Context, env model.Envelope) (model.ConversationTurn, error)
	Stop(ctx context.Context, grace time.Duration) error
}
