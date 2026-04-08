# Install

## Requirements

- Go 1.22 or newer (only for building from source)
- A real `claude` or `codex` install on your `PATH` for production use, or the bundled mock agent for tests and demos
- Linux or macOS (Windows is untested; the PTY runtime depends on POSIX pty semantics)

## With go install

```bash
go install github.com/kamilandrzejrybacki-inc/vitis/cmd/vitis@latest
```

This drops a `vitis` binary into `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/kamilandrzejrybacki-inc/vitis.git
cd vitis
go build -o vitis ./cmd/vitis
```

You can move the resulting binary anywhere on your `PATH`:

```bash
sudo install -m 0755 vitis /usr/local/bin/vitis
```

## From a release archive

Tagged releases publish prebuilt binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`. Grab the archive that matches your platform from the [releases page](https://github.com/kamilandrzejrybacki-inc/vitis/releases), check the `.sha256` sidecar, and extract:

```bash
tar xzf vitis-0.2.0-linux-amd64.tar.gz
sudo install -m 0755 vitis /usr/local/bin/vitis
```

## Verify the install

```bash
vitis doctor --provider claude-code
```

You should see a JSON report listing the resolved binary path, the provider version, and the rtk integration status. If `provider_available` is `false`, install Claude Code first or set `VITIS_CLAUDE_BINARY` to point at the binary you want Vitis to drive.

## Override the binary

Vitis resolves provider binaries via three mechanisms, in order:

1. The environment variable for the provider (`VITIS_CLAUDE_BINARY`, `VITIS_CODEX_BINARY`)
2. The system `PATH`
3. A hardcoded default (`claude`, `codex`)

For tests, point at the bundled mock agent:

```bash
go build -o /tmp/mockagent ./internal/testutil/mockagent
export VITIS_CLAUDE_BINARY=/tmp/mockagent
export MOCK_RESPONSE="hello from the mock"
vitis run --provider claude-code --prompt "ping"
```
