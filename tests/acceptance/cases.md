# Acceptance Cases

## Happy Path

- run `clank run --provider claude-code --prompt "What is 2+2?"`
- expect `status: "completed"`
- expect a non-empty response

## Blocked Prompt

- trigger a prompt that requires confirmation
- expect `status: "blocked_on_input"`
- expect no false success

## Authentication Required

- run in an environment where Claude Code requires login
- expect `status: "auth_required"`

## Rate Limited

- capture a run after usage is exhausted
- expect `status: "rate_limited"`

## Timeout

- force timeout with a short deadline
- expect `status: "timeout"`
