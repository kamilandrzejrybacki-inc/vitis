#!/usr/bin/env bash
# setup_rtk.sh — install rtk and configure its hooks for the providers clank
# spawns. Idempotent: safe to re-run; skips steps that are already done.
#
# What it does:
#   1. Verifies rtk is on PATH; offers to install via cargo or homebrew if not
#   2. Runs `rtk init -g` for Claude Code (writes the rtk PreToolUse hook to
#      ~/.claude/settings.json)
#   3. Runs `rtk init -g --codex` for Codex (writes the equivalent hook to
#      ~/.codex/...)
#   4. Verifies via `clank doctor` that the rtk hook is detected as active
#      for both providers
#
# Why: clank drives interactive AI agent CLIs through a PTY. The agents do
# the actual tool calls (Bash, Read, Grep, ...), and rtk compresses those
# command outputs by 60-90% before they reach the agent's context. In A2A
# conversations where two agents may run dozens of tool calls per
# conversation, this is an outsized win.
#
# Run: tests/manual/setup_rtk.sh
set -euo pipefail
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/lib/common.sh"

header "setup_rtk: install + configure rtk for clank-spawned agents"

# ---- Step 1: install rtk if missing ----------------------------------------

if command -v rtk >/dev/null 2>&1; then
  ok "rtk already on PATH: $(command -v rtk) ($(rtk --version 2>/dev/null | head -1))"
else
  warn "rtk binary not found"
  if command -v brew >/dev/null 2>&1; then
    info "installing via Homebrew: brew install rtk"
    brew install rtk
  elif command -v cargo >/dev/null 2>&1; then
    info "installing via cargo: cargo install --git https://github.com/rtk-ai/rtk"
    cargo install --git https://github.com/rtk-ai/rtk
  elif command -v curl >/dev/null 2>&1; then
    info "installing via the upstream curl-installer (writes to ~/.local/bin)"
    curl -fsSL https://raw.githubusercontent.com/rtk-ai/rtk/refs/heads/master/install.sh | sh
  else
    fail "no installer available — install rtk manually from https://github.com/rtk-ai/rtk#installation, then re-run this script"
  fi

  # The curl installer writes to ~/.local/bin which may not be on PATH yet.
  if ! command -v rtk >/dev/null 2>&1; then
    if [[ -x "${HOME}/.local/bin/rtk" ]]; then
      warn "rtk installed at ${HOME}/.local/bin/rtk but PATH does not include ~/.local/bin"
      warn "exporting PATH for this script only — add 'export PATH=\$HOME/.local/bin:\$PATH' to ~/.bashrc or ~/.zshrc to make it permanent"
      export PATH="${HOME}/.local/bin:${PATH}"
    fi
  fi

  if ! command -v rtk >/dev/null 2>&1; then
    fail "rtk install completed but the binary is still not on PATH"
  fi
  ok "rtk installed: $(command -v rtk) ($(rtk --version 2>/dev/null | head -1))"
fi

# ---- Step 2: install hooks for Claude Code ---------------------------------

info "installing rtk hook script for Claude Code (rtk init -g)"
# rtk init -g writes the rtk-rewrite.sh hook script and RTK.md, but it
# refuses to patch ~/.claude/settings.json non-interactively (it uses
# isatty(stdin), so `yes |` doesn't help). We run it to install the
# script + RTK.md, then patch settings.json ourselves with jq below.
if ! rtk init -g 2>&1; then
  warn "rtk init -g exited non-zero — continuing anyway"
fi

CLAUDE_SETTINGS="${HOME}/.claude/settings.json"
RTK_HOOK_SCRIPT="${HOME}/.claude/hooks/rtk-rewrite.sh"

if [[ ! -x "${RTK_HOOK_SCRIPT}" ]]; then
  warn "rtk init -g did not produce ${RTK_HOOK_SCRIPT}; skipping settings patch"
elif ! command -v jq >/dev/null 2>&1; then
  warn "jq not installed; cannot auto-patch ${CLAUDE_SETTINGS} — install jq or add the hook manually"
elif grep -qF "rtk-rewrite.sh" "${CLAUDE_SETTINGS}" 2>/dev/null; then
  ok "Claude Code settings.json already references rtk-rewrite.sh"
else
  info "patching ${CLAUDE_SETTINGS} to add the rtk PreToolUse hook"
  cp "${CLAUDE_SETTINGS}" "${CLAUDE_SETTINGS}.bak.$(date +%s)" 2>/dev/null || true
  # Build the rtk hook entry, then merge it into .hooks.PreToolUse,
  # creating intermediate keys/arrays if missing. This preserves any
  # existing PreToolUse entries (e.g. jcodemunch-mcp).
  tmp="$(mktemp)"
  if [[ -f "${CLAUDE_SETTINGS}" ]]; then
    src="${CLAUDE_SETTINGS}"
  else
    echo '{}' > "${tmp}.empty"
    src="${tmp}.empty"
  fi
  jq --arg cmd "${RTK_HOOK_SCRIPT}" '
    .hooks //= {}
    | .hooks.PreToolUse //= []
    | .hooks.PreToolUse += [{
        "matcher": "Bash",
        "hooks": [{"type":"command","command":$cmd}]
      }]
  ' "${src}" > "${tmp}"
  mv "${tmp}" "${CLAUDE_SETTINGS}"
  chmod 600 "${CLAUDE_SETTINGS}"
  rm -f "${tmp}.empty"
  ok "patched ${CLAUDE_SETTINGS} with rtk PreToolUse hook"
fi

# ---- Step 3: install hooks for Codex ---------------------------------------

info "installing rtk integration for Codex (rtk init -g --codex)"
# Codex uses an instructions-based integration (writes ~/.codex/AGENTS.md
# referencing ~/.codex/RTK.md) rather than a runtime hook. No interactive
# prompt to bypass.
if rtk init -g --codex 2>&1; then
  ok "rtk init -g --codex completed"
else
  warn "rtk init -g --codex failed — Codex hook support may differ in your rtk version"
fi

# ---- Step 4: verify via clank doctor ---------------------------------------

CLANK="$(clank_bin)"

verify_provider() {
  local provider="$1"
  printf '\n%s---%s clank doctor --provider %s\n' "$C_DIM" "$C_RESET" "${provider}"
  out=$( "${CLANK}" doctor --provider "${provider}" 2>&1 )
  echo "${out}"

  hook_installed=$(echo "${out}" | python3 -c '
import json, sys
data = json.loads(sys.stdin.read())
rtk = data.get("rtk", {})
print("yes" if rtk.get("hook_installed") else "no")
' 2>/dev/null || echo "?")

  if [[ "${hook_installed}" == "yes" ]]; then
    ok "${provider}: rtk hook detected as installed"
  else
    warn "${provider}: rtk hook NOT detected — check ~/.${provider#claude-}/settings.json by hand"
  fi
}

verify_provider "claude-code"
verify_provider "codex"

printf '\n%sdone%s — rtk should now be active for any agent clank spawns.\n' "$C_GREEN$C_BOLD" "$C_RESET"
verify "next: run ./tests/manual/14_rtk_integration.sh to confirm end-to-end"
