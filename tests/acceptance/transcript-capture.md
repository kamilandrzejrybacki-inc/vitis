# Transcript Capture

## Goal

Capture representative PTY transcripts from a real local Claude Code run so adapter logic can be regression-tested against real behavior.

## Rules

- sanitize secrets before committing fixtures
- preserve raw bytes where possible
- keep a normalized text version next to the raw capture if needed for review
- record the status the transcript should classify as

## Minimum fixture set

- completed
- blocked_on_input
- auth_required
- rate_limited
- partial
- crashed
