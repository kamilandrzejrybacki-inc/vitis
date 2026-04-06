package codex

import (
	"testing"
)

func TestAdapterID(t *testing.T) {
	a := New()
	if a.ID() != "codex" {
		t.Fatalf("expected ID 'codex', got %q", a.ID())
	}
}

func TestBuildSpawnSpec_Defaults(t *testing.T) {
	t.Setenv("CLANK_CODEX_BINARY", "")
	t.Setenv("CLANK_CODEX_ARGS", "")

	a := New()
	spec := a.BuildSpawnSpec("/tmp", map[string]string{}, "/home/user", 80, 24, "test prompt")

	if spec.Command != "codex" {
		t.Errorf("unexpected Command: %q", spec.Command)
	}
	// Default args: ["exec", "--full-auto", "test prompt"]
	if len(spec.Args) != 3 {
		t.Fatalf("expected 3 args, got %d: %#v", len(spec.Args), spec.Args)
	}
	if spec.Args[0] != "exec" {
		t.Errorf("expected args[0]='exec', got %q", spec.Args[0])
	}
	if spec.Args[1] != "--full-auto" {
		t.Errorf("expected args[1]='--full-auto', got %q", spec.Args[1])
	}
	if spec.Args[2] != "test prompt" {
		t.Errorf("expected args[2]='test prompt', got %q", spec.Args[2])
	}
	if !spec.PromptInArgs {
		t.Error("expected PromptInArgs=true")
	}
	if spec.Cwd != "/tmp" {
		t.Errorf("unexpected Cwd: %q", spec.Cwd)
	}
	if spec.HomeDir != "/home/user" {
		t.Errorf("unexpected HomeDir: %q", spec.HomeDir)
	}
	if spec.TerminalCols != 80 {
		t.Errorf("unexpected TerminalCols: %d", spec.TerminalCols)
	}
	if spec.TerminalRows != 24 {
		t.Errorf("unexpected TerminalRows: %d", spec.TerminalRows)
	}
}

func TestBuildSpawnSpec_EnvOverride(t *testing.T) {
	t.Setenv("CLANK_CODEX_BINARY", "")
	t.Setenv("CLANK_CODEX_ARGS", "")

	a := New()
	env := map[string]string{"CLANK_CODEX_BINARY": "/usr/local/bin/codex"}
	spec := a.BuildSpawnSpec("/tmp", env, "/home/user", 80, 24, "test")

	if spec.Command != "/usr/local/bin/codex" {
		t.Errorf("expected command '/usr/local/bin/codex', got %q", spec.Command)
	}
}

func TestBuildSpawnSpec_ArgsOverride(t *testing.T) {
	t.Setenv("CLANK_CODEX_BINARY", "")
	t.Setenv("CLANK_CODEX_ARGS", "")

	a := New()
	env := map[string]string{"CLANK_CODEX_ARGS": "run --verbose"}
	spec := a.BuildSpawnSpec("/tmp", env, "/home/user", 80, 24, "my prompt")

	// When CLANK_CODEX_ARGS overrides, prompt is still appended as last arg.
	if len(spec.Args) < 1 {
		t.Fatalf("expected at least one arg, got %d: %#v", len(spec.Args), spec.Args)
	}
	if spec.Args[0] != "run" {
		t.Errorf("expected args[0]='run', got %q", spec.Args[0])
	}
	// Prompt is appended after the override args.
	last := spec.Args[len(spec.Args)-1]
	if last != "my prompt" {
		t.Errorf("expected last arg='my prompt', got %q", last)
	}
}

func TestFormatPrompt(t *testing.T) {
	a := New()
	result := a.FormatPrompt("hello")
	if string(result) != "hello\n" {
		t.Errorf("FormatPrompt(%q) = %q, want %q", "hello", string(result), "hello\n")
	}
}

func TestFormatPrompt_EmptyString(t *testing.T) {
	a := New()
	result := a.FormatPrompt("")
	if string(result) != "\n" {
		t.Errorf("FormatPrompt(%q) = %q, want %q", "", string(result), "\n")
	}
}

func TestResolveCommand_Defaults(t *testing.T) {
	t.Setenv("CLANK_CODEX_BINARY", "")
	t.Setenv("CLANK_CODEX_ARGS", "")

	command, args := ResolveCommand(map[string]string{})
	if command != "codex" {
		t.Fatalf("unexpected command: %q", command)
	}
	if len(args) != 2 || args[0] != "exec" || args[1] != "--full-auto" {
		t.Fatalf("unexpected default args: %#v", args)
	}
}

func TestResolveCommand_EnvOverride(t *testing.T) {
	t.Setenv("CLANK_CODEX_BINARY", "")
	t.Setenv("CLANK_CODEX_ARGS", "")

	command, args := ResolveCommand(map[string]string{
		"CLANK_CODEX_BINARY": "/tmp/mock-codex",
		"CLANK_CODEX_ARGS":   "run --model o4-mini",
	})
	if command != "/tmp/mock-codex" {
		t.Fatalf("unexpected command: %q", command)
	}
	if len(args) != 3 || args[0] != "run" || args[1] != "--model" || args[2] != "o4-mini" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestResolveCommand_UnsafeBinaryFallsBackToDefault(t *testing.T) {
	t.Setenv("CLANK_CODEX_BINARY", "")

	command, _ := ResolveCommand(map[string]string{
		"CLANK_CODEX_BINARY": "cmd;inject",
	})
	if command != "codex" {
		t.Fatalf("expected fallback to 'codex', got %q", command)
	}
}

func TestValidateExecutable_UnsafeChars(t *testing.T) {
	unsafe := []string{
		"cmd;bad",
		"cmd&bad",
		"cmd|bad",
		"cmd$bad",
		"cmd`bad",
		"cmd(bad",
		"cmd)bad",
		"cmd\nbad",
	}
	for _, s := range unsafe {
		if err := validateExecutable(s); err == nil {
			t.Errorf("validateExecutable(%q) expected error, got nil", s)
		}
	}
}

func TestValidateExecutable_SafePaths(t *testing.T) {
	safe := []string{
		"/usr/local/bin/codex",
		"codex",
		"/tmp/mock-codex",
	}
	for _, s := range safe {
		if err := validateExecutable(s); err != nil {
			t.Errorf("validateExecutable(%q) unexpected error: %v", s, err)
		}
	}
}
