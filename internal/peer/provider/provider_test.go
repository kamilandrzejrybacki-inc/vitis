package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func TestTransportEndToEndScripted(t *testing.T) {
	pty := newFakePTY()
	go func() {
		time.Sleep(5 * time.Millisecond)
		pty.emit("hello world\nTURN_END_aaaaaaaaaaaa\n")
	}()

	tx := New(func(_ context.Context, _ model.PeerSpec) (rawPTYProcess, error) {
		return pty, nil
	}, 500*time.Millisecond)

	bus := inproc.New()
	defer bus.Close()
	ctx := context.Background()
	require.NoError(t, tx.Start(ctx, model.PeerSpec{URI: "provider:fake"}, bus, "conv-1", model.PeerSlotA))

	turn, err := tx.Deliver(ctx, model.Envelope{
		ConversationID: "conv-1",
		TurnIndex:      1,
		Body:           "envelope-1",
		MarkerToken:    "TURN_END_aaaaaaaaaaaa",
	})
	require.NoError(t, err)
	require.Equal(t, model.PeerSlotA, turn.From)
	require.Equal(t, 1, turn.Index)
	require.Equal(t, "hello world", strings.TrimSpace(turn.Response))
	require.Equal(t, "TURN_END_aaaaaaaaaaaa", turn.MarkerToken)
	require.NoError(t, tx.Stop(ctx, time.Second))
}

func TestTransportSpawnerError(t *testing.T) {
	tx := New(func(_ context.Context, _ model.PeerSpec) (rawPTYProcess, error) {
		return nil, errSpawn
	}, time.Second)
	bus := inproc.New()
	defer bus.Close()
	err := tx.Start(context.Background(), model.PeerSpec{URI: "provider:fake"}, bus, "conv-1", model.PeerSlotA)
	require.Error(t, err)
	require.ErrorIs(t, err, errSpawn)
}

type sentinelErr string

func (s sentinelErr) Error() string { return string(s) }

var errSpawn sentinelErr = "spawn failed"
