package file

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func TestNew_EmptyRoot(t *testing.T) {
	if _, err := New("", false); err == nil {
		t.Fatal("expected error for empty root")
	}
}

func TestUpdateConversation_NotExist(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	err = s.UpdateConversation(context.Background(), "no-such", model.ConversationPatch{})
	if err == nil {
		t.Fatal("expected error for missing conversation")
	}
}

func TestUpdateSession_NotExist(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	if err := s.UpdateSession(context.Background(), "missing", model.SessionPatch{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPeekConversationTurns_MalformedJSONL(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	// Pre-create the conversations dir and write a malformed jsonl line.
	if err := os.MkdirAll(s.conversationsDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	path := s.conversationTurnPath("c1")
	if err := os.WriteFile(path, []byte("{not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := s.PeekConversationTurns(context.Background(), "c1", 10)
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestPeekConversationTurns_NotExist(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	turns, err := s.PeekConversationTurns(context.Background(), "ghost", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turns != nil {
		t.Errorf("expected nil turns, got %v", turns)
	}
}

func TestPeekConversationTurns_LastNGreaterThanTotal(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		_ = s.AppendConversationTurn(ctx, model.ConversationTurn{
			ConversationID: "c2", Index: i, From: model.PeerSlotA,
		})
	}
	turns, err := s.PeekConversationTurns(ctx, "c2", 99)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 3 {
		t.Errorf("expected 3 turns, got %d", len(turns))
	}
}

func TestPeekTurns_MalformedJSONL(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	if err := os.WriteFile(s.turnPath("s1"), []byte("garbage\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PeekTurns(context.Background(), "s1", 0); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestAppendConversationTurn_Concurrent(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = s.AppendConversationTurn(ctx, model.ConversationTurn{
				ConversationID: "ccc", Index: i, From: model.PeerSlotA,
			})
		}(i)
	}
	wg.Wait()
	turns, err := s.PeekConversationTurns(ctx, "ccc", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 20 {
		t.Errorf("expected 20 turns, got %d", len(turns))
	}
}

func TestWriteJSONAtomic_RenameFailure(t *testing.T) {
	// Pass a path whose parent doesn't exist; rename should fail.
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	bogus := filepath.Join(dir, "no-such-dir", "x.json")
	if err := s.writeJSONAtomic(bogus, map[string]string{"k": "v"}); err == nil {
		t.Fatal("expected write error")
	}
}

func TestAppendJSONL_BadPath(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	bogus := filepath.Join(dir, "missing-dir", "x.jsonl")
	if err := s.appendJSONL(bogus, map[string]string{"k": "v"}); err == nil {
		t.Fatal("expected open error")
	}
}

func TestReadJSON_NotExist(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	var v any
	if err := s.readJSON(filepath.Join(dir, "ghost.json"), &v); err == nil {
		t.Fatal("expected error")
	}
}

func TestReadJSON_BadJSON(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, false)
	defer s.Close()
	p := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(p, []byte("not-json"), 0o600)
	var v map[string]string
	if err := s.readJSON(p, &v); err == nil {
		t.Fatal("expected unmarshal error")
	}
}
