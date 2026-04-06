package file

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func newTestStore(t *testing.T, debugRaw bool) *Store {
	t.Helper()
	dir := t.TempDir()
	store, err := New(dir, debugRaw)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

func newTestSession(id string) model.Session {
	return model.Session{
		ID:        id,
		Provider:  "claude-code",
		Status:    model.RunRunning,
		StartedAt: time.Now().UTC(),
		AuthMode:  "unknown",
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	store := newTestStore(t, true)
	ctx := context.Background()

	session := newTestSession("sess_test")
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := store.AppendTurn(ctx, model.Turn{
		SessionID: "sess_test",
		Index:     0,
		Role:      "user",
		Content:   "hello",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("append turn: %v", err)
	}

	turns, err := store.PeekTurns(ctx, "sess_test", 10)
	if err != nil {
		t.Fatalf("peek turns: %v", err)
	}
	if len(turns) != 1 || turns[0].Content != "hello" {
		t.Fatalf("unexpected turns: %#v", turns)
	}
}

func TestFileStore_UpdateSession(t *testing.T) {
	store := newTestStore(t, false)
	ctx := context.Background()

	session := newTestSession("sess_update")
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	now := time.Now().UTC()
	exitCode := 0
	warnings := []string{"warn1", "warn2"}
	durationMs := int64(1234)
	status := model.RunCompleted

	patch := model.SessionPatch{
		Status:     &status,
		EndedAt:    &now,
		ExitCode:   &exitCode,
		DurationMs: &durationMs,
		Warnings:   warnings,
	}
	if err := store.UpdateSession(ctx, "sess_update", patch); err != nil {
		t.Fatalf("update session: %v", err)
	}

	// Read back via a fresh store pointed at same dir to confirm persistence.
	var got model.Session
	path := store.sessionPath("sess_update")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal session: %v", err)
	}

	if got.Status != model.RunCompleted {
		t.Errorf("status: got %q, want %q", got.Status, model.RunCompleted)
	}
	if got.ExitCode == nil || *got.ExitCode != exitCode {
		t.Errorf("exit_code: got %v, want %d", got.ExitCode, exitCode)
	}
	if got.DurationMs == nil || *got.DurationMs != durationMs {
		t.Errorf("duration_ms: got %v, want %d", got.DurationMs, durationMs)
	}
	if got.EndedAt == nil {
		t.Error("ended_at: expected non-nil")
	}
	if len(got.Warnings) != 2 || got.Warnings[0] != "warn1" || got.Warnings[1] != "warn2" {
		t.Errorf("warnings: got %v, want %v", got.Warnings, warnings)
	}
}

func TestFileStore_AppendAndPeekMultipleTurns(t *testing.T) {
	store := newTestStore(t, false)
	ctx := context.Background()

	session := newTestSession("sess_multi")
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Append 5 turns out of order to verify sort by index.
	indices := []int{4, 2, 0, 3, 1}
	for _, idx := range indices {
		turn := model.Turn{
			SessionID: "sess_multi",
			Index:     idx,
			Role:      "user",
			Content:   "turn-content-" + string(rune('0'+idx)),
			CreatedAt: time.Now().UTC(),
		}
		if err := store.AppendTurn(ctx, turn); err != nil {
			t.Fatalf("append turn %d: %v", idx, err)
		}
	}

	turns, err := store.PeekTurns(ctx, "sess_multi", 3)
	if err != nil {
		t.Fatalf("peek turns: %v", err)
	}

	if len(turns) != 3 {
		t.Fatalf("expected 3 turns, got %d", len(turns))
	}
	// Last 3 of sorted [0,1,2,3,4] => [2,3,4]
	for i, want := range []int{2, 3, 4} {
		if turns[i].Index != want {
			t.Errorf("turns[%d].Index = %d, want %d", i, turns[i].Index, want)
		}
	}
}

func TestFileStore_AppendStreamEvent(t *testing.T) {
	store := newTestStore(t, true)
	ctx := context.Background()

	session := newTestSession("sess_raw")
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	events := []model.StoredStreamEvent{
		{
			SessionID: "sess_raw",
			Timestamp: time.Now().UTC(),
			Kind:      model.StreamEventOutput,
			Data:      []byte("hello pty"),
		},
		{
			SessionID: "sess_raw",
			Timestamp: time.Now().UTC(),
			Kind:      model.StreamEventInput,
			Data:      []byte("user input"),
		},
	}
	for _, ev := range events {
		if err := store.AppendStreamEvent(ctx, ev); err != nil {
			t.Fatalf("append stream event: %v", err)
		}
	}

	rawPath := store.rawPath("sess_raw")
	f, err := os.Open(rawPath)
	if err != nil {
		t.Fatalf("open raw file: %v", err)
	}
	defer f.Close()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("unmarshal raw line: %v", err)
		}
		lines = append(lines, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan raw file: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 raw lines, got %d", len(lines))
	}
	if lines[0]["kind"] != string(model.StreamEventOutput) {
		t.Errorf("line[0] kind = %v, want %q", lines[0]["kind"], model.StreamEventOutput)
	}
	if lines[1]["kind"] != string(model.StreamEventInput) {
		t.Errorf("line[1] kind = %v, want %q", lines[1]["kind"], model.StreamEventInput)
	}
	// Verify data_base64 field is present and non-empty.
	if v, ok := lines[0]["data_base64"].(string); !ok || v == "" {
		t.Errorf("line[0] data_base64 missing or empty")
	}
}

func TestFileStore_AppendStreamEvent_DebugRawFalse(t *testing.T) {
	store := newTestStore(t, false)
	ctx := context.Background()

	session := newTestSession("sess_noraw")
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	ev := model.StoredStreamEvent{
		SessionID: "sess_noraw",
		Timestamp: time.Now().UTC(),
		Kind:      model.StreamEventOutput,
		Data:      []byte("ignored"),
	}
	if err := store.AppendStreamEvent(ctx, ev); err != nil {
		t.Fatalf("append stream event: %v", err)
	}

	// File should NOT exist when debugRaw=false.
	rawPath := store.rawPath("sess_noraw")
	if _, err := os.Stat(rawPath); !os.IsNotExist(err) {
		t.Errorf("expected raw file to not exist, but stat returned: %v", err)
	}
}

func TestFileStore_PeekTurns_EmptySession(t *testing.T) {
	store := newTestStore(t, false)
	ctx := context.Background()

	session := newTestSession("sess_empty")
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	turns, err := store.PeekTurns(ctx, "sess_empty", 10)
	if err != nil {
		t.Fatalf("peek turns: %v", err)
	}
	if len(turns) != 0 {
		t.Errorf("expected empty slice, got %d turns", len(turns))
	}
}

func TestFileStore_PeekTurns_NoSession(t *testing.T) {
	store := newTestStore(t, false)
	ctx := context.Background()

	// No session or turns file created; PeekTurns should return nil, nil.
	turns, err := store.PeekTurns(ctx, "nonexistent", 5)
	if err != nil {
		t.Fatalf("expected no error for missing session turns, got: %v", err)
	}
	if turns != nil {
		t.Errorf("expected nil slice, got %v", turns)
	}
}

func TestFileStore_CreateSession_DuplicateID(t *testing.T) {
	store := newTestStore(t, false)
	ctx := context.Background()

	session := newTestSession("sess_dup")
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// Second create with same ID — implementation overwrites (atomic rename).
	updated := session
	updated.Status = model.RunCompleted
	if err := store.CreateSession(ctx, updated); err != nil {
		t.Fatalf("second create (overwrite): %v", err)
	}

	var got model.Session
	data, err := os.ReadFile(store.sessionPath("sess_dup"))
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Status != model.RunCompleted {
		t.Errorf("status after overwrite: got %q, want %q", got.Status, model.RunCompleted)
	}
}

func TestFileStore_UpdateSession_NonExistent(t *testing.T) {
	store := newTestStore(t, false)
	ctx := context.Background()

	status := model.RunCompleted
	patch := model.SessionPatch{Status: &status}
	err := store.UpdateSession(ctx, "no_such_session", patch)
	if err == nil {
		t.Fatal("expected error updating non-existent session, got nil")
	}
}

func TestFileStore_PeekTurns_LastNZero(t *testing.T) {
	store := newTestStore(t, false)
	ctx := context.Background()

	session := newTestSession("sess_zero")
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := store.AppendTurn(ctx, model.Turn{
			SessionID: "sess_zero",
			Index:     i,
			Role:      "user",
			Content:   "msg",
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("append turn %d: %v", i, err)
		}
	}

	// lastN=0 means no upper-limit truncation (condition: lastN > 0 is false).
	turns, err := store.PeekTurns(ctx, "sess_zero", 0)
	if err != nil {
		t.Fatalf("peek turns: %v", err)
	}
	if len(turns) != 3 {
		t.Errorf("lastN=0: expected all 3 turns, got %d", len(turns))
	}
}

func TestFileStore_New_EmptyRoot(t *testing.T) {
	_, err := New("", false)
	if err == nil {
		t.Fatal("expected error for empty root, got nil")
	}
}

func TestFileStore_Close(t *testing.T) {
	store := newTestStore(t, false)
	if err := store.Close(); err != nil {
		t.Errorf("close: %v", err)
	}
}
