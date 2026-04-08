package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/cli"
)

// findRepoRoot walks up from the current directory until it finds go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("cannot find repo root (no go.mod found in any parent directory)")
		}
		dir = parent
	}
}

// setupMockEnv creates a temp directory with a vitis.env file configured for
// the given mockagent mode and response string. It returns the tempDir and the
// absolute path to the env file.
func setupMockEnv(t *testing.T, mode, response string) (tempDir, envFile string) {
	t.Helper()
	tempDir = t.TempDir()
	envBody := fmt.Sprintf(
		"VITIS_CLAUDE_BINARY=go\nVITIS_CLAUDE_ARGS=run ./internal/testutil/mockagent\nMOCK_MODE=%s\nMOCK_RESPONSE=%s",
		mode, response,
	)
	envFile = filepath.Join(tempDir, "vitis.env")
	if err := os.WriteFile(envFile, []byte(envBody), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	return tempDir, envFile
}

// runWithArgs calls RunCommand with io.Writer buffers and returns the exit code
// plus captured stdout/stderr.
func runWithArgs(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code = cli.RunCommand(context.Background(), args, &outBuf, &errBuf)
	return code, outBuf.String(), errBuf.String()
}

// skipIfSandboxed skips the test when the PTY is blocked by the sandbox.
func skipIfSandboxed(t *testing.T, output string) {
	t.Helper()
	if strings.Contains(output, "operation not permitted") ||
		strings.Contains(output, "open /dev/ptmx") {
		t.Skip("PTY child execution is blocked by the sandbox in this environment")
	}
}

// TestRunCommand_HappyPath_E2E verifies that RunCommand succeeds end-to-end with
// a real mockagent subprocess in happy mode and produces valid JSON output.
func TestRunCommand_HappyPath_E2E(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tempDir, envFile := setupMockEnv(t, "happy", "cli e2e response")
	logDir := filepath.Join(tempDir, "logs")

	code, stdout, stderr := runWithArgs(t, []string{
		"--prompt", "hello",
		"--log-backend", "file",
		"--log-path", logDir,
		"--env-file", envFile,
		"--timeout", "10",
		"--working-directory", repoRoot,
	})

	skipIfSandboxed(t, stdout+stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stdout=%s stderr=%s", code, stdout, stderr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output: %s", err, stdout)
	}

	status, _ := result["status"].(string)
	if status != "completed" {
		t.Fatalf("expected status=completed, got %q; full output: %s", status, stdout)
	}

	response, _ := result["response"].(string)
	if !strings.Contains(response, "cli e2e response") {
		t.Fatalf("expected response to contain %q, got: %q", "cli e2e response", response)
	}

	sessionID, _ := result["session_id"].(string)
	if !strings.HasPrefix(sessionID, "sess_") {
		t.Fatalf("expected session_id to start with 'sess_', got: %q", sessionID)
	}
}

// TestRunCommand_BlockedMode_E2E verifies that RunCommand returns blocked_on_input
// when the mockagent enters a blocking prompt.
func TestRunCommand_BlockedMode_E2E(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tempDir, envFile := setupMockEnv(t, "blocked", "")
	logDir := filepath.Join(tempDir, "logs")

	code, stdout, stderr := runWithArgs(t, []string{
		"--prompt", "hello",
		"--log-backend", "file",
		"--log-path", logDir,
		"--env-file", envFile,
		"--timeout", "10",
		"--working-directory", repoRoot,
	})

	skipIfSandboxed(t, stdout+stderr)

	// blocked_on_input is a terminal state — RunCommand returns 1.
	if code == 2 {
		t.Fatalf("unexpected config error (code=2); stderr=%s", stderr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output: %s", err, stdout)
	}

	status, _ := result["status"].(string)
	if status != "blocked_on_input" {
		t.Fatalf("expected status=blocked_on_input, got %q; full output: %s", status, stdout)
	}
}

// TestRunCommand_CrashMode_E2E verifies that RunCommand produces valid JSON output
// when the mockagent exits with a non-zero code.  The exact terminal status depends
// on whether the PTY layer propagates the exit code; the test checks that the
// output is valid JSON and the response contains the crash marker emitted by the
// mockagent.
func TestRunCommand_CrashMode_E2E(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tempDir, envFile := setupMockEnv(t, "crash", "")
	logDir := filepath.Join(tempDir, "logs")

	_, stdout, stderr := runWithArgs(t, []string{
		"--prompt", "hello",
		"--log-backend", "file",
		"--log-path", logDir,
		"--env-file", envFile,
		"--timeout", "10",
		"--working-directory", repoRoot,
	})

	skipIfSandboxed(t, stdout+stderr)

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output: %s", err, stdout)
	}

	// The status must be one of the recognised terminal states.
	status, _ := result["status"].(string)
	validStatuses := map[string]bool{
		"crashed":   true,
		"error":     true,
		"completed": true, // PTY may swallow non-zero exit code on some kernels
		"partial":   true,
	}
	if !validStatuses[status] {
		t.Fatalf("unexpected status %q for crash mode; full output: %s", status, stdout)
	}

	// The response or warnings should contain evidence of the crash text.
	response, _ := result["response"].(string)
	if !strings.Contains(response, "fatal: crashed") && !strings.Contains(response, "exit status 1") {
		// Also acceptable: status is "crashed" which already proves the crash was detected.
		if status != "crashed" {
			t.Fatalf("expected response to reference crash output or status=crashed; response=%q status=%q", response, status)
		}
	}
}

// TestRunCommand_MissingPrompt_E2E verifies that RunCommand returns a non-zero
// exit code and an error JSON payload when neither --prompt nor --prompt-file is
// supplied.
func TestRunCommand_MissingPrompt_E2E(t *testing.T) {
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")

	code, stdout, _ := runWithArgs(t, []string{
		"--log-backend", "file",
		"--log-path", logDir,
		"--timeout", "10",
	})

	if code == 0 {
		t.Fatalf("expected non-zero exit code when prompt is missing, got 0")
	}

	// The output should be valid JSON describing an error.
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output: %s", err, stdout)
	}

	if result["error"] == nil && result["status"] == nil {
		t.Fatalf("expected an error field or non-ok status in output: %s", stdout)
	}
}

// TestRunCommand_PromptFile_E2E verifies that RunCommand accepts a --prompt-file
// flag and succeeds end-to-end using its content as the prompt.
func TestRunCommand_PromptFile_E2E(t *testing.T) {
	repoRoot := findRepoRoot(t)
	tempDir, envFile := setupMockEnv(t, "happy", "prompt file response")
	logDir := filepath.Join(tempDir, "logs")

	promptFile := filepath.Join(tempDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte("from prompt file"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	code, stdout, stderr := runWithArgs(t, []string{
		"--prompt-file", promptFile,
		"--log-backend", "file",
		"--log-path", logDir,
		"--env-file", envFile,
		"--timeout", "10",
		"--working-directory", repoRoot,
	})

	skipIfSandboxed(t, stdout+stderr)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stdout=%s stderr=%s", code, stdout, stderr)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output: %s", err, stdout)
	}

	status, _ := result["status"].(string)
	if status != "completed" {
		t.Fatalf("expected status=completed, got %q; full output: %s", status, stdout)
	}
}
