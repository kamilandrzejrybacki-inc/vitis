package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/kamilandrzejrybacki-inc/clank/internal/cli"
)

// doctorWithArgs calls DoctorCommand and returns the exit code plus captured
// stdout output as a string.
func doctorWithArgs(t *testing.T, args []string) (code int, stdout string) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code = cli.DoctorCommand(context.Background(), args, &outBuf, &errBuf)
	return code, outBuf.String()
}

// TestDoctorCommand_RealProvider_E2E verifies that DoctorCommand correctly
// detects a binary that is always present on CI/dev machines (sh).
func TestDoctorCommand_RealProvider_E2E(t *testing.T) {
	code, stdout := doctorWithArgs(t, []string{"--provider", "sh"})

	if code != 0 {
		t.Fatalf("expected exit code 0 for 'sh' provider, got %d; stdout=%s", code, stdout)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output: %s", err, stdout)
	}

	available, _ := result["provider_available"].(bool)
	if !available {
		t.Fatalf("expected provider_available=true for 'sh', got false; output: %s", stdout)
	}

	path, _ := result["provider_path"].(string)
	if path == "" {
		t.Fatalf("expected non-empty provider_path for 'sh'; output: %s", stdout)
	}
}

// TestDoctorCommand_MissingProvider_E2E verifies that DoctorCommand returns
// provider_available=false for a binary that does not exist in PATH.
func TestDoctorCommand_MissingProvider_E2E(t *testing.T) {
	code, stdout := doctorWithArgs(t, []string{"--provider", "nonexistent-binary-abc123"})

	// Exit code 1 is expected when provider is not found.
	if code == 0 {
		t.Fatalf("expected non-zero exit code for missing provider, got 0; stdout=%s", stdout)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output: %s", err, stdout)
	}

	available, _ := result["provider_available"].(bool)
	if available {
		t.Fatalf("expected provider_available=false for missing binary, got true; output: %s", stdout)
	}
}

// TestDoctorCommand_DefaultProvider_E2E verifies that DoctorCommand still produces
// valid JSON output when no --provider flag is given (defaults to claude-code).
// Claude is unlikely to be installed in CI, so we only check JSON validity and the
// presence of the provider_available field.
func TestDoctorCommand_DefaultProvider_E2E(t *testing.T) {
	// No --provider flag → uses the default ("claude-code").
	_, stdout := doctorWithArgs(t, []string{})

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v; output: %s", err, stdout)
	}

	if _, ok := result["provider_available"]; !ok {
		t.Fatalf("expected provider_available field in output; got: %s", stdout)
	}

	if _, ok := result["provider"]; !ok {
		t.Fatalf("expected provider field in output; got: %s", stdout)
	}
}
