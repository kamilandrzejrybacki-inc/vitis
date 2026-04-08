# Configuration reference

Every flag, env var, and option that Vitis accepts.

## Global flags

These work for any subcommand.

| Flag | Default | Notes |
|---|---|---|
| `--log-backend` | `file` | The persistence backend. Postgres lands in Plan 3. |
| `--log-path` | `./logs` | Root directory for the file backend. Path-cleaned at startup. |

## `vitis run` flags

| Flag | Default | Notes |
|---|---|---|
| `--provider` | `claude-code` | Provider id: `claude-code` or `codex` |
| `--prompt` | required | Inline prompt. Mutually exclusive with `--prompt-file`. |
| `--prompt-file` | required | Path to a file containing the prompt. Mutually exclusive with `--prompt`. |
| `--timeout` | `600` | Wall-clock budget in seconds |
| `--peek-last` | `10` | Number of turns to include in the JSON report's `peek` field |
| `--working-directory` | cwd | Working directory for the spawned agent |
| `--env-file` | none | Path to a `.env` file loaded into the spawned process's environment |
| `--debug-raw` | false | Persist raw PTY events to `<log-path>/raw/<id>.jsonl` |
| `--terminal-cols` | `80` | PTY width in columns |
| `--terminal-rows` | `24` | PTY height in rows |
| `--home-dir` | `$HOME` | Override the spawned process's `HOME` |
| `--model` | from provider config | Model id forwarded to the spawned binary |
| `--reasoning-effort` | from provider config | Reasoning level (codex only) |

## `vitis converse` flags

| Flag | Default | Notes |
|---|---|---|
| `--peer-a`, `--peer-b` | required | Peer URI: `provider:claude-code`, `provider:codex`, or `provider:mock` (test builds only) |
| `--peer-a-opt key=value` | repeatable | Per-peer option for peer A |
| `--peer-b-opt key=value` | repeatable | Per-peer option for peer B |
| `--seed` | one of `--seed` or `--seed-a`+`--seed-b` is required | Same opening prompt for both peers |
| `--seed-a`, `--seed-b` | alternative to `--seed` | Asymmetric per-peer seeds |
| `--opener` | `a` | Which peer speaks first (`a` or `b`) |
| `--max-turns` | `50` | Hard cap, range 1 to 500 |
| `--per-turn-timeout` | `300` | Per-turn timeout in seconds, max 3600 |
| `--overall-timeout` | `max-turns × per-turn-timeout` | Whole-conversation budget in seconds, max 86400 |
| `--terminator` | `sentinel` | Termination strategy. `judge` lands in Plan 3. |
| `--sentinel` | `<<END>>` | Token a peer must emit on its own line to end the conversation cooperatively |
| `--style` | `normal` | Reply style: `normal`, `caveman-lite`, `caveman-full`, `caveman-ultra` |
| `--bus` | `inproc` | Event bus backend. NATS lands in Plan 4. |
| `--working-directory` | cwd | Working directory for both spawned peers (validated for path traversal) |
| `--stream-turns` | `true` | Emit each turn as JSONL on stdout while the conversation runs |

## Recognised peer-option keys

These are the keys that `--peer-a-opt key=value` and `--peer-b-opt key=value` understand:

| Key | Effect |
|---|---|
| `model` | Forwarded as `--model` to the spawned binary |
| `reasoning-effort` | Forwarded as `--reasoning-effort` to the spawned binary (codex only) |
| `cwd` | Working directory for the spawned process |
| `home` | `HOME` env var override for the spawned process |
| `env_KEY=value` | Forward an env var to the spawned process. Only allowlisted keys get through (see below). |

## Env var allowlist for `--peer-{a,b}-opt env_*=`

The broker forwards only these env var names to the spawned peer process:

- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY`
- `MOCK_RESPONSE`
- `MOCK_SENTINEL_AT_TURN`
- `MOCK_DELAY_MS`

Any other key is silently dropped with a stderr warning. This prevents `LD_PRELOAD`, `VITIS_CLAUDE_ARGS`, `VITIS_CODEX_BINARY`, and similar env vars from being smuggled into the spawned agent through `--peer-*-opt`.

## `vitis peek` flags

| Flag | Default | Notes |
|---|---|---|
| `--session-id` | required | The session or conversation id to read |
| `--last` | `10` | Return only the last N turns |
| `--log-path` | `./logs` | Where to look for the persisted session |
| `--log-backend` | `file` | Backend to read from |

## `vitis doctor` flags

| Flag | Default | Notes |
|---|---|---|
| `--provider` | `claude-code` | Provider to probe |

The doctor command writes a JSON object containing the resolved provider binary path, version, and an `rtk` block reporting whether the optional rtk hook is installed and active for that provider.

## Environment variables

| Variable | Purpose |
|---|---|
| `VITIS_CLAUDE_BINARY` | Override the binary path Vitis uses for the claude-code provider |
| `VITIS_CLAUDE_ARGS` | Extra arguments appended to claude invocations (space-separated) |
| `VITIS_CODEX_BINARY` | Override the binary path Vitis uses for the codex provider |
| `VITIS_CODEX_ARGS` | Extra arguments appended to codex invocations (space-separated) |
| `VITIS_MODEL` | Default model id used when no `--model` is given |
| `VITIS_REASONING_EFFORT` | Default reasoning level used when no `--reasoning-effort` is given |
| `HOME` | Resolved by spawned providers for their own state directories |
| `MOCK_RESPONSE`, `MOCK_MODE`, `MOCK_MULTI_TURN`, `MOCK_SENTINEL_AT_TURN`, `MOCK_DELAY_MS`, `MOCK_EXIT_CODE`, `MOCK_BIN` | Configure the bundled mock agent |

## Exit codes

The same convention applies to every Vitis subcommand:

| Code | Meaning |
|---|---|
| 0 | The command reached a terminal status (success or expected failure). The JSON report on stdout describes which. |
| 1 | Runtime error (spawn failure, store init failure, IO error, broker error) |
| 2 | Configuration error (bad flags, missing required arguments, conflicting options) |

The JSON report is always written to stdout regardless of exit code, so a script can rely on `jq .status` or `jq .conversation.status` to make decisions.
