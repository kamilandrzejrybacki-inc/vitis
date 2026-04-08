package claudecode

import (
	"fmt"
	"os"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter"
)

type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) ID() string {
	return "claude-code"
}

func (a *Adapter) BuildSpawnSpec(cwd string, env map[string]string, homeDir string, cols, rows int, _ string) adapter.SpawnSpec {
	command, args := ResolveCommand(env)
	if m := env["VITIS_MODEL"]; m != "" {
		args = append(args, "--model", m)
	}
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
	const defaultBinary = "claude"

	binaryOverride := firstNonEmpty(env["VITIS_CLAUDE_BINARY"], os.Getenv("VITIS_CLAUDE_BINARY"))
	command := defaultBinary
	if binaryOverride != "" {
		if err := validateExecutable(binaryOverride); err != nil {
			fmt.Fprintf(os.Stderr, "vitis: ignoring VITIS_CLAUDE_BINARY override: %v\n", err)
		} else {
			command = binaryOverride
		}
	}

	argsStr := firstNonEmpty(env["VITIS_CLAUDE_ARGS"], os.Getenv("VITIS_CLAUDE_ARGS"))
	var args []string
	for _, arg := range strings.Fields(argsStr) {
		if arg == "" {
			continue
		}
		args = append(args, arg)
	}

	return command, args
}

// validateExecutable rejects strings containing shell metacharacters that could
// enable command injection if the value were passed unsanitised to a shell.
func validateExecutable(s string) error {
	for _, c := range []string{";", "&", "|", "$", "`", "(", ")", "\n"} {
		if strings.Contains(s, c) {
			return fmt.Errorf("VITIS_CLAUDE_BINARY contains unsafe character %q", c)
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
