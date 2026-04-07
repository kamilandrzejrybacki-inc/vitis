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

// testAdapterFactories holds test-only adapter factories registered via
// RegisterTestAdapterFactory (called from init() in *_test.go files).
// This avoids shipping test adapters (e.g. "mock") in production binaries
// while still allowing resolveAdapter to find them during test runs.
var testAdapterFactories = map[string]func(map[string]string) adapter.Adapter{}

// RegisterTestAdapterFactory registers a test-only adapter factory for the
// given provider id. It is intended to be called from init() functions in
// _test.go files in any package that needs provider:mock during tests.
func RegisterTestAdapterFactory(id string, factory func(map[string]string) adapter.Adapter) {
	testAdapterFactories[id] = factory
}

// allowedEnvKeys is the set of env var keys that may be forwarded from
// --peer-*-opt env_KEY=val to the spawned subprocess. Any other key is
// silently dropped with a stderr warning. This prevents arbitrary env
// injection (e.g. LD_PRELOAD, CLANK_CLAUDE_ARGS).
var allowedEnvKeys = map[string]bool{
	"ANTHROPIC_API_KEY":    true,
	"OPENAI_API_KEY":       true,
	"MOCK_RESPONSE":        true,
	"MOCK_SENTINEL_AT_TURN": true,
	"MOCK_DELAY_MS":        true,
}

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
		// Forward only allowlisted env vars from peer options.
		for k, v := range spec.Options {
			if strings.HasPrefix(k, "env_") {
				key := strings.TrimPrefix(k, "env_")
				if allowedEnvKeys[key] {
					env[key] = v
				} else {
					fmt.Fprintf(os.Stderr, "clank: dropping disallowed env var %q from peer options\n", key)
				}
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
	default:
		// Check test-registered factories (populated via init() in _test.go files).
		if factory, ok := testAdapterFactories[id]; ok {
			return factory(spec.Options), nil
		}
		return nil, fmt.Errorf("provider transport: unknown provider %q", id)
	}
}
