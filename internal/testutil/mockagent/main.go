package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	delayMs, _ := strconv.Atoi(env("MOCK_DELAY_MS", "0"))
	mode := env("MOCK_MODE", "happy")
	response := env("MOCK_RESPONSE", "mock response")
	exitCode, _ := strconv.Atoi(env("MOCK_EXIT_CODE", "0"))
	multiTurn := env("MOCK_MULTI_TURN", "0") == "1"
	sentinelAt, _ := strconv.Atoi(env("MOCK_SENTINEL_AT_TURN", "0"))
	// Phase 7: addressed-routing trailer support for N-peer integration tests.
	// MOCK_NEXT_TRAILER=<id>: append "<<NEXT: id>>" to every multi-turn reply.
	// Setting it to a real declared peer id exercises the addressed path;
	// setting it to a bogus id (e.g. "ghost") exercises the unknown-addressee
	// fallback. Leaving it unset exercises the no-trailer round-robin path.
	nextTrailer := env("MOCK_NEXT_TRAILER", "")

	if multiTurn {
		runMultiTurn(delayMs, response, sentinelAt, nextTrailer)
		return
	}

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')

	if delayMs > 0 {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	switch mode {
	case "blocked":
		fmt.Fprintln(os.Stdout, "Continue? (y/n)")
		_, _ = reader.ReadString('\n')
	case "auth":
		fmt.Fprintln(os.Stdout, "Authentication required. Please log in.")
		_, _ = reader.ReadString('\n')
	case "rate_limit":
		fmt.Fprintln(os.Stdout, "You've hit your session limit")
	case "ansi":
		fmt.Fprintf(os.Stdout, "\x1b[32m%s\x1b[0m\n", response)
	case "partial":
		fmt.Fprint(os.Stdout, strings.TrimSuffix(response, "\n"))
	case "crash":
		fmt.Fprintln(os.Stdout, "fatal: crashed")
		if exitCode == 0 {
			exitCode = 1
		}
	default:
		fmt.Fprintln(os.Stdout, response)
	}

	os.Exit(exitCode)
}

// runMultiTurn implements the multi-turn loop used by A2A integration
// tests. Each iteration:
//  1. reads bytes from stdin until a recognised marker-instruction line
//     ("output the token <T> on its own line.") is observed; this
//     delimits the end of one envelope.
//  2. optionally sleeps MOCK_DELAY_MS
//  3. prints the configured response (with sentinel prepended on the
//     configured turn)
//  4. prints the marker token on its own line
//
// The loop exits cleanly on stdin EOF.
func runMultiTurn(delayMs int, response string, sentinelAt int, nextTrailer string) {
	reader := bufio.NewReader(os.Stdin)
	turn := 0
	for {
		marker, ok := readEnvelopeMarker(reader)
		if !ok {
			return
		}
		turn++
		if delayMs > 0 {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
		body := response
		// Append the addressed-routing trailer (if configured) BEFORE the
		// sentinel — when both are present, the broker's policy parser
		// gives <<END>> precedence on the last non-empty line, so the
		// sentinel must remain the final line.
		if nextTrailer != "" {
			body = body + "\n<<NEXT: " + nextTrailer + ">>"
		}
		if sentinelAt > 0 && turn == sentinelAt {
			body = body + "\n<<END>>"
		}
		fmt.Fprintf(os.Stdout, "turn %d: %s\n%s\n", turn, body, marker)
	}
}

// readEnvelopeMarker reads lines from r until it finds a line of the form
//
//	...output the token <TOKEN> on its own line.
//
// and returns the extracted token. Returns ("", false) on EOF.
func readEnvelopeMarker(r *bufio.Reader) (string, bool) {
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			if tok := extractMarker(line); tok != "" {
				return tok, true
			}
		}
		if err != nil {
			return "", false
		}
	}
}

func extractMarker(line string) string {
	const needle = "output the token "
	idx := strings.Index(line, needle)
	if idx < 0 {
		return ""
	}
	rest := line[idx+len(needle):]
	end := strings.IndexAny(rest, " \r\n")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
