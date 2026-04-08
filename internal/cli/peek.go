package cli

import (
	"context"
	"flag"
	"io"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func PeekCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("peek", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var sessionID string
	var last int
	var backend string
	var logPath string
	var databaseURL string

	fs.StringVar(&sessionID, "session-id", "", "session id")
	fs.IntVar(&last, "last", 10, "number of turns")
	fs.StringVar(&backend, "log-backend", "file", "file or db")
	fs.StringVar(&logPath, "log-path", "./logs", "file backend path")
	fs.StringVar(&databaseURL, "database-url", "", "postgres database url")
	if err := fs.Parse(args); err != nil {
		_ = WriteJSON(stdout, ErrorResult(model.ErrorConfig, err.Error()))
		return 2
	}
	if sessionID == "" {
		_ = WriteJSON(stdout, ErrorResult(model.ErrorConfig, "session-id is required"))
		return 2
	}

	store, storeErr := buildStore(ctx, backend, logPath, databaseURL, false)
	if storeErr != nil {
		_ = WriteJSON(stdout, ErrorResult(model.ErrorConfig, storeErr.Error()))
		return 2
	}
	defer store.Close()

	turns, err := store.PeekTurns(ctx, sessionID, last)
	if err != nil {
		_ = WriteJSON(stdout, ErrorResult(model.ErrorNotFound, err.Error()))
		return 1
	}

	_ = WriteJSON(stdout, map[string]any{
		"session_id": sessionID,
		"turns":      turns,
	})
	return 0
}
