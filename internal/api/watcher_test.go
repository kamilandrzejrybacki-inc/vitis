package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_DetectsConversationWrite(t *testing.T) {
	dir := t.TempDir()

	// Create conversations/<id>/conversation.json
	convDir := filepath.Join(dir, "conversations", "conv-1")
	require.NoError(t, os.MkdirAll(convDir, 0o755))
	convFile := filepath.Join(convDir, "conversation.json")
	require.NoError(t, os.WriteFile(convFile, []byte(`{"id":"conv-1"}`), 0o644))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := NewWatcher(dir)
	require.NoError(t, err)
	defer w.Close()

	events := make(chan WatchEvent, 10)
	go func() {
		for e := range w.Events() {
			events <- e
		}
	}()

	require.NoError(t, w.WatchConversation(ctx, "conv-1"))

	// Simulate a write
	require.NoError(t, os.WriteFile(convFile, []byte(`{"id":"conv-1","status":"running"}`), 0o644))

	select {
	case e := <-events:
		assert.Equal(t, "conv-1", e.ConversationID)
		assert.Equal(t, WatchEventConversationUpdated, e.Kind)
	case <-time.After(2 * time.Second):
		t.Fatal("no event received")
	}
}

func TestWatcher_DetectsTurnsWrite(t *testing.T) {
	dir := t.TempDir()

	convDir := filepath.Join(dir, "conversations", "conv-2")
	require.NoError(t, os.MkdirAll(convDir, 0o755))
	turnsFile := filepath.Join(convDir, "turns.jsonl")
	require.NoError(t, os.WriteFile(turnsFile, []byte(""), 0o644))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := NewWatcher(dir)
	require.NoError(t, err)
	defer w.Close()

	events := make(chan WatchEvent, 10)
	go func() {
		for e := range w.Events() {
			events <- e
		}
	}()

	require.NoError(t, w.WatchConversation(ctx, "conv-2"))

	f, err := os.OpenFile(turnsFile, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString(`{"id":"turn-1"}` + "\n")
	require.NoError(t, err)
	f.Close()

	select {
	case e := <-events:
		assert.Equal(t, "conv-2", e.ConversationID)
		assert.Equal(t, WatchEventTurnAdded, e.Kind)
	case <-time.After(2 * time.Second):
		t.Fatal("no event received")
	}
}
