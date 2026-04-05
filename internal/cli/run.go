package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter/claudecode"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/orchestrator"
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

	if err := fs.Parse(args); err != nil {
		_ = WriteJSON(stdout, ErrorResult(model.ErrorConfig, err.Error()))
		return 2
	}

	if req.LogBackend != "file" && req.LogBackend != "db" {
		_ = WriteJSON(stdout, ErrorResult(model.ErrorConfig, "log-backend must be file or db"))
		return 2
	}
	if req.LogBackend == "db" && req.DatabaseURL == "" {
		_ = WriteJSON(stdout, ErrorResult(model.ErrorConfig, "database-url is required when log-backend=db"))
		return 2
	}

	store, err := buildStore(ctx, req)
	if err != nil {
		_ = WriteJSON(stdout, ErrorResult(model.ErrorConfig, err.Error()))
		return 2
	}
	defer store.Close()

	if req.Cwd == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			req.Cwd = cwd
		}
	}

	deps := orchestrator.Dependencies{
		Adapters: adapter.NewRegistry(claudecode.New()),
		Runtime:  terminal.NewRuntime(),
		Store:    store,
	}

	result, err := orchestrator.Run(ctx, req, deps)
	if err != nil {
		runErr, ok := err.(*model.RunError)
		if !ok {
			runErr = &model.RunError{Code: model.ErrorInternal, Message: err.Error()}
		}
		_ = WriteJSON(stdout, ErrorResult(runErr.Code, runErr.Message))
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

func buildStore(ctx context.Context, req model.RunRequest) (interface {
	Close() error
	CreateSession(model.Session) error
	UpdateSession(string, model.SessionPatch) error
	AppendTurn(model.Turn) error
	PeekTurns(string, int) ([]model.Turn, error)
	AppendStreamEvent(model.StoredStreamEvent) error
}, error) {
	if req.LogBackend == "db" {
		return pgstore.New(ctx, req.DatabaseURL)
	}
	return filestore.New(req.LogPath, req.DebugRaw)
}
