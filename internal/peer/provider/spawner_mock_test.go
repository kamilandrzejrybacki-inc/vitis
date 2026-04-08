package provider

import (
	"os"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter"
)

func init() {
	testAdapterFactories["mock"] = newMockAdapter
}

// mockProviderAdapter is the test-only adapter that runs the mock-agent
// binary identified by spec.Options["bin"] (or MOCK_BIN env var). It is
// compiled only into test binaries via the _test.go suffix, so it cannot
// be reached from a production vitis binary.
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
