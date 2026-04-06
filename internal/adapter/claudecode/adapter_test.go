package claudecode

import (
	"testing"
)

func TestResolveCommandDefaults(t *testing.T) {
	t.Setenv("CLANK_CLAUDE_BINARY", "")
	t.Setenv("CLANK_CLAUDE_ARGS", "")

	command, args := ResolveCommand(map[string]string{})
	if command != "claude" {
		t.Fatalf("unexpected command: %q", command)
	}
	if len(args) != 0 {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestResolveCommandEnvOverride(t *testing.T) {
	command, args := ResolveCommand(map[string]string{
		"CLANK_CLAUDE_BINARY": "/tmp/mock-claude",
		"CLANK_CLAUDE_ARGS":   "serve --color=never",
	})
	if command != "/tmp/mock-claude" {
		t.Fatalf("unexpected command: %q", command)
	}
	if len(args) != 2 || args[0] != "serve" || args[1] != "--color=never" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestAdapterID(t *testing.T) {
	a := New()
	if a.ID() != "claude-code" {
		t.Fatalf("unexpected ID: %q", a.ID())
	}
}

func TestBuildSpawnSpec_Defaults(t *testing.T) {
	t.Setenv("CLANK_CLAUDE_BINARY", "")
	t.Setenv("CLANK_CLAUDE_ARGS", "")

	a := New()
	spec := a.BuildSpawnSpec("/work", map[string]string{}, "/home/user", 120, 40)

	if spec.Command != "claude" {
		t.Errorf("unexpected Command: %q", spec.Command)
	}
	if len(spec.Args) != 0 {
		t.Errorf("unexpected Args: %#v", spec.Args)
	}
	if spec.Cwd != "/work" {
		t.Errorf("unexpected Cwd: %q", spec.Cwd)
	}
	if spec.HomeDir != "/home/user" {
		t.Errorf("unexpected HomeDir: %q", spec.HomeDir)
	}
	if spec.TerminalCols != 120 {
		t.Errorf("unexpected TerminalCols: %d", spec.TerminalCols)
	}
	if spec.TerminalRows != 40 {
		t.Errorf("unexpected TerminalRows: %d", spec.TerminalRows)
	}
	if spec.Env == nil {
		t.Error("expected non-nil Env map")
	}
}

func TestFormatPrompt(t *testing.T) {
	a := New()

	cases := []struct {
		input string
		want  string
	}{
		{"hello", "hello\n"},
		{"hello\n", "hello\n"},
		{"hello\n\n", "hello\n"},
		{"", "\n"},
	}

	for _, tc := range cases {
		got := a.FormatPrompt(tc.input)
		if string(got) != tc.want {
			t.Errorf("FormatPrompt(%q) = %q, want %q", tc.input, got, tc.want)
		}
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

func TestValidateExecutable_SafePath(t *testing.T) {
	safe := []string{
		"/usr/local/bin/claude",
		"claude",
		"/tmp/mock-claude",
	}
	for _, s := range safe {
		if err := validateExecutable(s); err != nil {
			t.Errorf("validateExecutable(%q) unexpected error: %v", s, err)
		}
	}
}

func TestResolveCommand_UnsafeBinaryFallsBackToDefault(t *testing.T) {
	t.Setenv("CLANK_CLAUDE_BINARY", "")

	command, _ := ResolveCommand(map[string]string{
		"CLANK_CLAUDE_BINARY": "cmd;inject",
	})
	if command != "claude" {
		t.Fatalf("expected fallback to 'claude', got %q", command)
	}
}
