package claudecode

import "testing"

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
