package policy

import "github.com/kamilandrzejrybacki-inc/vitis/internal/model"

// roundRobinAfter returns the peer that follows current in peers, wrapping
// around to peers[0] after the last. If current is not present in peers,
// it returns peers[0]. Panics on empty peers because a turn loop with zero
// peers is an invariant violation the caller should never reach.
func roundRobinAfter(current model.PeerID, peers []model.PeerID) model.PeerID {
	if len(peers) == 0 {
		panic("policy.roundRobinAfter: empty peer list")
	}
	for i, p := range peers {
		if p == current {
			return peers[(i+1)%len(peers)]
		}
	}
	return peers[0]
}
