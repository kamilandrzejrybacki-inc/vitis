# Troubleshooting

## "provider_available": false

`vitis doctor` could not find the provider binary on `PATH`. Either install Claude Code or Codex, or set the binary override env var:

```bash
export VITIS_CLAUDE_BINARY=/opt/claude/bin/claude
export VITIS_CODEX_BINARY=/opt/codex/bin/codex
vitis doctor --provider claude-code | jq .
```

## A run finishes with `status: "blocked_on_input"`

The agent is sitting at an interactive prompt waiting for user confirmation. Vitis intentionally does not auto-answer permission, auth, or rate-limit prompts. Look at the persisted session log to see what triggered it:

```bash
vitis peek --session-id <session_id> --last 5
```

If the prompt is something the agent should not have asked at all (for example, asking for permission to run a command that should be auto-approved), update the agent's own settings or run it with the relevant `--dangerously-...` flag yourself.

## A conversation hits `max_turns_hit` instead of `completed_sentinel`

The peers ran the full turn budget without either side emitting the sentinel token. Check the streamed turns to see whether the model actually understood the instruction. The seed must explicitly tell each peer how to end:

```
End your reply with <<END>> when you believe the conversation has reached its goal.
```

If you are routing through `portkeyagent`, also check `PORTKEYAGENT_SYSTEM` is not telling the model the opposite.

## Real codex multi-turn does not work

Known limitation. The marker-injection protocol Vitis uses for turn detection was tuned against the line-oriented mock agent. Real `codex` is a TUI app whose interactive REPL does not echo the marker token reliably, so conversations through it tend to time out or hit max-turns. The fix is sidecar JSONL detection (Plan 2.5 in the design notes). Until that ships, treat real `provider:codex` as a smoke test for spawn shape and byte flow, not as a functional A2A target.

## A run logs `peer_blocked` with `auth_required`

The spawned agent is reporting it needs authentication. Run the agent yourself once with no Vitis wrapper to complete the auth flow:

```bash
claude
# follow the auth prompts
```

Then re-run Vitis. The cached credentials in `~/.claude/` (or the codex equivalent) should let subsequent runs proceed without prompting.

## "buffer overflow" mid-conversation

A peer wrote more than 64 MiB of output between turn markers. This usually means the agent went into a runaway loop or got stuck rendering a very large file. Cut the input size or set a tighter `--per-turn-timeout`.

## "context cancelled" or `interrupted`

The wall-clock `--overall-timeout` budget elapsed, or the user pressed Ctrl-C. The conversation finalises with `status: "interrupted"` and the `FinalResult` JSON includes whatever turns were captured before the cancel. Persisted logs are written normally.

## rtk hook is installed but not compressing anything

Run `rtk init -g` once interactively first. The non-interactive setup helper installs the hook script but `rtk init -g` itself refuses to patch `~/.claude/settings.json` without an interactive confirmation. The bundled `tests/manual/setup_rtk.sh` works around this by patching settings.json directly with jq. If you ran the helper and the hook still isn't active, re-run it and check the output.

## A peer process won't exit cleanly on Ctrl-C

`vitis converse` calls `peer.Stop` with a 1-second grace period in its deferred cleanup. Stop sends SIGINT to the process group, waits, then escalates to SIGKILL. If you still see lingering processes, list them with `ps -ef | grep claude` and kill manually. This usually means the agent is buffering output that takes longer than the grace period to flush.

## The published doc site is out of date

The docs workflow runs only on pushes to `main` that touch `docs/**`, `mkdocs.yml`, `README.md`, or the workflow file itself. If the workflow ran but the site did not update, check the workflow logs at the [Actions tab](https://github.com/kamilandrzejrybacki-inc/vitis/actions/workflows/docs.yml) for build errors. The most common cause is a markdown file with broken pymdownx code-fence attributes; rerun the workflow after fixing.

## Where to look when the JSON output looks wrong

| Field | What it tells you |
|---|---|
| `status` (run) or `conversation.status` (converse) | The terminal state of the run or conversation |
| `meta.warnings` | Anything Vitis collected as a non-fatal warning, including marker-missing notes and store failures |
| `meta.parser_confidence` | How confident the extractor is that the captured response is correct |
| `meta.observation_confidence` | How confident the observer is in the run status it assigned |
| `error` (when present) | Structured `code` + `message` for fatal errors |
| `peek[]` | Last N turns from the persisted session for quick inspection |

If `parser_confidence` or `observation_confidence` is below `0.8`, the extracted response is suspect and you should look at the raw transcript with `--debug-raw` enabled.
