---
name: claudecode-pty-chrome-patterns
description: "Claude Code Ink TUI uses relative-only cursor moves; filter ●─, ✻, standalone % lines as chrome when extracting PTY responses"
user-invocable: false
origin: auto-extracted
---

# Claude Code PTY Output: TUI Rendering Characteristics and Chrome Patterns

**Extracted:** 2026-04-07  
**Context:** Extracting Claude Code's text response from raw PTY bytes in clank

## Problem

Claude Code's Ink/React TUI produces output that is hard to parse naively:
1. Uses ONLY **relative cursor movements** (CUF, CUU, CUD, CUB, `\r\n`) — NO absolute CUP (`H`) or DECSTBM (`r`) escape sequences
2. Ink re-renders the entire UI on each token, leaving intermediate "streaming" artifacts in the PTY output
3. The live **session token usage bar** (e.g. `"16.9%"`) appears as a standalone line in the final rendered screen
4. During streaming, responses are wrapped: `●─CONTENT─────────────────────` (full line is chrome)
5. Thinking/coalescing indicators: `✻ Coalescing…`, `✽ Thinking…`, `● 4`, etc.

All of these must be classified as **UI chrome**, not response content.

## Solution

In `isUIChrome()`, add detection for:

```go
// Standalone percentage — built-in live token usage bar (e.g. "16.9%")
// Present with or without ccstatusline plugin.
var chromePercentageRE = regexp.MustCompile(`^\d+(\.\d+)?%$`)

// Ink streaming/thinking indicators
var chromeStreamingRE = regexp.MustCompile(`^[✻✽✶✢·●]\s`)

// In isUIChrome():
if chromePercentageRE.MatchString(line) { return true }
if chromeStreamingRE.MatchString(line)  { return true }
if strings.HasPrefix(line, "●─") { return true }
```

The ccstatusline plugin (`github.com/sirmalloc/ccstatusline`) adds `Ctx(u): 0.0%`, `Session: <1m`, `Model: Sonnet 4.6` status bar lines — but these are **optional**. Chrome detection must work without them.

Built-in chrome always present (no plugin needed):
- `❯` prompt prefix
- `? for shortcuts` footer
- Box-drawing separator lines (`──────────────────────`)
- User-message separator (`──prompt text────────────`)

## Confirmed Facts (from PTY capture analysis)

- Claude Code PTY capture (3243 bytes): **0 instances** of CUP (`H`) or DECSTBM (`r`) CSI sequences
- All cursor movement is relative: CUF, CUU, CUD, CUB
- Response text settles at a stable row in the bounded 24-row screen after all re-renders complete
- `\r` (carriage return) is required to submit input — `\n` triggers the API call but the response never renders to the PTY

## When to Use

- Writing or debugging `extractFromScreen` / `isUIChrome` in clank
- Adding support for new Claude Code output patterns
- Investigating why clank returns empty or garbled responses
