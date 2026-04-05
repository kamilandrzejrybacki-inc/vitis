package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter/claudecode"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	filestore "github.com/kamilandrzejrybacki-inc/clank/internal/store/file"
	"github.com/kamilandrzejrybacki-inc/clank/internal/terminal"
)

func TestRunWithRealPTYAndMockAgent(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot = filepath.Clean(filepath.Join(repoRoot, "..", ".."))

	tempDir := t.TempDir()
	envFile := filepath.Join(tempDir, "clank.env")
	envBody := strings.Join([]string{
		"CLANK_CLAUDE_BINARY=go",
		"CLANK_CLAUDE_ARGS=run ./internal/testutil/mockagent",
		"MOCK_MODE=happy",
		"MOCK_RESPONSE=integration response",
	}, "\n")
	if err := os.WriteFile(envFile, []byte(envBody), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	store, err := filestore.New(filepath.Join(tempDir, "logs"), true)
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}

	result, err := Run(context.Background(), model.RunRequest{
		Provider:   "claude-code",
		Prompt:     "hello from test",
		Cwd:        repoRoot,
		EnvFile:    envFile,
		LogBackend: "file",
		LogPath:    filepath.Join(tempDir, "logs"),
		DebugRaw:   true,
		TimeoutSec: 10,
	}, Dependencies{
		Adapters: adapter.NewRegistry(claudecode.New()),
		Runtime:  terminal.NewRuntime(),
		Store:    store,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status == model.RunFailed && result.Error != nil && strings.Contains(result.Error.Message, "operation not permitted") {
		t.Skip("PTY child execution is blocked by the sandbox in this environment")
	}
	if result.Status != model.RunCompleted {
		t.Fatalf("unexpected status: %s error=%+v meta=%+v", result.Status, result.Error, result.Meta)
	}
	if !strings.Contains(result.Response, "integration response") {
		t.Fatalf("unexpected response: %q", result.Response)
	}
	if result.Meta.BytesCaptured == 0 {
		t.Fatalf("expected transcript bytes to be captured")
	}
}
