package cli

// This file registers the mock provider adapter for CLI E2E tests so that
// provider:mock URIs resolve correctly in test binaries without being
// reachable in production binaries.

import (
	"os"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/peer/provider"
)

func init() {
	provider.RegisterTestAdapterFactory("mock", newCliMockAdapter)
}

type cliMockProviderAdapter struct {
	bin string
}

func newCliMockAdapter(opts map[string]string) adapter.Adapter {
	bin := opts["bin"]
	if bin == "" {
		bin = os.Getenv("MOCK_BIN")
	}
	return &cliMockProviderAdapter{bin: bin}
}

func (m *cliMockProviderAdapter) ID() string { return "mock" }

func (m *cliMockProviderAdapter) BuildSpawnSpec(cwd string, env map[string]string, homeDir string, cols, rows int, _ string) adapter.SpawnSpec {
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

func (m *cliMockProviderAdapter) FormatPrompt(raw string) []byte { return []byte(raw + "\n") }

func (m *cliMockProviderAdapter) Observe(_ adapter.CompletionContext) *adapter.TranscriptObservation {
	return nil
}

func (m *cliMockProviderAdapter) ExtractResponse(_ []byte, _ string) adapter.ExtractionResult {
	return adapter.ExtractionResult{}
}
