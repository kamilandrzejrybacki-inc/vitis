package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter/claudecode"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter/codex"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminal"
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
// injection (e.g. LD_PRELOAD, VITIS_CLAUDE_ARGS).
var allowedEnvKeys = map[string]bool{
	"ANTHROPIC_API_KEY":     true,
	"OPENAI_API_KEY":        true,
	"MOCK_RESPONSE":         true,
	"MOCK_SENTINEL_AT_TURN": true,
	"MOCK_DELAY_MS":         true,
}

// NewTerminalSpawner returns a Spawner that resolves the URI scheme of a
// PeerSpec to a concrete adapter, builds an adapter.SpawnSpec, and starts
// a real PTY process via terminal.Runtime.
func NewTerminalSpawner() Spawner {
	rt := terminal.NewRuntime()
	return func(ctx context.Context, spec model.PeerSpec) (rawPTYProcess, error) {
		spawnSpec, err := buildPersistentSpawnSpec(spec)
		if err != nil {
			return nil, err
		}
		// In persistent mode the prompt is delivered turn-by-turn via the
		// PTY, never as part of argv. Force PromptInArgs to false as a safety
		// override even if a particular branch builds otherwise.
		spawnSpec.PromptInArgs = false
		proc, err := rt.Spawn(spawnSpec)
		if err != nil {
			return nil, fmt.Errorf("spawn pty for %s: %w", spec.URI, err)
		}
		return proc, nil
	}
}

// buildPersistentSpawnSpec resolves a PeerSpec into an adapter.SpawnSpec for
// converse mode (long-lived interactive PTY). Unlike the single-shot adapter
// path, this never includes one-shot subcommands (e.g. codex's `exec`) or
// trailing prompt arguments — the prompt is written into the PTY turn-by-turn
// via PersistentProcess.ConverseTurn.
func buildPersistentSpawnSpec(spec model.PeerSpec) (adapter.SpawnSpec, error) {
	const prefix = "provider:"
	if !strings.HasPrefix(spec.URI, prefix) {
		return adapter.SpawnSpec{}, fmt.Errorf("provider transport: unsupported URI scheme: %s", spec.URI)
	}
	id := strings.TrimPrefix(spec.URI, prefix)

	cwd := spec.Options["cwd"]
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	homeDir := spec.Options["home"]
	if homeDir == "" {
		homeDir = os.Getenv("HOME")
	}
	env := buildPeerEnv(spec.Options)

	switch id {
	case "claude-code", "claudecode":
		// claudecode's BuildSpawnSpec is already interactive: no subcommand,
		// no prompt argument, no PromptInArgs. Use it directly.
		return claudecode.New().BuildSpawnSpec(cwd, env, homeDir, 80, 24, ""), nil
	case "codex":
		// codex's BuildSpawnSpec returns the one-shot `codex exec ...` form
		// with the prompt as the trailing argv. For converse mode we want
		// plain `codex` (interactive) with no `exec` subcommand and no
		// trailing prompt arg.
		binary, _ := codex.ResolveCommand(env)
		var args []string
		if m := env["VITIS_MODEL"]; m != "" {
			args = append(args, "--model", m)
		}
		if re := env["VITIS_REASONING_EFFORT"]; re != "" {
			args = append(args, "--reasoning-effort", re)
		}
		return adapter.SpawnSpec{
			Command:      binary,
			Args:         args,
			Env:          env,
			Cwd:          cwd,
			HomeDir:      homeDir,
			TerminalCols: 80,
			TerminalRows: 24,
			PromptInArgs: false,
		}, nil
	default:
		// Test-registered factories (populated via init() in _test.go files).
		if factory, ok := testAdapterFactories[id]; ok {
			ad := factory(spec.Options)
			return ad.BuildSpawnSpec(cwd, env, homeDir, 80, 24, ""), nil
		}
		return adapter.SpawnSpec{}, fmt.Errorf("provider transport: unknown provider %q", id)
	}
}

// buildPeerEnv extracts the per-peer env map from PeerSpec.Options.
//
// Three sources are merged into the resulting map:
//  1. allowlisted env_KEY=value opts (filtered against allowedEnvKeys)
//  2. spec.Options["model"] -> VITIS_MODEL
//  3. spec.Options["reasoning-effort"] -> VITIS_REASONING_EFFORT
//
// Anything else (including unrecognised env_ keys) is dropped with a stderr
// warning so callers see what was filtered.
func buildPeerEnv(options map[string]string) map[string]string {
	env := map[string]string{}
	for k, v := range options {
		if strings.HasPrefix(k, "env_") {
			key := strings.TrimPrefix(k, "env_")
			if allowedEnvKeys[key] {
				env[key] = v
			} else {
				fmt.Fprintf(os.Stderr, "vitis: dropping disallowed env var %q from peer options\n", key)
			}
		}
	}
	if m := options["model"]; m != "" {
		env["VITIS_MODEL"] = m
	}
	if re := options["reasoning-effort"]; re != "" {
		env["VITIS_REASONING_EFFORT"] = re
	}
	return env
}

// resolveAdapter is retained for tests that exercise the URI -> adapter
// resolution path. Production code uses buildPersistentSpawnSpec instead.
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
