# Vitis Documentation

Vitis is a local PTY orchestrator for AI agent CLIs. It drives Claude Code, Codex, and similar tools through a real terminal so you can run a single prompt and capture the response (`vitis run`) or run a structured multi-turn conversation between two agents (`vitis converse`).

The README at the [repository root](https://github.com/kamilandrzejrybacki-inc/vitis/blob/main/README.md) is the canonical install and quick-start doc. This site exists to give you a navigable view of the design specs, implementation plans, and review reports without cloning the repo.

## What's here

| Section | Contents |
|---|---|
| Specs | Canonical design specs for the A2A conversation system, the v1 agent bridge, and the PTY runtime. |
| Plans | Implementation plans, executed via subagent-driven development. |
| Reviews | Consolidated findings from the parallel review passes. |

## Where to start

If you want a working `vitis converse` against real LLMs in a few minutes, read the [README quick start](https://github.com/kamilandrzejrybacki-inc/vitis/blob/main/README.md#quick-start). If you want to understand the protocol, start with the [A2A Conversations spec](superpowers/specs/2026-04-07-vitis-a2a-conversations-design.md). If you want to see how the foundation packages were built, the [A2A Plan 1](superpowers/plans/2026-04-07-a2a-plan-1-foundation.md) document is the canonical reference, and [A2A Plan 2](superpowers/plans/2026-04-07-a2a-plan-2-pty-cli.md) covers the PTY runtime and CLI wiring. The [review findings](superpowers/reviews/2026-04-07-a2a-review-findings.md) record what every review pass caught and how each finding was fixed.

For day-to-day operation, [`tests/manual/README.md`](https://github.com/kamilandrzejrybacki-inc/vitis/blob/main/tests/manual/README.md) is the operator handbook: 15 numbered shell scripts that exercise every Vitis surface, plus the recommended setup order for the rtk, caveman, and Portkey integrations.
