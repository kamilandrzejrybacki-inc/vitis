package config

import (
	"errors"
	"fmt"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

// RunConfig captures and validates the CLI flags for the run command.
type RunConfig struct {
	Prompt      string
	PromptFile  string
	Cwd         string
	TimeoutSec  int
	LogBackend  string
	LogPath     string
	DatabaseURL string
	DebugRaw    bool
	Cols        int
	Rows        int
}

// Validate checks that the RunConfig is internally consistent.
func (c *RunConfig) Validate() error {
	if c.Prompt == "" && c.PromptFile == "" {
		return errors.New("one of --prompt or --prompt-file must be set")
	}
	if c.Prompt != "" && c.PromptFile != "" {
		return errors.New("--prompt and --prompt-file are mutually exclusive")
	}
	if c.TimeoutSec <= 0 {
		return fmt.Errorf("timeout must be > 0, got %d", c.TimeoutSec)
	}
	switch c.LogBackend {
	case "file":
		if c.LogPath == "" {
			return errors.New("--log-path must be set when --log-backend=file")
		}
	case "postgres":
		if c.DatabaseURL == "" {
			return errors.New("--database-url must be set when --log-backend=postgres")
		}
	case "db":
		if c.DatabaseURL == "" {
			return errors.New("--database-url must be set when --log-backend=db")
		}
	default:
		return fmt.Errorf("unknown --log-backend %q; must be file, postgres, or db", c.LogBackend)
	}
	return nil
}

// ToRunRequest converts the RunConfig into a model.RunRequest for the given provider.
func (c *RunConfig) ToRunRequest(provider string) model.RunRequest {
	return model.RunRequest{
		Provider:     provider,
		Prompt:       c.Prompt,
		PromptFile:   c.PromptFile,
		Cwd:          c.Cwd,
		TimeoutSec:   c.TimeoutSec,
		LogBackend:   c.LogBackend,
		LogPath:      c.LogPath,
		DatabaseURL:  c.DatabaseURL,
		DebugRaw:     c.DebugRaw,
		TerminalCols: c.Cols,
		TerminalRows: c.Rows,
	}
}
