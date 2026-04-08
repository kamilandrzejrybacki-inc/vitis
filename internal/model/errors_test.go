package model

import (
	"strings"
	"testing"
)

func TestRunError_Error(t *testing.T) {
	e := &RunError{Code: "E_SPAWN", Message: "spawn failed"}
	got := e.Error()
	if !strings.Contains(got, "E_SPAWN") {
		t.Errorf("expected E_SPAWN in error string, got: %q", got)
	}
	if !strings.Contains(got, "spawn failed") {
		t.Errorf("expected 'spawn failed' in error string, got: %q", got)
	}
}

func TestRunError_Error_NoDetail(t *testing.T) {
	e := &RunError{Code: "E_TIMEOUT", Message: "timed out"}
	got := e.Error()
	if !strings.Contains(got, "E_TIMEOUT") {
		t.Errorf("expected E_TIMEOUT in error string, got: %q", got)
	}
	if !strings.Contains(got, "timed out") {
		t.Errorf("expected 'timed out' in error string, got: %q", got)
	}
}

func TestRunError_Error_NilPointer(t *testing.T) {
	var e *RunError
	if got := e.Error(); got != "" {
		t.Errorf("expected empty string for nil RunError, got: %q", got)
	}
}

func TestErrorCode_Constants(t *testing.T) {
	codes := []ErrorCode{
		ErrorConfig, ErrorSpawn, ErrorPromptIO, ErrorTimeout, ErrorExtract,
		ErrorStore, ErrorRuntime, ErrorProvider, ErrorInput, ErrorNotFound, ErrorInternal,
	}
	seen := map[ErrorCode]bool{}
	for _, c := range codes {
		if c == "" {
			t.Errorf("error code must not be empty")
		}
		if seen[c] {
			t.Errorf("duplicate error code: %q", c)
		}
		seen[c] = true
	}
}
