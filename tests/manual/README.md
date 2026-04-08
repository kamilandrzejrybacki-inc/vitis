# Vitis Manual Test Suite

Shell scripts that exercise Vitis end-to-end against the real PTY runtime, the bundled mock agent, and (optionally) real `claude` and `codex` CLIs. Each script is self-contained, sources `lib/common.sh` for shared helpers, builds the binaries it needs into `tests/manual/.build/`, and prints PASS / FAIL / WARN / VERIFY lines on stdout.

## Quick start

```bash
# Run everything available. Real-provider tests auto-skip when their
# binaries are missing, so this is safe on a stock dev box.
./tests/manual/run_all.sh

# Mock-only sweep, around 30 seconds total, no real LLM calls.
./tests/manual/run_all.sh --quick

# Skip real-provider tests even if claude/codex are on PATH.
./tests/manual/run_all.sh --no-real

# Run a single test by its number prefix.
./tests/manual/run_all.sh --only 05
```

Make every script executable once:

```bash
chmod +x tests/manual/*.sh
```

## What's covered

| #  | Script | Needs real provider? | Tests |
|----|---|---|---|
| 01 | `01_doctor.sh` | no | `vitis doctor` exits cleanly |
| 02 | `02_run_mock_happy.sh` | no | Single-shot `vitis run` happy path plus `peek` |
| 03 | `03_run_mock_modes.sh` | no | Observer status classification across every `MOCK_MODE` value (happy, blocked, auth, rate_limit, partial, crash, ansi) |
| 04 | `04_converse_mock_max_turns.sh` | no | A2A reaches `max_turns_hit` when no sentinel is emitted |
| 05 | `05_converse_mock_sentinel.sh` | no | Sentinel terminator early-exit, sentinel stripped from forwarded envelopes |
| 06 | `06_converse_asymmetric_seeds.sh` | no | `--seed-a` / `--seed-b` / `--opener` |
| 07 | `07_converse_validation_errors.sh` | no | Every CLI validation rejection (exit 2 plus stderr message) |
| 08 | `08_converse_real_claude.sh` | claude | A2A claude vs claude (real LLM calls) |
| 09 | `09_converse_real_codex.sh` | codex | A2A codex vs codex (real LLM calls; known fragile until Plan 2.5) |
| 10 | `10_converse_cross_provider.sh` | both | A2A claude vs codex, the canonical cross-provider demo |
| 11 | `11_logs_and_peek.sh` | no | File-store persistence shape, file permissions (0600), `peek` for both single-shot and conversation logs |
| 12 | `12_security_path_traversal.sh` | no | Path-traversal hardening (`--working-directory`, `--log-path`), `env_KEY` allowlist enforcement (LD_PRELOAD, VITIS_CLAUDE_ARGS dropped) |
| 13 | `13_converse_portkey.sh` | portkey | A2A end-to-end via [portkeyagent](https://github.com/kamilrybacki/portkeyagent) routed through the Portkey gateway. Auto-skips if portkeyagent or `PORTKEY_API_KEY` is missing. |
| 14 | `14_rtk_integration.sh` | rtk | Verifies [rtk](https://github.com/rtk-ai/rtk) is installed and the rtk PreToolUse hook is active for at least one of the spawned providers. Auto-skips if rtk is missing. |
| 15 | `15_converse_caveman.sh` | portkey | Runs the same conversation twice (`--style normal` then `--style caveman-ultra`) and asserts the caveman version is at least 10 percent shorter. Uses the [JuliusBrussee/caveman](https://github.com/JuliusBrussee/caveman) rules embedded in the per-peer briefing, no external skill install required. |

## Real-provider tests (08, 09, 10)

These exist to validate Vitis against actual `claude` and `codex` binaries. They cost real money (Anthropic or OpenAI quota), take minutes per turn, and auto-skip if the binary is missing via `require_claude_code` and `require_codex` in `lib/common.sh`.

Override the binary path without installing globally:

```bash
VITIS_CLAUDE_BINARY=/opt/claude-cli/claude ./tests/manual/08_converse_real_claude.sh
VITIS_CODEX_BINARY=/opt/codex-cli/codex   ./tests/manual/09_converse_real_codex.sh
```

Cap the cost on a real run:

```bash
VITIS_MANUAL_MAX_TURNS=2 ./tests/manual/08_converse_real_claude.sh
```

## Token efficiency via rtk

[rtk](https://github.com/rtk-ai/rtk) is a CLI proxy that compresses common shell command outputs (git, ls, cat, grep, test runners) by 60 to 90 percent before they reach the agent's context. Vitis itself does not execute commands. The agents Vitis spawns do, and in long A2A conversations where two agents may run dozens of tool calls, having rtk hooks active for both providers means smaller agent input contexts and smaller envelopes flowing through the broker.

`vitis doctor` reports rtk health for the queried provider:

```bash
vitis doctor --provider claude-code | jq .rtk
```

```json
{
  "available": true,
  "path": "/home/me/.local/bin/rtk",
  "version": "rtk 0.28.2",
  "hook_installed": true,
  "note": "rtk is active for this provider, shell commands the agent runs will be auto-compressed"
}
```

One-shot setup:

```bash
./tests/manual/setup_rtk.sh
```

This installs rtk via Homebrew, cargo, or the upstream curl-installer (whichever is available), runs `rtk init -g` for both Claude Code and Codex, patches `~/.claude/settings.json` with the rtk PreToolUse hook (using jq directly because `rtk init` refuses to patch non-interactively), and verifies the result through `vitis doctor`.

Verification:

```bash
./tests/manual/14_rtk_integration.sh
```

Auto-skips if rtk is not installed (run `setup_rtk.sh` first). Confirms rtk is on `PATH` and at least one provider has the rtk hook active. End-to-end token-savings verification (inspecting actual tool-call traffic) is left to a real-provider run via script 08 or 13.

## Reply-token compression via caveman style

The `--style` flag embeds the [JuliusBrussee/caveman](https://github.com/JuliusBrussee/caveman) ruleset into the per-peer briefing, so every reply the spawned agents produce comes back roughly 60 to 75 percent shorter without losing technical content. This stacks with rtk:

| Layer | What it shrinks | Mechanism |
|---|---|---|
| `rtk` | Tool call **input** going TO the agent | PreToolUse hook rewrites `git status` into `rtk git status` |
| `caveman` | Model **output** coming FROM the agent | Briefing instructions tell the model to drop filler, articles, hedging |

The combined effect on A2A is roughly two to four times more turns per fixed context budget: smaller envelopes plus smaller replies plus smaller tool outputs.

```bash
vitis converse --style caveman-full ...     # default caveman, drops articles and uses fragments
vitis converse --style caveman-lite ...     # professional but tight, keeps grammar
vitis converse --style caveman-ultra ...    # maximum compression, telegraphic, abbreviated
vitis converse --style normal ...           # default, no style instructions
```

The instructions are embedded in `internal/conversation/style.go`, adapted from caveman's MIT-licensed `SKILL.md`. No external install needed. Code blocks, error messages, security warnings, and irreversible-action confirmations stay verbatim per the canonical caveman rules, so the compression is safe for technical conversations.

Verification:

```bash
./tests/manual/15_converse_caveman.sh
```

Runs the same conversation twice (normal then caveman-ultra) against the homelab Portkey gateway and reports the compression ratio. On a real run with Groq llama-3.3-70b-versatile, this script measures roughly 60 percent reply-length reduction for the same prompt, well above the 10 percent threshold the script asserts. Auto-skips if portkeyagent or `PORTKEY_API_KEY` is missing.

## Free-LLM testing via Portkey

Script `13_converse_portkey.sh` runs a real A2A conversation through Vitis but routes the LLM calls via the [portkeyagent](https://github.com/kamilrybacki/portkeyagent) binary, which fronts the [Portkey](https://portkey.ai) gateway. This lets you exercise the full multi-turn marker-injection protocol against an actual model without spending Anthropic or OpenAI quota.

Setup once:

```bash
go install github.com/kamilrybacki/portkeyagent@latest
export PORTKEY_API_KEY=pk-xxxxx
export PORTKEY_VIRTUAL_KEY=openai-free-tier  # optional
export PORTKEY_MODEL=gpt-4o-mini             # optional
```

Then:

```bash
./tests/manual/13_converse_portkey.sh
```

The script auto-skips if `portkeyagent` is not on `PATH` or `PORTKEY_API_KEY` is unset, so it is safe to leave in `run_all.sh`.

### Known limitation: real codex multi-turn

The marker-injection approach in Plan 2 was tuned against the line-oriented mock agent. Real `codex` is a TUI app whose interactive REPL does not echo the marker token reliably. Tests 09 and 10 may hit `max_turns_hit` instead of `completed_sentinel` even when the model-side conversation is working fine. The fix is sidecar JSONL detection (Plan 2.5 in the design spec). Until that ships, treat 09 and 10 as smoke tests for "did the spawn shape work and did real bytes flow", not as functional pass/fail.

## How the mock-driven scripts work

The mock-driven tests (02 to 07, 11, 12) work without any real LLM by overriding `VITIS_CLAUDE_BINARY` to point at the bundled `internal/testutil/mockagent` binary. The mock agent supports two modes:

1. Single-shot mode (`MOCK_MODE=happy|blocked|auth|...`), used by `vitis run`
2. Multi-turn mode (`MOCK_MULTI_TURN=1`), used by `vitis converse`. The agent loops reading envelopes, extracts the per-turn marker token from the envelope text, optionally emits `<<END>>` on a configurable turn (`MOCK_SENTINEL_AT_TURN=N`), and prints `turn N: <response>\n<marker>\n`.

Both binaries are built on demand into `tests/manual/.build/` and reused across scripts.

## Helpers in `lib/common.sh`

Sourced by every script. Provides:

- Color output: `header`, `info`, `ok`, `warn`, `fail`, `verify`, `skip`
- Build helpers: `vitis_bin`, `mockagent_bin` (build on demand, idempotent)
- Provider gating: `have_claude_code`, `have_codex`, `require_claude_code`, `require_codex` (the latter two auto-`skip` the script)
- JSON helpers: `json_field <json> <path>`, `assert_status`, `assert_conv_status`, `assert_nonempty_response`, `print_json`
- Temp dirs: `setup_tmp_logs` registers an EXIT trap that cleans `${TEST_LOG_DIR}`

## When to run which script

| Situation | Run |
|---|---|
| Pre-commit / pre-push smoke | `run_all.sh --quick` |
| Before a release or PR merge | `run_all.sh --no-real` (full mock sweep) |
| After modifying observer or extractor logic | `03_run_mock_modes.sh` |
| After modifying broker or sentinel | `04`, `05`, `06` |
| After modifying CLI flags | `07` |
| After modifying file store | `11` |
| After modifying spawner or env handling | `12` |
| Before claiming "real claude works" | `08` (read the streamed turns) |
| Investigating a Plan 2.5 codex bug | `09`, `10` (capture transcripts for the design notes) |

## Adding a new manual test

Copy the structure of an existing script (`02_run_mock_happy.sh` is the simplest template), follow the naming pattern `NN_short_name.sh`, source `lib/common.sh`, and append the entry to the `TESTS` array in `run_all.sh`. If your new test exercises a real provider, add `require_claude_code` or `require_codex` near the top so it auto-skips on stock dev boxes, and tag it `real` in the `run_all.sh` catalog.

## Cleanup

```bash
rm -rf tests/manual/.build      # remove built binaries
```

Per-test temp dirs are cleaned automatically by the EXIT trap in `setup_tmp_logs`.
