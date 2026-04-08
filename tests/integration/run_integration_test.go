//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter/claudecode"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/orchestrator"
	filestore "github.com/kamilandrzejrybacki-inc/vitis/internal/store/file"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminal"
)

// repoRoot returns the absolute path to the repository root.
func repoRoot(t *testing.T) string {
	t.Helper()
	// tests/integration is two levels below repo root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

// newDeps creates a standard Dependencies value for integration tests.
func newDeps(t *testing.T, logPath string) orchestrator.Dependencies {
	t.Helper()
	store, err := filestore.New(logPath, true)
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	return orchestrator.Dependencies{
		Adapters: adapter.NewRegistry(claudecode.New()),
		Runtime:  terminal.NewRuntime(),
		Store:    store,
	}
}

// writeEnvFile writes key=value pairs to a temp file and returns its path.
func writeEnvFile(t *testing.T, dir string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, "vitis.env")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	return path
}

// skipIfSandboxed skips the test when the PTY child is blocked by a sandbox.
func skipIfSandboxed(t *testing.T, result *model.RunResult) {
	t.Helper()
	if result.Status == model.RunFailed &&
		result.Error != nil &&
		strings.Contains(result.Error.Message, "operation not permitted") {
		t.Skip("PTY child execution is blocked by the sandbox in this environment")
	}
}

// TestIntegration_HappyPath verifies a successful run produces a complete result.
func TestIntegration_HappyPath(t *testing.T) {
	root := repoRoot(t)
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs")

	envFile := writeEnvFile(t, tmpDir, []string{
		"VITIS_CLAUDE_BINARY=go",
		"VITIS_CLAUDE_ARGS=run ./internal/testutil/mockagent",
		"MOCK_MODE=happy",
		"MOCK_RESPONSE=integration test response",
	})

	result, err := orchestrator.Run(context.Background(), model.RunRequest{
		Provider:   "claude-code",
		Prompt:     "hello",
		Cwd:        root,
		EnvFile:    envFile,
		LogBackend: "file",
		LogPath:    logPath,
		DebugRaw:   true,
		TimeoutSec: 10,
	}, newDeps(t, logPath))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	skipIfSandboxed(t, result)

	if result.Status != model.RunCompleted {
		t.Fatalf("unexpected status: %s error=%+v meta=%+v", result.Status, result.Error, result.Meta)
	}
	if !strings.Contains(result.Response, "integration test response") {
		t.Fatalf("response does not contain expected text: %q", result.Response)
	}
	if result.SessionID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if result.Meta.BytesCaptured == 0 {
		t.Fatal("expected BytesCaptured > 0")
	}
}

// TestIntegration_BlockedOnInput verifies the blocked_on_input status is returned.
func TestIntegration_BlockedOnInput(t *testing.T) {
	root := repoRoot(t)
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs")

	envFile := writeEnvFile(t, tmpDir, []string{
		"VITIS_CLAUDE_BINARY=go",
		"VITIS_CLAUDE_ARGS=run ./internal/testutil/mockagent",
		"MOCK_MODE=blocked",
	})

	result, err := orchestrator.Run(context.Background(), model.RunRequest{
		Provider:   "claude-code",
		Prompt:     "hello",
		Cwd:        root,
		EnvFile:    envFile,
		LogBackend: "file",
		LogPath:    logPath,
		DebugRaw:   true,
		TimeoutSec: 10,
	}, newDeps(t, logPath))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	skipIfSandboxed(t, result)

	if result.Status == model.RunCompleted {
		t.Fatalf("expected non-completed status, got %s", result.Status)
	}
	if result.Status != model.RunBlockedOnInput {
		t.Fatalf("expected blocked_on_input status, got %s", result.Status)
	}
}

// TestIntegration_AuthRequired verifies the auth_required status is returned.
func TestIntegration_AuthRequired(t *testing.T) {
	root := repoRoot(t)
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs")

	envFile := writeEnvFile(t, tmpDir, []string{
		"VITIS_CLAUDE_BINARY=go",
		"VITIS_CLAUDE_ARGS=run ./internal/testutil/mockagent",
		"MOCK_MODE=auth",
	})

	result, err := orchestrator.Run(context.Background(), model.RunRequest{
		Provider:   "claude-code",
		Prompt:     "hello",
		Cwd:        root,
		EnvFile:    envFile,
		LogBackend: "file",
		LogPath:    logPath,
		DebugRaw:   true,
		TimeoutSec: 10,
	}, newDeps(t, logPath))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	skipIfSandboxed(t, result)

	if result.Status != model.RunAuthRequired {
		t.Fatalf("expected auth_required status, got %s error=%+v", result.Status, result.Error)
	}
}

