package conversation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/peer/mock"
	"github.com/kamilandrzejrybacki-inc/clank/internal/terminator"
)

type discardStore struct{}

func (discardStore) CreateConversation(_ context.Context, _ model.Conversation) error {
	return nil
}
func (discardStore) UpdateConversation(_ context.Context, _ string, _ model.ConversationPatch) error {
	return nil
}
func (discardStore) AppendConversationTurn(_ context.Context, _ model.ConversationTurn) error {
	return nil
}

func newConv(maxTurns int) model.Conversation {
	return model.Conversation{
		ID:             "conv-test",
		CreatedAt:      time.Now().UTC(),
		Status:         model.ConvRunning,
		MaxTurns:       maxTurns,
		PerTurnTimeout: 10 * time.Second,
		OverallTimeout: 60 * time.Second,
		Terminator:     model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
		Opener:         model.PeerSlotA,
		PeerA:          model.PeerSpec{URI: "mock:a"},
		PeerB:          model.PeerSpec{URI: "mock:b"},
		SeedA:          "Discuss",
		SeedB:          "Discuss",
	}
}

func TestBrokerStrictAlternation(t *testing.T) {
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
	require.Equal(t, model.ConvMaxTurnsHit, res.Conversation.Status)
	require.Equal(t, 5, len(res.Turns))
	// Turns 1,3,5 from A; turns 2,4 from B (opener=A)
	require.Equal(t, model.PeerSlotA, res.Turns[0].From)
	require.Equal(t, model.PeerSlotB, res.Turns[1].From)
	require.Equal(t, model.PeerSlotA, res.Turns[2].From)
	require.Equal(t, model.PeerSlotB, res.Turns[3].From)
	require.Equal(t, model.PeerSlotA, res.Turns[4].From)
}

func TestBrokerSentinelTermination(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Responses: []string{"hello", "I think we agree.\n<<END>>"}})
	bb := mock.New(mock.Script{Responses: []string{"yes hello"}})
	conv := newConv(50)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, model.ConvCompletedSentinel, res.Conversation.Status)
	require.Equal(t, 3, len(res.Turns)) // a, b, a-with-sentinel
	require.Contains(t, res.Turns[2].Response, "<<END>>")
}

func TestBrokerOpenerB(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Responses: []string{"a-reply"}})
	bb := mock.New(mock.Script{Responses: []string{"b-opens"}})
	conv := newConv(2)
	conv.Opener = model.PeerSlotB
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx := context.Background()
	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, model.PeerSlotB, res.Turns[0].From)
	require.Equal(t, model.PeerSlotA, res.Turns[1].From)
}

func TestBrokerPeerErrorFinalizes(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Err: errForTesting("boom")})
	bb := mock.New(mock.Script{Responses: []string{"unused"}})
	conv := newConv(5)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx := context.Background()
	res, err := br.Run(ctx)
	require.NoError(t, err) // peer errors finalize the conversation, not bubble up
	require.Equal(t, model.ConvError, res.Conversation.Status)
	require.Empty(t, res.Turns)
}

func TestBrokerContextCancellation(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Responses: []string{"a1", "a2", "a3", "a4"}})
	bb := mock.New(mock.Script{Responses: []string{"b1", "b2", "b3", "b4"}})
	conv := newConv(100)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, model.ConvInterrupted, res.Conversation.Status)
}

type stringErr string

func (e stringErr) Error() string    { return string(e) }
func errForTesting(msg string) error { return stringErr(msg) }
