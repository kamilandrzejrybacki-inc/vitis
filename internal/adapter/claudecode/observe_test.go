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
