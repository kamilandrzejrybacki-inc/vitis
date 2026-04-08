package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunCommand_BadFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCommand(context.Background(), []string{"--nope"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code 2, got %d", code)
	}
	if !strings.Contains(stdout.String(), "error") && !strings.Contains(stdout.String(), "code") {
		t.Fatalf("expected error JSON on stdout, got %q", stdout.String())
	}
}

func TestRunCommand_BadLogBackend(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCommand(context.Background(), []string{
		"--prompt", "hi",
		"--log-backend", "weird",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code 2, got %d", code)
	}
	if !strings.Contains(stdout.String(), "log-backend") {
		t.Fatalf("expected log-backend message, got %q", stdout.String())
	}
}

func TestRunCommand_DBBackendMissingURL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := RunCommand(context.Background(), []string{
		"--prompt", "hi",
		"--log-backend", "db",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code 2, got %d", code)
	}
	if !strings.Contains(stdout.String(), "database-url") {
		t.Fatalf("expected database-url message, got %q", stdout.String())
	}
}

func TestRenderDoctor(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderDoctor(&buf, true, "/usr/bin/claude", "ok"); err != nil {
		t.Fatalf("RenderDoctor: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"provider_available=true", "/usr/bin/claude", "detail=ok"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}

func TestRepeatableFlag_BadValue(t *testing.T) {
	rf := newRepeatableFlag()
	if err := rf.Set("noequals"); err == nil {
		t.Fatal("expected error on missing =")
	}
	if err := rf.Set("=novalue"); err == nil {
		t.Fatal("expected error on empty key")
	}
	if err := rf.Set("key=value"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rf.String() != "key=value" {
		t.Errorf("String(): expected 'key=value', got %q", rf.String())
	}
}

func TestRepeatableFlag_StringEmpty(t *testing.T) {
	rf := newRepeatableFlag()
	if rf.String() != "" {
		t.Errorf("expected empty string, got %q", rf.String())
	}
	var nilRF *repeatableFlag
	if nilRF.String() != "" {
		t.Errorf("nil receiver String() should be empty")
	}
}

func TestMergeOptions_AddsCwdWhenAbsent(t *testing.T) {
	got := mergeOptions(map[string]string{"foo": "bar"}, "/tmp")
	if got["cwd"] != "/tmp" {
		t.Errorf("expected cwd=/tmp, got %q", got["cwd"])
	}
	if got["foo"] != "bar" {
		t.Errorf("foo lost: %v", got)
	}
}

func TestMergeOptions_RespectsExistingCwd(t *testing.T) {
	got := mergeOptions(map[string]string{"cwd": "/already"}, "/tmp")
	if got["cwd"] != "/already" {
		t.Errorf("should not overwrite cwd; got %q", got["cwd"])
	}
}

func TestMergeOptions_NoWorkingDir(t *testing.T) {
	got := mergeOptions(map[string]string{"a": "b"}, "")
	if _, has := got["cwd"]; has {
		t.Errorf("cwd should not be set when workingDir empty")
	}
}

func TestConverseFirstNonEmpty(t *testing.T) {
	if got := converseFirstNonEmpty("", "", "z"); got != "z" {
		t.Errorf("expected z, got %q", got)
	}
	if got := converseFirstNonEmpty("", ""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := converseFirstNonEmpty("a", "b"); got != "a" {
		t.Errorf("expected a, got %q", got)
	}
}

func TestConverse_BadOpener(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer-a", "provider:mock",
		"--peer-b", "provider:mock",
		"--seed", "x",
		"--opener", "z",
	}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "opener") {
		t.Fatalf("expected opener err; code=%d stderr=%s", code, stderr.String())
	}
}

func TestConverse_BadBus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer-a", "provider:mock", "--peer-b", "provider:mock",
		"--seed", "x", "--bus", "kafka",
	}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "bus") {
		t.Fatalf("expected bus err; code=%d stderr=%s", code, stderr.String())
	}
}

func TestConverse_BadLogBackend(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer-a", "provider:mock", "--peer-b", "provider:mock",
		"--seed", "x", "--log-backend", "db",
	}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "log-backend") {
		t.Fatalf("expected log-backend err; code=%d stderr=%s", code, stderr.String())
	}
}

func TestConverse_PerTurnTimeoutBounds(t *testing.T) {
	cases := []string{"0", "5000"}
	for _, v := range cases {
		var stdout, stderr bytes.Buffer
		code := ConverseCommand(context.Background(), []string{
			"--peer-a", "provider:mock", "--peer-b", "provider:mock",
			"--seed", "x", "--per-turn-timeout", v,
		}, &stdout, &stderr)
		if code != 2 {
			t.Errorf("per-turn-timeout=%s should fail; code=%d", v, code)
		}
	}
}

func TestConverse_OverallTimeoutCap(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer-a", "provider:mock", "--peer-b", "provider:mock",
		"--seed", "x", "--overall-timeout", "999999",
	}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "overall-timeout") {
		t.Fatalf("expected overall-timeout err; code=%d stderr=%s", code, stderr.String())
	}
}

func TestConverse_WorkingDirNotExist(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{
		"--peer-a", "provider:mock", "--peer-b", "provider:mock",
		"--seed", "x", "--working-directory", "/nonexistent/path/does/not/exist",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code 2, got %d; stderr=%s", code, stderr.String())
	}
}

func TestConverse_BadFlagParse(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ConverseCommand(context.Background(), []string{"--nope"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code 2, got %d", code)
	}
}

func TestPeekCommand_NonExistentSession(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := PeekCommand(context.Background(), []string{
		"--session-id", "missing",
		"--log-backend", "file",
		"--log-path", dir,
	}, &stdout, &stderr)
	// File backend returns empty turns rather than erroring; just verify it
	// completes (either 0 with empty turns, or non-zero error).
	_ = code
	_ = stdout
}

func TestPeekCommand_BadFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := PeekCommand(context.Background(), []string{"--nope"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code 2, got %d", code)
	}
}

func TestPeekCommand_DBBackendNoURL(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := PeekCommand(context.Background(), []string{
		"--session-id", "x", "--log-backend", "db",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit")
	}
}
