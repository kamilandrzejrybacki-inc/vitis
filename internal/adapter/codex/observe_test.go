package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	return string(data)
}

func TestObserve_HappyPathWithExit(t *testing.T) {
	content := loadFixture(t, "happy_path.txt")
	exitCode := 0
	a := New()
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: content,
		ExitCode:       &exitCode,
		BytesSeen:      int64(len(content)),
	})
	if obs == nil {
		t.Fatal("expected observation, got nil")
	}
	if obs.Status != model.RunCompleted {
		t.Fatalf("expected RunCompleted, got %v (reason: %s)", obs.Status, obs.Reason)
	}
	if obs.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", obs.Confidence)
	}
	if !obs.Terminal {
		t.Error("expected Terminal=true")
	}
}

func TestObserve_AuthRequired(t *testing.T) {
	content := loadFixture(t, "auth_required.txt")
	a := New()
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: content,
		IdleMs:         2000,
		BytesSeen:      int64(len(content)),
	})
	if obs == nil {
		t.Fatal("expected observation, got nil")
	}
	if obs.Status != model.RunAuthRequired {
		t.Fatalf("expected RunAuthRequired, got %v (reason: %s)", obs.Status, obs.Reason)
	}
	if !obs.Terminal {
		t.Error("expected Terminal=true")
	}
}

func TestObserve_RateLimited(t *testing.T) {
	content := loadFixture(t, "rate_limited.txt")
	a := New()
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: content,
		IdleMs:         2000,
		BytesSeen:      int64(len(content)),
	})
	if obs == nil {
		t.Fatal("expected observation, got nil")
	}
	if obs.Status != model.RunRateLimited {
		t.Fatalf("expected RunRateLimited, got %v (reason: %s)", obs.Status, obs.Reason)
	}
	if !obs.Terminal {
		t.Error("expected Terminal=true")
	}
}

func TestObserve_CrashWithNonZeroExit(t *testing.T) {
	content := loadFixture(t, "crash.txt")
	exitCode := 1
	a := New()
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: content,
		ExitCode:       &exitCode,
		BytesSeen:      int64(len(content)),
	})
	if obs == nil {
		t.Fatal("expected observation, got nil")
	}
	if obs.Status != model.RunCrashed {
		t.Fatalf("expected RunCrashed, got %v (reason: %s)", obs.Status, obs.Reason)
	}
	if !obs.Terminal {
		t.Error("expected Terminal=true")
	}
}

func TestObserve_StillRunning(t *testing.T) {
	content := loadFixture(t, "happy_path.txt")
	a := New()
	// No exit code, idle time below all thresholds — should return nil (still running).
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: content,
		IdleMs:         500,
		BytesSeen:      int64(len(content)),
	})
	if obs != nil {
		t.Fatalf("expected nil (still running), got status=%v", obs.Status)
	}
}

func TestObserve_IdleFallback(t *testing.T) {
	content := loadFixture(t, "happy_path.txt")
	a := New()
	// No exit code, but idle >= 5000ms — should return low-confidence RunCompleted.
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: content,
		IdleMs:         6000,
		BytesSeen:      int64(len(content)),
	})
	if obs == nil {
		t.Fatal("expected observation from idle fallback, got nil")
	}
	if obs.Status != model.RunCompleted {
		t.Fatalf("expected RunCompleted from idle fallback, got %v", obs.Status)
	}
	if obs.Confidence > 0.5 {
		t.Errorf("expected low confidence for idle fallback, got %f", obs.Confidence)
	}
}

func TestObserve_IdleThresholdBoundary(t *testing.T) {
	a := New()

	// Below heuristic threshold (1499ms): pattern should not trigger.
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: loadFixture(t, "auth_required.txt"),
		IdleMs:         1499,
		BytesSeen:      100,
	})
	if obs != nil {
		t.Fatalf("expected nil below idle threshold 1500ms, got status=%v", obs.Status)
	}

	// At threshold (1500ms): auth pattern should trigger.
	obs = a.Observe(adapter.CompletionContext{
		NormalizedTail: loadFixture(t, "auth_required.txt"),
		IdleMs:         1500,
		BytesSeen:      100,
	})
	if obs == nil || obs.Status != model.RunAuthRequired {
		t.Fatalf("expected RunAuthRequired at idle threshold 1500ms, got %#v", obs)
	}
}

func TestObserve_BlockedPrompt(t *testing.T) {
	a := New()
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: "Do you approve this action? (y/n)",
		IdleMs:         2000,
		BytesSeen:      35,
	})
	if obs == nil || obs.Status != model.RunBlockedOnInput {
		t.Fatalf("expected RunBlockedOnInput, got %#v", obs)
	}
}

func TestObserve_NonZeroExitWithoutOutput(t *testing.T) {
	a := New()
	exitCode := 2
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: "",
		ExitCode:       &exitCode,
		BytesSeen:      0,
	})
	if obs == nil || obs.Status != model.RunCrashed {
		t.Fatalf("expected RunCrashed for non-zero exit, got %#v", obs)
	}
}

func TestObserveFromFixtures(t *testing.T) {
	exitZero := 0

	cases := []struct {
		name       string
		fixture    string
		wantStatus model.RunStatus
		exitCode   *int
		idleMs     int64
	}{
		{
			name:       "happy_path exit 0",
			fixture:    "happy_path.txt",
			wantStatus: model.RunCompleted,
			exitCode:   &exitZero,
			idleMs:     0,
		},
		{
			name:       "auth_required idle",
			fixture:    "auth_required.txt",
			wantStatus: model.RunAuthRequired,
			idleMs:     2000,
		},
		{
			name:       "rate_limited idle",
			fixture:    "rate_limited.txt",
			wantStatus: model.RunRateLimited,
			idleMs:     2000,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := loadFixture(t, tc.fixture)
			a := New()
			obs := a.Observe(adapter.CompletionContext{
				NormalizedTail: content,
				ExitCode:       tc.exitCode,
				IdleMs:         tc.idleMs,
				BytesSeen:      int64(len(content)),
			})
			if obs == nil {
				t.Fatalf("fixture %s: Observe returned nil", tc.fixture)
			}
			if obs.Status != tc.wantStatus {
				t.Fatalf("fixture %s: expected status %v, got %v (reason: %s)",
					tc.fixture, tc.wantStatus, obs.Status, obs.Reason)
			}
		})
	}
}
