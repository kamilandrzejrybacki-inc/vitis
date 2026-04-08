package inproc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
)

func TestPublishSubscribeFanOut(t *testing.T) {
	b := New()
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	subA, cancelA, err := b.Subscribe(ctx, "conv/x/turn")
	require.NoError(t, err)
	defer cancelA()

	subB, cancelB, err := b.Subscribe(ctx, "conv/x/turn")
	require.NoError(t, err)
	defer cancelB()

	msg := bus.BusMessage{
		ConversationID: "x",
		Topic:          "conv/x/turn",
		Kind:           bus.KindTurn,
		Payload:        []byte(`{"hello":"world"}`),
		Timestamp:      time.Unix(0, 0),
	}
	require.NoError(t, b.Publish(ctx, "conv/x/turn", msg))

	for _, sub := range []<-chan bus.BusMessage{subA, subB} {
		select {
		case got := <-sub:
			require.Equal(t, msg.ConversationID, got.ConversationID)
			require.Equal(t, msg.Kind, got.Kind)
			require.Equal(t, msg.Payload, got.Payload)
		case <-time.After(time.Second):
			t.Fatal("expected message on subscriber")
		}
	}
}

func TestSubscribeIsolatedByTopic(t *testing.T) {
	b := New()
	defer b.Close()
	ctx := context.Background()

	subTurn, cancelTurn, err := b.Subscribe(ctx, "conv/x/turn")
	require.NoError(t, err)
	defer cancelTurn()
	subCtl, cancelCtl, err := b.Subscribe(ctx, "conv/x/control")
	require.NoError(t, err)
	defer cancelCtl()

	require.NoError(t, b.Publish(ctx, "conv/x/turn", bus.BusMessage{Topic: "conv/x/turn"}))

	select {
	case <-subTurn:
	case <-time.After(time.Second):
		t.Fatal("turn subscriber should have received")
	}
	select {
	case msg := <-subCtl:
		t.Fatalf("control subscriber should not receive turn topic: got %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCancelStopsDelivery(t *testing.T) {
	b := New()
	defer b.Close()
	ctx := context.Background()

	sub, cancel, err := b.Subscribe(ctx, "topic")
	require.NoError(t, err)

	cancel()

	require.NoError(t, b.Publish(ctx, "topic", bus.BusMessage{Topic: "topic"}))
	select {
	case _, open := <-sub:
		require.False(t, open, "expected closed channel after cancel")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscribe channel should be closed after cancel")
	}
}

func TestPublishToNoSubscribersIsNoOp(t *testing.T) {
	b := New()
	defer b.Close()
	require.NoError(t, b.Publish(context.Background(), "nobody", bus.BusMessage{Topic: "nobody"}))
}

func TestCloseClosesAllSubscribers(t *testing.T) {
	b := New()
	ctx := context.Background()
	sub, _, err := b.Subscribe(ctx, "topic")
	require.NoError(t, err)
	require.NoError(t, b.Close())
	select {
	case _, open := <-sub:
		require.False(t, open)
	case <-time.After(time.Second):
		t.Fatal("close should close all subscribers")
	}
}

func TestFullSubscriberDoesNotBlockPublisher(t *testing.T) {
	b := New(WithBufferSize(1))
	defer b.Close()
	ctx := context.Background()

	_, _, err := b.Subscribe(ctx, "topic")
	require.NoError(t, err)

	// Publish 50 messages without anyone draining; the publisher must not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			_ = b.Publish(ctx, "topic", bus.BusMessage{Topic: "topic"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publisher blocked on full subscriber")
	}
}
