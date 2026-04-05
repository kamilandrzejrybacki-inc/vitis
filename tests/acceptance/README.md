# Clank Acceptance

This directory documents manual acceptance checks for Clank against a real local Claude Code installation.

For local harness validation without a real Claude Code session, set `CLANK_CLAUDE_BINARY` to a test double such as the bundled mock agent.

The acceptance goal is not just "does it run".

The acceptance goal is:

- honest status classification
- raw transcript fidelity
- sane extraction quality
- predictable operator-facing failure modes

See `transcript-capture.md` and `cases.md`.
