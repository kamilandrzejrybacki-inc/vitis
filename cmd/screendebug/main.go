package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminal"
)

func main() {
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read: %v\n", err)
		os.Exit(1)
	}

	scr := terminal.NewScreen(80, 24)
	scr.Write(raw)
	lines := scr.Lines()

	fmt.Printf("Screen content (%d lines):\n", len(lines))
	for i, line := range lines {
		fmt.Printf("[%3d] %q\n", i, line)
	}

	fmt.Printf("\nNormalized:\n%s\n", strings.Repeat("-", 80))
	normalized := terminal.NormalizePTYText(raw)
	for i, line := range strings.Split(normalized, "\n") {
		if strings.TrimSpace(line) != "" {
			fmt.Printf("[%3d] %q\n", i, line)
		}
	}
}
