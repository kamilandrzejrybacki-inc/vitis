// Package policy implements turn-ordering policies for multi-peer
// conversations. The orchestrator delegates next-speaker selection to a
// TurnPolicy implementation after every non-terminal turn.
//
// The v1 implementation shipped in this package is AddressedPolicy, which
// reads a structured <<NEXT: peer-id>> trailer from the replying peer's
// output and falls back to round-robin over the declared peer order when
// the trailer is missing, unparseable, unknown, or self-addressed.
package policy

import "github.com/kamilandrzejrybacki-inc/vitis/internal/model"

// Decision is the output of TurnPolicy.Next for a single turn.
type Decision struct {
	// Next is the peer id that will receive the next envelope.
	Next model.PeerID
	// Parsed is the id extracted from the reply's trailer, if any. It is
	// non-nil when the reply contained a syntactically valid <<NEXT: id>>
	// trailer, even if the id is unknown or self-addressed.
	Parsed *model.PeerID
	// FallbackUsed is true if Next was chosen via round-robin rather than
	// from the parsed trailer.
	FallbackUsed bool
}

// TurnPolicy selects the next speaker given the current peer and their
// reply. Implementations MUST always return a valid Next peer that exists
// in the peers slice; if they cannot, they MUST fall back to a
// deterministic policy (round-robin in declared order).
type TurnPolicy interface {
	Next(current model.PeerID, reply string, peers []model.PeerID) Decision
}
