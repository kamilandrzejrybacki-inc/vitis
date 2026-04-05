package claudecode

import (
	"testing"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

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
