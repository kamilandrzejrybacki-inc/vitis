package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRTKHookInstalledClaudeCode(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))

	// No settings file → not installed.
	require.False(t, isRTKHookInstalled("claude-code", home))

	// settings.json without rtk → not installed.
	require.NoError(t, os.WriteFile(
		filepath.Join(home, ".claude", "settings.json"),
		[]byte(`{"hooks":{"PreToolUse":[{"matcher":"Read","hooks":[{"type":"command","command":"some-other-tool"}]}]}}`),
		0o600,
	))
	require.False(t, isRTKHookInstalled("claude-code", home))

	// settings.json WITH rtk-rewrite → installed.
	require.NoError(t, os.WriteFile(
		filepath.Join(home, ".claude", "settings.json"),
		[]byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"~/.claude/hooks/rtk-rewrite.sh"}]}]}}`),
		0o600,
	))
	require.True(t, isRTKHookInstalled("claude-code", home))
}

func TestIsRTKHookInstalledClaudeCodeLocalSettings(t *testing.T) {
	// rtk init may write to settings.local.json instead of settings.json.
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(home, ".claude", "settings.local.json"),
		[]byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"rtk rewrite"}]}]}}`),
		0o600,
	))
	require.True(t, isRTKHookInstalled("claude-code", home))
}

func TestIsRTKHookInstalledCodexConfigToml(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex"), 0o700))

	require.False(t, isRTKHookInstalled("codex", home))

	require.NoError(t, os.WriteFile(
		filepath.Join(home, ".codex", "config.toml"),
		[]byte("[hooks]\npre_tool = \"rtk-rewrite\"\n"),
		0o600,
	))
	require.True(t, isRTKHookInstalled("codex", home))
}

func TestIsRTKHookInstalledCodexAgentsMd(t *testing.T) {
	// rtk init -g --codex writes ~/.codex/AGENTS.md referencing RTK.md
	// rather than installing a runtime hook (codex has no PreToolUse system).
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex"), 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(home, ".codex", "AGENTS.md"),
		[]byte("# Codex Global Instructions\n\n@/home/me/.codex/RTK.md\n"),
		0o600,
	))
	require.True(t, isRTKHookInstalled("codex", home))
}

func TestIsRTKHookInstalledUnknownProvider(t *testing.T) {
	require.False(t, isRTKHookInstalled("not-a-real-provider", t.TempDir()))
}

func TestIsRTKHookInstalledNoHomeDir(t *testing.T) {
	require.False(t, isRTKHookInstalled("claude-code", ""))
}

func TestDetectRTKBinaryMissing(t *testing.T) {
	// Force PATH to a directory with no rtk binary so the lookup fails
	// regardless of the host machine's rtk install state.
	tmpPath := t.TempDir()
	t.Setenv("PATH", tmpPath)

	got := DetectRTK("claude-code")
	require.False(t, got.Available)
	require.Empty(t, got.Path)
	require.Empty(t, got.Version)
	require.False(t, got.HookInstalled)
	require.Contains(t, got.Note, "not found on PATH")
}
