package file

import (
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir, true)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	session := model.Session{
		ID:        "sess_test",
		Provider:  "claude-code",
		Status:    model.RunRunning,
		StartedAt: time.Now().UTC(),
		AuthMode:  "unknown",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := store.AppendTurn(model.Turn{
		SessionID: "sess_test",
		Index:     0,
		Role:      "user",
		Content:   "hello",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("append turn: %v", err)
	}

	turns, err := store.PeekTurns("sess_test", 10)
	if err != nil {
		t.Fatalf("peek turns: %v", err)
	}
	if len(turns) != 1 || turns[0].Content != "hello" {
		t.Fatalf("unexpected turns: %#v", turns)
	}
}
