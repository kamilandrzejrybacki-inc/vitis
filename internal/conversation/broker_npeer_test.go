package conversation

import (
	"context"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/peer"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/peer/mock"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminator"
	"github.com/stretchr/testify/require"
)

// newConv3 builds a v2 3-peer Conversation. Each peer's seed is the same
// for simplicity; tests that need asymmetric seeds can override.
func newConv3(maxTurns int) model.Conversation {
	return model.Conversation{
		ID:             "conv-3p",
		Status:         model.ConvRunning,
		MaxTurns:       maxTurns,
		PerTurnTimeout: 5,
		OverallTimeout: 30,
		Terminator:     model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
		SchemaVersion:  2,
		Peers: []model.PeerParticipant{
			{ID: "alice", Spec: model.PeerSpec{URI: "mock:alice"}},
			{ID: "bob", Spec: model.PeerSpec{URI: "mock:bob"}},
			{ID: "carol", Spec: model.PeerSpec{URI: "mock:carol"}},
		},
		Seeds: map[model.PeerID]string{
			"alice": "discuss",
			"bob":   "discuss",
			"carol": "discuss",
		},
		OpenerID: "alice",
	}
}

// TestBrokerNPeerRoundRobinFallback runs a 3-peer conversation where no
// peer emits a <<NEXT: id>> trailer, so every turn falls back to round-
// robin in declared order: alice -> bob -> carol -> alice -> ...
func TestBrokerNPeerRoundRobinFallback(t *testing.T) {
	b := inproc.New()
	defer b.Close()

	alice := mock.New(mock.Script{Responses: []string{"a1", "a2", "a3"}})
	bob := mock.New(mock.Script{Responses: []string{"b1", "b2"}})
	carol := mock.New(mock.Script{Responses: []string{"c1", "c2"}})

	conv := newConv3(6)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
		PeersByID: map[model.PeerID]peer.PeerTransport{
			"alice": alice,
			"bob":   bob,
			"carol": carol,
		},
		PeerOrder: []model.PeerID{"alice", "bob", "carol"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, model.ConvMaxTurnsHit, res.Conversation.Status)
	require.Equal(t, 6, len(res.Turns))

	// alice -> bob -> carol -> alice -> bob -> carol
	want := []model.PeerID{"alice", "bob", "carol", "alice", "bob", "carol"}
	for i, w := range want {
		require.Equal(t, w, res.Turns[i].FromID, "turn %d FromID", i)
	}

	// Opener pinned; everything after is fallback.
	require.Equal(t, model.TurnReasonOpener, res.Turns[0].Reason)
	for i := 1; i < len(res.Turns); i++ {
		require.Equal(t, model.TurnReasonFallbackRoundRobin, res.Turns[i].Reason, "turn %d", i)
		require.True(t, res.Turns[i].FallbackUsed, "turn %d", i)
	}
}

// TestBrokerNPeerAddressedRouting runs a 3-peer conversation where each
// reply names the next speaker explicitly via <<NEXT: id>>. The order is
// alice -> carol -> bob -> alice (skipping the natural round-robin).
func TestBrokerNPeerAddressedRouting(t *testing.T) {
	b := inproc.New()
	defer b.Close()

	alice := mock.New(mock.Script{Responses: []string{
		"hi carol\n<<NEXT: carol>>",
		"alice again\n<<NEXT: bob>>",
	}})
	bob := mock.New(mock.Script{Responses: []string{
		"bob says\n<<NEXT: alice>>",
	}})
	carol := mock.New(mock.Script{Responses: []string{
		"carol says\n<<NEXT: bob>>",
	}})

	conv := newConv3(4)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
		PeersByID: map[model.PeerID]peer.PeerTransport{
			"alice": alice,
			"bob":   bob,
			"carol": carol,
		},
		PeerOrder: []model.PeerID{"alice", "bob", "carol"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Turns), 4)

	// alice -> carol -> bob -> alice
	want := []model.PeerID{"alice", "carol", "bob", "alice"}
	for i, w := range want {
		require.Equal(t, w, res.Turns[i].FromID, "turn %d FromID", i)
	}

	// Turn 0 is opener but its decision was addressed (NextIDParsed=carol).
	require.Equal(t, model.TurnReasonOpener, res.Turns[0].Reason)
	require.NotNil(t, res.Turns[0].NextIDParsed)
	require.Equal(t, model.PeerID("carol"), *res.Turns[0].NextIDParsed)
	require.False(t, res.Turns[0].FallbackUsed)

	// Turns 1-3 are addressed (each carries a clean trailer).
	for i := 1; i < 4; i++ {
		require.Equal(t, model.TurnReasonAddressed, res.Turns[i].Reason, "turn %d reason", i)
		require.False(t, res.Turns[i].FallbackUsed, "turn %d fallback", i)
	}
}

