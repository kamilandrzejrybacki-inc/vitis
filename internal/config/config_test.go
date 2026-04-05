package config

import (
	"testing"
)

func validConfig() RunConfig {
	return RunConfig{
		Prompt:     "hello",
		TimeoutSec: 60,
		LogBackend: "file",
		LogPath:    "./logs",
		Cols:       80,
		Rows:       24,
	}
}

func TestValidate_MissingPrompt(t *testing.T) {
	c := validConfig()
	c.Prompt = ""
	c.PromptFile = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for missing prompt and prompt-file, got nil")
	}
}

func TestValidate_BothPromptAndFile(t *testing.T) {
	c := validConfig()
	c.PromptFile = "prompt.txt"
	if err := c.Validate(); err == nil {
		t.Fatal("expected error when both prompt and prompt-file are set, got nil")
	}
}

func TestValidate_TimeoutZero(t *testing.T) {
	c := validConfig()
	c.TimeoutSec = 0
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for timeout=0, got nil")
	}
}

func TestValidate_FileBackendMissingPath(t *testing.T) {
	c := validConfig()
	c.LogBackend = "file"
	c.LogPath = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for file backend with empty log-path, got nil")
	}
}

func TestValidate_PostgresBackendMissingURL(t *testing.T) {
	c := validConfig()
	c.LogBackend = "postgres"
	c.DatabaseURL = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for postgres backend with empty database-url, got nil")
	}
}

func TestValidate_DBBackendMissingURL(t *testing.T) {
	c := validConfig()
	c.LogBackend = "db"
	c.DatabaseURL = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for db backend with empty database-url, got nil")
	}
}

func TestValidate_UnknownBackend(t *testing.T) {
	c := validConfig()
	c.LogBackend = "memory"
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for unknown log-backend, got nil")
	}
}

func TestValidate_Valid(t *testing.T) {
	c := validConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}
}

func TestToRunRequest(t *testing.T) {
	c := RunConfig{
		Prompt:      "do something",
		Cwd:         "/tmp/work",
		TimeoutSec:  120,
		LogBackend:  "file",
		LogPath:     "./logs",
		DebugRaw:    true,
		Cols:        120,
		Rows:        40,
	}
	req := c.ToRunRequest("claude-code")
	if req.Provider != "claude-code" {
		t.Errorf("Provider: want claude-code, got %q", req.Provider)
	}
	if req.Prompt != c.Prompt {
		t.Errorf("Prompt mismatch: want %q, got %q", c.Prompt, req.Prompt)
	}
	if req.TerminalCols != c.Cols {
		t.Errorf("TerminalCols: want %d, got %d", c.Cols, req.TerminalCols)
	}
	if req.TerminalRows != c.Rows {
		t.Errorf("TerminalRows: want %d, got %d", c.Rows, req.TerminalRows)
	}
}
