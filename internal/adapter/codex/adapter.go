package codex

import (
	"fmt"
	"os"
	"strings"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) ID() string { return "codex" }

func (a *Adapter) BuildSpawnSpec(cwd string, env map[string]string, homeDir string, cols, rows int, prompt string) adapter.SpawnSpec {
	command, args := ResolveCommand(env)
	// Insert --model and --reasoning-effort before the prompt positional arg.
	if m := env["CLANK_MODEL"]; m != "" {
		args = append(args, "--model", m)
	}
	if re := env["CLANK_REASONING_EFFORT"]; re != "" {
		args = append(args, "--reasoning-effort", re)
	}
	// Append the prompt as the last positional argument.
	args = append(args, prompt)
	return adapter.SpawnSpec{
		Command:      command,
		Args:         args,
		Env:          env,
		Cwd:          cwd,
		HomeDir:      homeDir,
		TerminalCols: cols,
		TerminalRows: rows,
		PromptInArgs: true, // Codex reads prompt from args, not stdin
	}
}

func (a *Adapter) FormatPrompt(raw string) []byte {
	// Not used when PromptInArgs=true, but implement for interface compliance
	return []byte(raw + "\n")
}

// ResolveCommand resolves the codex binary and arguments from the environment.
func ResolveCommand(env map[string]string) (string, []string) {
	const defaultBinary = "codex"
	defaultArgs := []string{"exec", "--full-auto", "--skip-git-repo-check"}

	binaryOverride := firstNonEmpty(env["CLANK_CODEX_BINARY"], os.Getenv("CLANK_CODEX_BINARY"))
	command := defaultBinary
	if binaryOverride != "" {
		if err := validateExecutable(binaryOverride); err != nil {
			fmt.Fprintf(os.Stderr, "clank: ignoring CLANK_CODEX_BINARY override: %v\n", err)
		} else {
			command = binaryOverride
		}
	}

	argsStr := firstNonEmpty(env["CLANK_CODEX_ARGS"], os.Getenv("CLANK_CODEX_ARGS"))
	if argsStr != "" {
		var args []string
		for _, arg := range strings.Fields(argsStr) {
			if arg == "" {
				continue
			}
			args = append(args, arg)
		}
		return command, args
	}

	return command, defaultArgs
}

// validateExecutable rejects strings containing shell metacharacters that could
// enable command injection if the value were passed unsanitised to a shell.
func validateExecutable(s string) error {
	for _, c := range []string{";", "&", "|", "$", "`", "(", ")", "\n"} {
		if strings.Contains(s, c) {
			return fmt.Errorf("CLANK_CODEX_BINARY contains unsafe character %q", c)
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
