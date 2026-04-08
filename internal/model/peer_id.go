package model

import (
	"errors"
	"regexp"
)

// PeerID is a stable identifier for a peer within a conversation.
// It is referenced in <<NEXT: peer-id>> trailers and in the bus topic
// layout. PeerID is case-sensitive and must match the validation regex.
type PeerID string

var peerIDRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// ErrInvalidPeerID is returned by PeerID.Validate for ids that do not
// match the required pattern.
var ErrInvalidPeerID = errors.New("invalid peer id: must match ^[a-z][a-z0-9_-]{0,31}$")

// Validate returns nil if the id is well-formed.
func (p PeerID) Validate() error {
	if !peerIDRegex.MatchString(string(p)) {
		return ErrInvalidPeerID
	}
	return nil
}

// String returns the id as a plain string.
func (p PeerID) String() string { return string(p) }

// PeerIDFromSlot maps a legacy PeerSlot to the equivalent PeerID ("a" or "b").
// This supports the back-compat alias path in the CLI and the orchestrator.
func PeerIDFromSlot(s PeerSlot) PeerID { return PeerID(s) }
