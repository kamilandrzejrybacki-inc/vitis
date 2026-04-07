package terminator

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
	"github.com/kamilandrzejrybacki-inc/clank/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func TestSentinelDetectsAndPublishesVerdict(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	conv := model.Conversation{
		ID:         "conv-1",
		Terminator: model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
	}
	term := NewSentinel("<<END>>")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, term.Start(ctx, conv, b))
	defer term.Stop(context.Background())

	ctlSub, ctlCancel, err := b.Subscribe(ctx, bus.TopicControl(conv.ID))
	require.NoError(t, err)
	defer ctlCancel()

	turn := model.ConversationTurn{
		ConversationID: conv.ID,
		Index:          2,
		Response:       "I think we're done here.\n<<END>>",
	}
	payload, _ := json.Marshal(turn)
	require.NoError(t, b.Publish(ctx, bus.TopicTurn(conv.ID), bus.BusMessage{
		ConversationID: conv.ID,
		Topic:          bus.TopicTurn(conv.ID),
		Kind:           bus.KindTurn,
		Payload:        payload,
		Timestamp:      time.Now(),
	}))

	select {
	case msg := <-ctlSub:
		require.Equal(t, bus.KindControl, msg.Kind)
		var ctl bus.ControlMsg
		require.NoError(t, json.Unmarshal(msg.Payload, &ctl))
		require.Equal(t, bus.ControlVerdict, ctl.Kind)
		require.NotNil(t, ctl.Verdict)
		require.Equal(t, "terminate", ctl.Verdict.Decision)
		require.Equal(t, model.ConvCompletedSentinel, ctl.Verdict.Status)
	case <-time.After(time.Second):
		t.Fatal("expected verdict on control topic")
	}
}

func TestSentinelIgnoresAbsentSentinel(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	conv := model.Conversation{ID: "conv-1"}
	term := NewSentinel("<<END>>")
	ctx := context.Background()
	require.NoError(t, term.Start(ctx, conv, b))
	defer term.Stop(context.Background())

	ctlSub, ctlCancel, err := b.Subscribe(ctx, bus.TopicControl(conv.ID))
	require.NoError(t, err)
	defer ctlCancel()

	payload, _ := json.Marshal(model.ConversationTurn{Response: "still going"})
	require.NoError(t, b.Publish(ctx, bus.TopicTurn(conv.ID), bus.BusMessage{
		ConversationID: conv.ID,
		Topic:          bus.TopicTurn(conv.ID),
		Kind:           bus.KindTurn,
		Payload:        payload,
	}))

	select {
	case <-ctlSub:
		t.Fatal("should not have published a verdict")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSentinelStripFromResponse(t *testing.T) {
	require.Equal(t, "I'm done.", StripSentinel("I'm done.\n<<END>>", "<<END>>"))
	require.Equal(t, "still talking", StripSentinel("still talking", "<<END>>"))
	require.Equal(t, "before", StripSentinel("before<<END>>after", "<<END>>"))
}
