package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/cli"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:]))
}

func run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: vitis <run|peek|converse|doctor|serve>")
		return 2
	}

	switch args[0] {
	case "run":
		return cli.RunCommand(ctx, args[1:], os.Stdout, os.Stderr)
	case "peek":
		return cli.PeekCommand(ctx, args[1:], os.Stdout, os.Stderr)
	case "converse":
		return cli.ConverseCommand(ctx, args[1:], os.Stdout, os.Stderr)
	case "doctor":
		return cli.DoctorCommand(ctx, args[1:], os.Stdout, os.Stderr)
	case "serve":
		return cli.ServeCommand(ctx, args[1:], os.Stdout, os.Stderr)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		return 2
	}
}
