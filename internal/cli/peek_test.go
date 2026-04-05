package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	filestore "github.com/kamilandrzejrybacki-inc/clank/internal/store/file"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func TestPeekCommand_MissingSessionID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := PeekCommand(context.Background(), []string{}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when session-id is missing")
	}
	output := stdout.String()
	if !strings.Contains(output, "session-id") {
		t.Fatalf("expected error mentioning session-id, got: %s", output)
	}
}

func TestPeekCommand_FileBackend(t *testing.T) {
	dir := t.TempDir()

	store, err := filestore.New(dir, false)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	session := model.Session{
		ID:        "test-session-peek",
		Provider:  "claude-code",
		Status:    model.RunCompleted,
		StartedAt: time.Now().UTC(),
		AuthMode:  "unknown",
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	turn := model.Turn{
		SessionID: "test-session-peek",
		Index:     0,
		Role:      "user",
		Content:   "hello from test",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.AppendTurn(turn); err != nil {
		t.Fatalf("append turn: %v", err)
	}
	store.Close()

	var stdout, stderr bytes.Buffer
	code := PeekCommand(context.Background(), []string{
		"--session-id", "test-session-peek",
		"--log-backend", "file",
		"--log-path", dir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s; stdout: %s", code, stderr.String(), stdout.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "test-session-peek") {
		t.Fatalf("output does not contain session_id; got: %s", output)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output: %s", err, output)
	}

	if result["session_id"] != "test-session-peek" {
		t.Fatalf("unexpected session_id in output: %v", result["session_id"])
	}

	turns, ok := result["turns"].([]any)
	if !ok {
		t.Fatalf("expected turns array in output, got: %T", result["turns"])
	}
	if len(turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(turns))
	}
}

func TestPeekCommand_MissingLogPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := PeekCommand(context.Background(), []string{
		"--session-id", "some-session",
		"--log-backend", "file",
		"--log-path", "",
	}, &stdout, &stderr)

	if code == 0 {
		t.Fatal("expected non-zero exit code when log-path is empty")
	}
}
