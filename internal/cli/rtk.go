package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RTKStatus reports rtk (https://github.com/rtk-ai/rtk) availability for a
// specific provider. rtk is a CLI proxy that compresses common command
// outputs (git, ls, cat, grep, test runners, ...) before they reach the
// agent's context, reducing per-tool-call token consumption by 60-90%.
//
// In A2A conversations where two long-lived agents may run dozens of tool
// calls per conversation, having rtk hooks active for both providers
// translates directly into:
//   - more turns within each agent's context window
//   - smaller envelopes flowing through the broker
//   - smaller persisted conversation logs
//   - faster model inference per turn
//
// clank does not execute commands itself, so rtk has no role inside clank's
// own data path. The integration is purely at the agent layer: detect
// whether rtk is installed and whether each spawned provider's hook config
// references rtk, and recommend the install command if not.
type RTKStatus struct {
	Available          bool   `json:"available"`
	Path               string `json:"path,omitempty"`
	Version            string `json:"version,omitempty"`
	HookInstalled      bool   `json:"hook_installed"`
	HookInstallCommand string `json:"hook_install_command,omitempty"`
	Note               string `json:"note,omitempty"`
}

// DetectRTK probes the local environment for an rtk install and an
// active rtk hook for the named provider. The result is purely
// informational — clank never refuses to operate when rtk is missing.
func DetectRTK(provider string) RTKStatus {
	status := RTKStatus{}

	path, err := exec.LookPath("rtk")
	if err != nil {
		status.Note = "rtk binary not found on PATH. Install for 60-90% token savings on agent shell commands: https://github.com/rtk-ai/rtk#installation"
		return status
	}
	status.Available = true
	status.Path = path

	if out, verr := exec.Command(path, "--version").Output(); verr == nil {
		status.Version = strings.TrimSpace(string(out))
	}

	home, _ := os.UserHomeDir()
	status.HookInstalled = isRTKHookInstalled(provider, home)
	if status.HookInstalled {
		status.Note = "rtk is active for this provider — shell commands the agent runs will be auto-compressed"
		return status
	}

	switch provider {
	case "claude-code", "claudecode":
		status.HookInstallCommand = "rtk init -g"
	case "codex":
		status.HookInstallCommand = "rtk init -g --codex"
	default:
		status.HookInstallCommand = "rtk init -g"
	}
	status.Note = "rtk is installed but the hook is not active for this provider. Run: " + status.HookInstallCommand
	return status
}

// isRTKHookInstalled inspects the provider's settings/config/instructions
// files for any reference to the rtk rewrite hook. The detection is
// intentionally substring-based rather than parsing settings.json or
// config.toml: rtk's integration takes a different shape per provider —
//
//   - Claude Code: a shell-script PreToolUse hook entry in
//     ~/.claude/settings.json (or settings.local.json) referencing
//     ~/.claude/hooks/rtk-rewrite.sh
//
//   - Codex: an instructions-based integration that writes a reference to
//     ~/.codex/RTK.md from ~/.codex/AGENTS.md (codex doesn't yet have a
//     PreToolUse hook system, so the integration nudges the model via
//     prompt instructions to prefix shell calls with `rtk`)
//
// The canonical marker in any of these forms is the literal substring
// "rtk-rewrite", "rtk rewrite", or "RTK.md".
func isRTKHookInstalled(provider, home string) bool {
	if home == "" {
		return false
	}
	var paths []string
	switch provider {
	case "claude-code", "claudecode":
		paths = []string{
			filepath.Join(home, ".claude", "settings.json"),
			filepath.Join(home, ".claude", "settings.local.json"),
		}
	case "codex":
		paths = []string{
			filepath.Join(home, ".codex", "config.toml"),
			filepath.Join(home, ".codex", "settings.json"),
			filepath.Join(home, ".codex", "AGENTS.md"),
		}
	default:
		return false
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := string(data)
		if strings.Contains(s, "rtk-rewrite") ||
			strings.Contains(s, "rtk rewrite") ||
			strings.Contains(s, "RTK.md") {
			return true
		}
	}
	return false
}
