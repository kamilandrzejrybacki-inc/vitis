package claudecode

import (
	"os"
	"strings"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
)

type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) ID() string {
	return "claude-code"
}

func (a *Adapter) BuildSpawnSpec(cwd string, env map[string]string, homeDir string, cols, rows int) adapter.SpawnSpec {
	command, args := ResolveCommand(env)
	return adapter.SpawnSpec{
		Command:      command,
		Args:         args,
		Env:          env,
		Cwd:          cwd,
		HomeDir:      homeDir,
		TerminalCols: cols,
		TerminalRows: rows,
	}
}

func (a *Adapter) FormatPrompt(raw string) []byte {
	return []byte(strings.TrimRight(raw, "\n") + "\n")
}

func ResolveCommand(env map[string]string) (string, []string) {
	command := firstNonEmpty(env["CLANK_CLAUDE_BINARY"], os.Getenv("CLANK_CLAUDE_BINARY"), "claude")
	args := strings.Fields(firstNonEmpty(env["CLANK_CLAUDE_ARGS"], os.Getenv("CLANK_CLAUDE_ARGS"), ""))
	return command, args
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
