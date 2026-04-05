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

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')

	if delayMs > 0 {
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	switch mode {
	case "blocked":
		fmt.Fprintln(os.Stdout, "Continue? (y/n)")
	case "auth":
		fmt.Fprintln(os.Stdout, "Authentication required. Please log in.")
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

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
