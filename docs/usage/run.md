# Single-shot runs

`vitis run` sends one prompt to an agent, captures the full response, and writes a JSON report. The agent process spawns, runs to completion, and exits. There is no second turn and no persistent state between runs.

Use `vitis run` for one-off questions, batch jobs, scriptable LLM calls, and anywhere you would normally invoke `claude --print` or `codex exec` directly. If you want a long-lived back-and-forth between two agents, use [`vitis converse`](converse.md) instead.

## Basic usage

```bash
vitis run --provider claude-code --prompt "what is 2+2?"
```

The output is a JSON object with the session id, captured response, persisted log paths, exit code, and a `meta` block with timing and confidence information. Diagnostics go to stderr; the JSON report is the only thing on stdout.

## Reading a prompt from a file

```bash
vitis run \
  --provider claude-code \
  --prompt-file ./long-prompt.txt \
  --log-path ./run-logs
```

`--prompt` and `--prompt-file` are mutually exclusive. Pass exactly one.

## Choosing the model and reasoning effort

```bash
vitis run \
  --provider claude-code \
  --prompt "explain context.Context" \
  --model claude-sonnet-4-6 \
  --reasoning-effort high
```

`--model` and `--reasoning-effort` are forwarded to the spawned binary. Whether they take effect depends on the provider; check `vitis doctor` to see which provider is being driven.

## Setting a timeout

```bash
vitis run \
  --provider claude-code \
  --prompt "ping" \
  --timeout 30
```

`--timeout` is the wall-clock budget in seconds. If the agent does not produce a complete response in that window, the run finalizes with `status: "timeout"` and the JSON report still includes whatever partial output was captured.

## Where the session is stored

By default Vitis writes sessions to `./logs/`. Override with `--log-path`:

```bash
vitis run \
  --provider claude-code \
  --prompt "hello" \
  --log-path /var/lib/vitis/runs
```

The directory layout is:

```
<log-path>/
├── sessions/<session_id>.json    # session summary
├── turns/<session_id>.jsonl      # one JSON object per turn
└── raw/<session_id>.jsonl        # raw PTY events (only with --debug-raw)
```

All files are written with `0600` permissions.

## Reading a session back

```bash
vitis peek --session-id sess_abc123 --log-path ./logs --last 10
```

The peek output is a JSON object with the session id and an array of turns. Use `--last N` to limit the number of turns returned, or omit it to get the full conversation.

## Run statuses

The `status` field in the JSON report tells you what happened. The full set of values:

| Status | Meaning |
|---|---|
| `completed` | Agent finished and Vitis captured a non-empty response |
| `partial` | Agent exited but extraction confidence is low |
| `blocked_on_input` | Agent is sitting at an interactive prompt waiting for confirmation |
| `auth_required` | Agent reports it needs authentication |
| `rate_limited` | Agent reports it has hit a rate limit |
| `timeout` | The `--timeout` budget elapsed before the agent finished |
| `crashed` | Agent process exited with a non-zero code and no recognisable output |
| `error` | Internal Vitis error (spawn failure, IO error, etc.) |

Vitis does not auto-answer permission, auth, or rate-limit prompts. If you see one of those non-terminal statuses, the run reports it honestly and exits.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | The run reached a terminal status (`completed`, `partial`, `timeout`, etc.) and a JSON report was written |
| 1 | Runtime error (spawn failure, store init failure, IO error) |
| 2 | Configuration error (bad flags, missing required arguments) |

The JSON report is always written to stdout regardless of exit code.
