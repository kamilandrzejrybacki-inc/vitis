package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter/claudecode"
	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter/codex"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/orchestrator"
	"github.com/kamilandrzejrybacki-inc/clank/internal/store"
	filestore "github.com/kamilandrzejrybacki-inc/clank/internal/store/file"
	pgstore "github.com/kamilandrzejrybacki-inc/clank/internal/store/postgres"
	"github.com/kamilandrzejrybacki-inc/clank/internal/terminal"
)

func RunCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var req model.RunRequest
	fs.StringVar(&req.Provider, "provider", "claude-code", "provider id")
	fs.StringVar(&req.Prompt, "prompt", "", "inline prompt")
	fs.StringVar(&req.PromptFile, "prompt-file", "", "path to prompt file")
	fs.IntVar(&req.TimeoutSec, "timeout", 600, "timeout in seconds")
	fs.IntVar(&req.PeekLast, "peek-last", 10, "number of turns to include in result")
	fs.StringVar(&req.Cwd, "working-directory", "", "working directory")
	fs.StringVar(&req.EnvFile, "env-file", "", "env file path")
	fs.StringVar(&req.LogBackend, "log-backend", "file", "file or db")
	fs.StringVar(&req.LogPath, "log-path", "./logs", "file backend path")
	fs.StringVar(&req.DatabaseURL, "database-url", "", "postgres database url")
	fs.BoolVar(&req.DebugRaw, "debug-raw", false, "persist raw PTY events")
	fs.IntVar(&req.TerminalCols, "terminal-cols", 80, "terminal columns")
	fs.IntVar(&req.TerminalRows, "terminal-rows", 24, "terminal rows")
	fs.StringVar(&req.HomeDir, "home-dir", "", "home directory override")
	fs.StringVar(&req.Model, "model", "", "model name (passed to provider)")
	fs.StringVar(&req.ReasoningEffort, "reasoning-effort", "", "reasoning effort level (provider-specific)")

	if err := fs.Parse(args); err != nil {
		if writeErr := WriteJSON(stdout, ErrorResult(model.ErrorConfig, err.Error())); writeErr != nil {
			fmt.Fprintf(os.Stderr, "clank: failed to write output: %v\n", writeErr)
		}
		return 2
	}

	if req.LogBackend != "file" && req.LogBackend != "db" {
		if writeErr := WriteJSON(stdout, ErrorResult(model.ErrorConfig, "log-backend must be file or db")); writeErr != nil {
			fmt.Fprintf(os.Stderr, "clank: failed to write output: %v\n", writeErr)
		}
		return 2
	}
	if req.LogBackend == "db" && req.DatabaseURL == "" {
		if writeErr := WriteJSON(stdout, ErrorResult(model.ErrorConfig, "database-url is required when log-backend=db")); writeErr != nil {
			fmt.Fprintf(os.Stderr, "clank: failed to write output: %v\n", writeErr)
		}
		return 2
	}

	store, err := buildStore(ctx, req.LogBackend, req.LogPath, req.DatabaseURL, req.DebugRaw)
	if err != nil {
		if writeErr := WriteJSON(stdout, ErrorResult(model.ErrorConfig, err.Error())); writeErr != nil {
			fmt.Fprintf(os.Stderr, "clank: failed to write output: %v\n", writeErr)
		}
		return 2
	}
	defer store.Close()

	if req.Cwd == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			req.Cwd = cwd
		}
	}

	deps := orchestrator.Dependencies{
		Adapters: adapter.NewRegistry(claudecode.New(), codex.New()),
		Runtime:  terminal.NewRuntime(),
		Store:    store,
	}

	result, err := orchestrator.Run(ctx, req, deps)
	if err != nil {
		runErr, ok := err.(*model.RunError)
		if !ok {
			runErr = &model.RunError{Code: model.ErrorInternal, Message: err.Error()}
		}
		if writeErr := WriteJSON(stdout, ErrorResult(runErr.Code, runErr.Message)); writeErr != nil {
			fmt.Fprintf(os.Stderr, "clank: failed to write output: %v\n", writeErr)
		}
		return 1
	}

	if err := WriteJSON(stdout, result); err != nil {
		fmt.Fprintf(stderr, "write result: %v\n", err)
		return 1
	}
	if result.Error != nil {
		return 1
	}
	return 0
}

func buildStore(ctx context.Context, backend, logPath, dbURL string, debugRaw bool) (store.Store, error) {
	if backend == "db" {
		return pgstore.New(ctx, dbURL)
	}
	return filestore.New(logPath, debugRaw)
}
