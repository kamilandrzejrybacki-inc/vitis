package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile_Empty(t *testing.T) {
	result, err := LoadEnvFile("")
	if err != nil {
		t.Fatalf("expected no error for empty path, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil map for empty path")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestLoadEnvFile_ValidFile(t *testing.T) {
	content := "KEY1=value1\nKEY2=value2\nKEY3=value3\n"
	path := writeTempFile(t, content)

	result, err := LoadEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	}
	for k, v := range want {
		if got, ok := result[k]; !ok {
			t.Errorf("missing key %q", k)
		} else if got != v {
			t.Errorf("key %q: want %q, got %q", k, v, got)
		}
	}
}

func TestLoadEnvFile_CommentsAndBlanks(t *testing.T) {
	content := "# this is a comment\n\nKEY=hello\n\n# another comment\n"
	path := writeTempFile(t, content)

	result, err := LoadEnvFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 entry, got %d: %v", len(result), result)
	}
	if got := result["KEY"]; got != "hello" {
		t.Errorf("KEY: want %q, got %q", "hello", got)
	}
}

func TestLoadEnvFile_MissingFile(t *testing.T) {
	_, err := LoadEnvFile("/nonexistent/path/that/does/not/exist.env")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadEnvFile_MalformedLine(t *testing.T) {
	// A line without '=' has no separator; the actual implementation returns an error.
	content := "NOEQUALS\n"
	path := writeTempFile(t, content)

	_, err := LoadEnvFile(path)
	if err == nil {
		t.Fatal("expected error for malformed line (no '='), got nil")
	}
}

// writeTempFile writes content to a temporary file and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}
