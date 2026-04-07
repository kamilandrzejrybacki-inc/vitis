package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter/claudecode"
	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter/codex"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/terminal"
)

// NewTerminalSpawner returns a Spawner that resolves the URI scheme of a
// PeerSpec to a concrete adapter, builds an adapter.SpawnSpec, and starts
// a real PTY process via terminal.Runtime.
func NewTerminalSpawner() Spawner {
	rt := terminal.NewRuntime()
	return func(ctx context.Context, spec model.PeerSpec) (rawPTYProcess, error) {
		ad, err := resolveAdapter(spec)
		if err != nil {
			return nil, err
		}
		cwd := spec.Options["cwd"]
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		homeDir := spec.Options["home"]
		if homeDir == "" {
			homeDir = os.Getenv("HOME")
		}
		env := map[string]string{}
		// Pass through any explicitly-set provider env vars from options.
		for k, v := range spec.Options {
			if strings.HasPrefix(k, "env_") {
				env[strings.TrimPrefix(k, "env_")] = v
			}
		}
		spawnSpec := ad.BuildSpawnSpec(cwd, env, homeDir, 80, 24, "")
		// In persistent mode the prompt is delivered turn-by-turn via the
		// PTY, never as part of argv. Force PromptInArgs to false even if
		// the adapter set it.
		spawnSpec.PromptInArgs = false
		proc, err := rt.Spawn(spawnSpec)
		if err != nil {
			return nil, fmt.Errorf("spawn pty for %s: %w", spec.URI, err)
		}
		return proc, nil
	}
}

func resolveAdapter(spec model.PeerSpec) (adapter.Adapter, error) {
	const prefix = "provider:"
	if !strings.HasPrefix(spec.URI, prefix) {
		return nil, fmt.Errorf("provider transport: unsupported URI scheme: %s", spec.URI)
	}
	id := strings.TrimPrefix(spec.URI, prefix)
	switch id {
	case "claude-code", "claudecode":
		return claudecode.New(), nil
	case "codex":
		return codex.New(), nil
	case "mock":
		return newMockAdapter(spec.Options), nil
	default:
		return nil, fmt.Errorf("provider transport: unknown provider %q", id)
	}
}

// mockProviderAdapter is the test-only adapter that runs the mock-agent
// binary identified by spec.Options["bin"] (or MOCK_BIN env var). It is
// declared in this file (not under a build tag) so the integration test
// can drive it without conditional compilation.
type mockProviderAdapter struct {
	bin string
}

func newMockAdapter(opts map[string]string) adapter.Adapter {
	bin := opts["bin"]
	if bin == "" {
		bin = os.Getenv("MOCK_BIN")
	}
	return &mockProviderAdapter{bin: bin}
}

func (m *mockProviderAdapter) ID() string { return "mock" }

func (m *mockProviderAdapter) BuildSpawnSpec(cwd string, env map[string]string, homeDir string, cols, rows int, _ string) adapter.SpawnSpec {
	if env == nil {
		env = map[string]string{}
	}
	env["MOCK_MULTI_TURN"] = "1"
	if env["MOCK_RESPONSE"] == "" {
		env["MOCK_RESPONSE"] = "ok"
	}
	return adapter.SpawnSpec{
		Command:      m.bin,
		Env:          env,
		Cwd:          cwd,
		HomeDir:      homeDir,
		TerminalCols: cols,
		TerminalRows: rows,
	}
}

func (m *mockProviderAdapter) FormatPrompt(raw string) []byte { return []byte(raw + "\n") }

func (m *mockProviderAdapter) Observe(_ adapter.CompletionContext) *adapter.TranscriptObservation {
	return nil
}

func (m *mockProviderAdapter) ExtractResponse(_ []byte, _ string) adapter.ExtractionResult {
	return adapter.ExtractionResult{}
}
