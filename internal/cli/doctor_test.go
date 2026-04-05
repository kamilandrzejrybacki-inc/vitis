package cli_test

import (
	"testing"

	"github.com/kamilandrzejrybacki-inc/clank/internal/util"
)

// TestDoctorDetectsMissingProvider verifies that util.LookPath returns an error
// when the executable is not present in PATH. The doctor command relies on this
// utility to detect unavailable providers.
func TestDoctorDetectsMissingProvider(t *testing.T) {
	const nonexistent = "clank-nonexistent-provider-xyz-12345"
	_, err := util.LookPath(nonexistent)
	if err == nil {
		t.Fatalf("expected LookPath to return an error for %q, but got nil", nonexistent)
	}
}

// TestDoctorDetectsAvailableProvider verifies that util.LookPath succeeds for
// a binary that is known to be present on all CI/dev machines (e.g. "sh").
func TestDoctorDetectsAvailableProvider(t *testing.T) {
	path, err := util.LookPath("sh")
	if err != nil {
		t.Fatalf("expected LookPath to find 'sh', got error: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path for 'sh'")
	}
}
