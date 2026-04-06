package claudecode

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

func TestObserveBlockedPrompt(t *testing.T) {
	a := New()
	observation := a.Observe(adapter.CompletionContext{
		NormalizedTail: "Continue? (y/n)\n",
		IdleMs:         2000,
		BytesSeen:      10,
	})
	if observation == nil || observation.Status != model.RunBlockedOnInput {
		t.Fatalf("expected blocked_on_input, got %#v", observation)
	}
}

func TestObserveAuthPrompt(t *testing.T) {
	a := New()
	observation := a.Observe(adapter.CompletionContext{
		NormalizedTail: "Authentication required. Please log in.\n",
		IdleMs:         2000,
		BytesSeen:      10,
	})
	if observation == nil || observation.Status != model.RunAuthRequired {
		t.Fatalf("expected auth_required, got %#v", observation)
	}
}

func TestObserveExitCompletion(t *testing.T) {
	a := New()
	exitCode := 0
	observation := a.Observe(adapter.CompletionContext{
		NormalizedTail: "done\n",
		ExitCode:       &exitCode,
		BytesSeen:      4,
	})
	if observation == nil || observation.Status != model.RunCompleted {
		t.Fatalf("expected completed, got %#v", observation)
	}
}

func TestObserve_IdleThresholdBoundary(t *testing.T) {
	a := New()

	// Below threshold (1499ms): pattern should NOT trigger, Observe returns nil
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: "Continue? (y/n)",
		IdleMs:         1499,
		BytesSeen:      20,
	})
	if obs != nil {
		t.Fatalf("expected nil observation below idle threshold, got status=%v", obs.Status)
	}

	// At threshold (1500ms): pattern should trigger RunBlockedOnInput
	obs = a.Observe(adapter.CompletionContext{
		NormalizedTail: "Continue? (y/n)",
		IdleMs:         1500,
		BytesSeen:      20,
	})
	if obs == nil || obs.Status != model.RunBlockedOnInput {
		t.Fatalf("expected RunBlockedOnInput at threshold 1500ms, got %#v", obs)
	}
}

func TestObserve_PermissionPrompt(t *testing.T) {
	a := New()
	// Must match permissionPatterns (contains "permission") but NOT blockedPromptPatterns,
	// authPatterns, or rateLimitPatterns. The tail must also contain "?" for the check to fire.
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: "Waiting for permission approval?",
		IdleMs:         2000,
		BytesSeen:      40,
	})
	if obs == nil || obs.Status != model.RunPermissionPrompt {
		t.Fatalf("expected RunPermissionPrompt, got %#v", obs)
	}
}

func TestObserve_CrashWithNonZeroExit(t *testing.T) {
	a := New()
	exitCode := 1
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: "fatal error occurred",
		ExitCode:       &exitCode,
		BytesSeen:      20,
	})
	if obs == nil || obs.Status != model.RunCrashed {
		t.Fatalf("expected RunCrashed for non-zero exit, got %#v", obs)
	}
	if !obs.Terminal {
		t.Error("expected Terminal=true for crashed observation")
	}
}

func TestObserve_RunningWhenNoExitAndBelowThreshold(t *testing.T) {
	a := New()
	// No exit code, idle below threshold, bytes seen > 0 — returns nil (still running)
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: "Processing...",
		IdleMs:         100,
		BytesSeen:      15,
	})
	if obs != nil {
		t.Fatalf("expected nil (running), got %#v", obs)
	}
}

func TestObserve_ExitNoOutput(t *testing.T) {
	a := New()
	exitCode := 0
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: "",
		ExitCode:       &exitCode,
		BytesSeen:      0,
	})
	if obs == nil || obs.Status != model.RunPartial {
		t.Fatalf("expected RunPartial for exit with no output, got %#v", obs)
	}
}

func TestObserve_PromptReappearance(t *testing.T) {
	a := New()
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: "some output\n>\n",
		IdleMs:         2000,
		BytesSeen:      20,
	})
	if obs == nil || obs.Status != model.RunCompleted {
		t.Fatalf("expected RunCompleted on prompt reappearance, got %#v", obs)
	}
}

func TestObserve_IdleFallback(t *testing.T) {
	a := New()
	obs := a.Observe(adapter.CompletionContext{
		NormalizedTail: "some output without prompt",
		IdleMs:         5001,
		BytesSeen:      30,
	})
	if obs == nil || obs.Status != model.RunCompleted {
		t.Fatalf("expected RunCompleted from idle fallback, got %#v", obs)
	}
	if obs.Confidence > 0.5 {
		t.Errorf("expected low confidence for idle fallback, got %f", obs.Confidence)
	}
}

func TestObserveFromFixtures(t *testing.T) {
	exitZero := 0

	cases := []struct {
		fixture    string
		wantStatus model.RunStatus
		exitCode   *int
		idleMs     int64
	}{
		{
			fixture:    "happy_path.txt",
			wantStatus: model.RunCompleted,
			exitCode:   &exitZero,
			idleMs:     0,
		},
		{
			fixture:    "blocked_prompt.txt",
			wantStatus: model.RunBlockedOnInput,
			exitCode:   nil,
			idleMs:     2000,
		},
		{
			fixture:    "auth_required.txt",
			wantStatus: model.RunAuthRequired,
			exitCode:   nil,
			idleMs:     2000,
		},
		{
			fixture:    "rate_limited.txt",
			wantStatus: model.RunRateLimited,
			exitCode:   nil,
			idleMs:     2000,
		},
	}

	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
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
				t.Fatalf("fixture %s: expected status %v, got %v (reason: %s)", tc.fixture, tc.wantStatus, obs.Status, obs.Reason)
			}
		})
	}
}
