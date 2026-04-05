# Clank

Clank is a local-first PTY orchestrator for driving Claude Code from a normal terminal session.

The implementation in this repo is intentionally scoped to pure terminal control. It does not rely on headless browser automation, and it is not designed to broker or share consumer Claude subscriptions for other users.

## Status

This repo now includes:

- a PTY runtime backed by `github.com/creack/pty`
- a Claude Code adapter with terminal-state detection
- file and Postgres transcript/session stores
- a JSON CLI with `run`, `peek`, and `doctor`
- fixture support and a mock-agent integration path for end-to-end PTY tests

## Commands

Run a prompt through the local Claude Code binary:

```bash
go run ./cmd/clank run --prompt "summarize the latest changes"
```

Inspect the local environment:

```bash
go run ./cmd/clank doctor
```

Read back the last turns from a session:

```bash
go run ./cmd/clank peek --session-id <session-id>
```

## Testing

Run the Go suite with writable caches:

```bash
env GOCACHE=/tmp/clank-go-build GOMODCACHE=/tmp/clank-go-mod go test ./...
```

For end-to-end PTY validation without a real Claude Code session, point the adapter at the bundled mock agent:

```bash
env \
  GOCACHE=/tmp/clank-go-build \
  GOMODCACHE=/tmp/clank-go-mod \
  CLANK_CLAUDE_BINARY=go \
  CLANK_CLAUDE_ARGS="run ./internal/testutil/mockagent" \
  MOCK_RESPONSE="mock integration response" \
  go run ./cmd/clank run --prompt "ping" --log-path /tmp/clank-logs --debug-raw
```

`CLANK_CLAUDE_BINARY` overrides the executable used by the Claude Code adapter. `CLANK_CLAUDE_ARGS` can append static arguments as a space-separated string.

## Safety Boundary

- Prefer local one-user operation.
- Do not position Clank as a hosted proxy for consumer Claude accounts.
- Do not auto-answer auth, permission, or rate-limit prompts.
- Treat raw PTY bytes as the source of truth for audits and debugging.

Detailed design material lives under `docs/superpowers/`.