// TestBrokerNPeerUnknownAddresseeFallsBack verifies that a <<NEXT: ghost>>
// trailer naming an undeclared peer logs the parsed id but falls back to
// round-robin in declared order. NextIDParsed should still capture "ghost"
// for downstream observability.
func TestBrokerNPeerUnknownAddresseeFallsBack(t *testing.T) {
	b := inproc.New()
	defer b.Close()

	alice := mock.New(mock.Script{Responses: []string{
		"hi\n<<NEXT: ghost>>",
		"a2",
	}})
	bob := mock.New(mock.Script{Responses: []string{"b1"}})
	carol := mock.New(mock.Script{Responses: []string{"never"}})

	conv := newConv3(3)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
		PeersByID: map[model.PeerID]peer.PeerTransport{
			"alice": alice, "bob": bob, "carol": carol,
		},
		PeerOrder: []model.PeerID{"alice", "bob", "carol"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Turns), 2)

	// Turn 0 (alice's reply with the bogus trailer):
	// - NextIDParsed should be "ghost" (the parser captured what it saw)
	// - FallbackUsed = true (id wasn't in PeerOrder)
	// - Reason = opener (pinned for the first turn regardless)
	require.NotNil(t, res.Turns[0].NextIDParsed)
	require.Equal(t, model.PeerID("ghost"), *res.Turns[0].NextIDParsed)
	require.True(t, res.Turns[0].FallbackUsed)
	require.Equal(t, model.TurnReasonOpener, res.Turns[0].Reason)

	// Turn 1 should be from bob (round-robin: alice -> bob).
	require.Equal(t, model.PeerID("bob"), res.Turns[1].FromID)
}

// TestBrokerNPeerSelfAddressFallsBack verifies that <<NEXT: alice>>
// emitted by alice itself is rejected and falls back to round-robin.
// The self-monologue lock vector is closed at the policy layer.
func TestBrokerNPeerSelfAddressFallsBack(t *testing.T) {
	b := inproc.New()
	defer b.Close()

	alice := mock.New(mock.Script{Responses: []string{
		"hi me\n<<NEXT: alice>>",
		"a2",
	}})
	bob := mock.New(mock.Script{Responses: []string{"b1"}})
	carol := mock.New(mock.Script{Responses: []string{"never"}})

	conv := newConv3(3)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
		PeersByID: map[model.PeerID]peer.PeerTransport{
			"alice": alice, "bob": bob, "carol": carol,
		},
		PeerOrder: []model.PeerID{"alice", "bob", "carol"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(res.Turns), 2)

	require.NotNil(t, res.Turns[0].NextIDParsed)
	require.Equal(t, model.PeerID("alice"), *res.Turns[0].NextIDParsed)
	require.True(t, res.Turns[0].FallbackUsed, "self-address must trigger fallback")

	require.Equal(t, model.PeerID("bob"), res.Turns[1].FromID, "round-robin gives the next slot to bob")
}

// TestBrokerNPeerSentinelEnds verifies that <<END>> from a middle peer
// terminates the 3-peer conversation cleanly with the correct status.
func TestBrokerNPeerSentinelEnds(t *testing.T) {
	b := inproc.New()
	defer b.Close()

	alice := mock.New(mock.Script{Responses: []string{"hi"}})
	bob := mock.New(mock.Script{Responses: []string{"bye\n<<END>>"}})
	carol := mock.New(mock.Script{Responses: []string{"never reached"}})

	conv := newConv3(10)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
		PeersByID: map[model.PeerID]peer.PeerTransport{
			"alice": alice,
			"bob":   bob,
			"carol": carol,
		},
		PeerOrder: []model.PeerID{"alice", "bob", "carol"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, model.ConvCompletedSentinel, res.Conversation.Status)
	require.Equal(t, 2, len(res.Turns)) // alice + bob; carol never spoke
	require.Equal(t, model.PeerID("alice"), res.Turns[0].FromID)
	require.Equal(t, model.PeerID("bob"), res.Turns[1].FromID)
	// Carol was never Deliver()'d but should have been Stop()'d on cleanup.
	require.GreaterOrEqual(t, carol.StopCalls(), 1)
}