// TestIntegration_Timeout verifies a slow agent triggers a timeout.
func TestIntegration_Timeout(t *testing.T) {
	root := repoRoot(t)
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs")

	envFile := writeEnvFile(t, tmpDir, []string{
		"VITIS_CLAUDE_BINARY=go",
		"VITIS_CLAUDE_ARGS=run ./internal/testutil/mockagent",
		"MOCK_MODE=happy",
		"MOCK_DELAY_MS=5000",
	})

	result, err := orchestrator.Run(context.Background(), model.RunRequest{
		Provider:   "claude-code",
		Prompt:     "hello",
		Cwd:        root,
		EnvFile:    envFile,
		LogBackend: "file",
		LogPath:    logPath,
		DebugRaw:   true,
		TimeoutSec: 1,
	}, newDeps(t, logPath))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	skipIfSandboxed(t, result)

	if result.Status != model.RunTimeout {
		t.Fatalf("expected timeout status, got %s error=%+v", result.Status, result.Error)
	}
}

// TestIntegration_ResultJSONSchema verifies the result marshals to JSON with required keys.
func TestIntegration_ResultJSONSchema(t *testing.T) {
	root := repoRoot(t)
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs")

	envFile := writeEnvFile(t, tmpDir, []string{
		"VITIS_CLAUDE_BINARY=go",
		"VITIS_CLAUDE_ARGS=run ./internal/testutil/mockagent",
		"MOCK_MODE=happy",
		"MOCK_RESPONSE=schema check",
	})

	result, err := orchestrator.Run(context.Background(), model.RunRequest{
		Provider:   "claude-code",
		Prompt:     "hello",
		Cwd:        root,
		EnvFile:    envFile,
		LogBackend: "file",
		LogPath:    logPath,
		DebugRaw:   true,
		TimeoutSec: 10,
	}, newDeps(t, logPath))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	skipIfSandboxed(t, result)

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	for _, key := range []string{"session_id", "status", "provider"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON missing key %q", key)
		}
	}

	// Verify status is a non-empty string (valid RunStatus)
	statusVal, ok := m["status"].(string)
	if !ok || statusVal == "" {
		t.Errorf("expected status to be a non-empty string, got %v", m["status"])
	}
}

// TestIntegration_FileStoreRoundTrip verifies a run persists session data that can be read back.
func TestIntegration_FileStoreRoundTrip(t *testing.T) {
	root := repoRoot(t)
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs")

	envFile := writeEnvFile(t, tmpDir, []string{
		"VITIS_CLAUDE_BINARY=go",
		"VITIS_CLAUDE_ARGS=run ./internal/testutil/mockagent",
		"MOCK_MODE=happy",
		"MOCK_RESPONSE=roundtrip check",
	})

	store, err := filestore.New(logPath, true)
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}

	result, err := orchestrator.Run(context.Background(), model.RunRequest{
		Provider:   "claude-code",
		Prompt:     "hello",
		Cwd:        root,
		EnvFile:    envFile,
		LogBackend: "file",
		LogPath:    logPath,
		DebugRaw:   true,
		TimeoutSec: 10,
	}, orchestrator.Dependencies{
		Adapters: adapter.NewRegistry(claudecode.New()),
		Runtime:  terminal.NewRuntime(),
		Store:    store,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	skipIfSandboxed(t, result)

	if result.Status != model.RunCompleted {
		t.Fatalf("unexpected status: %s error=%+v", result.Status, result.Error)
	}

	// Verify the log directory contains session files.
	entries, err := os.ReadDir(logPath)
	if err != nil {
		t.Fatalf("read log dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected log directory to contain session files, but it is empty")
	}

	// Verify turns can be read back via PeekTurns.
	turns, err := store.PeekTurns(context.Background(), result.SessionID, 10)
	if err != nil {
		t.Fatalf("peek turns: %v", err)
	}
	_ = turns // turns may be empty for a simple mock run; confirming no error is sufficient
}
