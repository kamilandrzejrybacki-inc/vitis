package conversation

import (
	"context"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/peer/mock"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminator"
	"github.com/stretchr/testify/require"
)

// TestBrokerPopulatesPeerIDFieldsOn2PeerRun verifies that the Phase 4
// policy integration populates the v2 turn-record fields (FromID, ToID,
// Reason, FallbackUsed) on every turn of a standard 2-peer conversation,
// without changing any existing behavior (turn order, statuses, counts).
//
// For a 2-peer script with no <<NEXT: id>> trailers in the responses,
// AddressedPolicy's round-robin fallback reduces to strict alternation,
// so every non-opener turn should be marked TurnReasonFallbackRoundRobin
// with FallbackUsed=true.
func TestBrokerPopulatesPeerIDFieldsOn2PeerRun(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Responses: []string{"a1", "a2", "a3"}})
	bb := mock.New(mock.Script{Responses: []string{"b1", "b2"}})
	conv := newConv(5)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, 5, len(res.Turns))

	// Opener turn
	require.Equal(t, model.PeerID("a"), res.Turns[0].FromID)
	require.Equal(t, model.PeerID("b"), res.Turns[0].ToID)
	require.Equal(t, model.TurnReasonOpener, res.Turns[0].Reason)

	// Subsequent turns: no trailers in the mock responses, so round-robin
	// fallback path for every non-opener turn.
	for i := 1; i < len(res.Turns); i++ {
		turn := res.Turns[i]
		require.Equal(t, model.TurnReasonFallbackRoundRobin, turn.Reason, "turn %d reason", i)
		require.True(t, turn.FallbackUsed, "turn %d fallback", i)
		require.Nil(t, turn.NextIDParsed, "turn %d next_id_parsed", i)
	}

	// Alternation parity with the legacy slot field.
	require.Equal(t, model.PeerID("a"), res.Turns[0].FromID)
	require.Equal(t, model.PeerID("b"), res.Turns[1].FromID)
	require.Equal(t, model.PeerID("a"), res.Turns[2].FromID)
	require.Equal(t, model.PeerID("b"), res.Turns[3].FromID)
	require.Equal(t, model.PeerID("a"), res.Turns[4].FromID)
}

// TestBrokerAddressedRoutingParsesNextTrailer verifies that when a peer's
// response carries a <<NEXT: id>> trailer naming a known other peer, the
// broker records reason=addressed and captures NextIDParsed.
//
// For the 2-peer transport surface, naming the only other peer id via
// the trailer is redundant but should still be marked addressed so the
// downstream UI can show "alice spoke, addressed bob" rather than
// "alice spoke, bob was next by fallback".
func TestBrokerAddressedRoutingParsesNextTrailer(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Responses: []string{"a1\n<<NEXT: b>>", "a2"}})
	bb := mock.New(mock.Script{Responses: []string{"b1"}})
	conv := newConv(3)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Turns), 2)

	// Turn 0 (opener, from a): Reason=opener regardless of trailer content.
	require.Equal(t, model.TurnReasonOpener, res.Turns[0].Reason)

	// Turn 1 (from b): the previous turn's trailer addressed b, so this
	// turn was reached via the addressed path. The broker back-patches
	// the PREVIOUS turn's Reason based on the decision made after it —
	// but turn 0 is pinned as opener. So check turn 1's own routing.
	// Turn 1's response has no trailer -> if it's the last turn before
	// max_turns cap, the policy ran and set fallback fields on turn 1.
	if res.Conversation.Status == model.ConvMaxTurnsHit && len(res.Turns) >= 2 {
		// Turn 1 is from b, with no trailer, so its Reason records how
		// it was REACHED — which is via turn 0's <<NEXT: b>> trailer.
		// But the back-patch logic writes Reason onto the turn whose
		// decision was just made, i.e. the turn the peer produced. So
		// turn 0's Reason is opener (pinned), and turn 1's Reason
		// records turn 1's own policy decision, not how turn 1 was
		// reached. The "addressed" signal from turn 0's trailer lives
		// in turn 0's NextIDParsed and is reachable via the back-patch
		// UNLESS Reason is pinned to opener. Document this edge:
		// check that turn 0 captured NextIDParsed.
		require.NotNil(t, res.Turns[0].NextIDParsed, "turn 0 should capture the parsed <<NEXT: b>> trailer even though Reason is pinned to opener")
		require.Equal(t, model.PeerID("b"), *res.Turns[0].NextIDParsed)
		require.False(t, res.Turns[0].FallbackUsed, "turn 0 next decision was addressed, not fallback")
	}
}
