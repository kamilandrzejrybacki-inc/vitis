package cli_test

import (
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/config"
)

func validRunConfig() config.RunConfig {
	return config.RunConfig{
		Prompt:     "hello world",
		TimeoutSec: 600,
		LogBackend: "file",
		LogPath:    "./logs",
		Cols:       80,
		Rows:       24,
	}
}

func TestRunConfigValidate_MissingPrompt(t *testing.T) {
	c := validRunConfig()
	c.Prompt = ""
	c.PromptFile = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error when both prompt and prompt-file are empty")
	}
}

func TestRunConfigValidate_BothPromptAndFile(t *testing.T) {
	c := validRunConfig()
	c.PromptFile = "some-file.txt"
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error when both prompt and prompt-file are set")
	}
}

func TestRunConfigValidate_FileBackendMissingPath(t *testing.T) {
	c := validRunConfig()
	c.LogBackend = "file"
	c.LogPath = ""
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error when log-backend=file and log-path is empty")
	}
}

func TestRunConfigValidate_Valid(t *testing.T) {
	c := validRunConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("expected no validation error for a valid config, got: %v", err)
	}
}

func TestRunConfigValidate_ValidPromptFile(t *testing.T) {
	c := validRunConfig()
	c.Prompt = ""
	c.PromptFile = "prompt.txt"
	if err := c.Validate(); err != nil {
		t.Fatalf("expected no validation error when only prompt-file is set, got: %v", err)
	}
}
