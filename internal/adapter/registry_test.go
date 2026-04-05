package adapter_test

import (
	"strings"
	"testing"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

// stubAdapter is a minimal Adapter implementation for testing.
type stubAdapter struct {
	id string
}

func (s *stubAdapter) ID() string { return s.id }

func (s *stubAdapter) BuildSpawnSpec(cwd string, env map[string]string, homeDir string, cols, rows int) adapter.SpawnSpec {
	return adapter.SpawnSpec{}
}

func (s *stubAdapter) FormatPrompt(raw string) []byte { return []byte(raw) }

func (s *stubAdapter) Observe(ctx adapter.CompletionContext) *adapter.TranscriptObservation {
	return &adapter.TranscriptObservation{Status: model.RunRunning}
}

func (s *stubAdapter) ExtractResponse(rawTranscript []byte, normalizedTranscript string) adapter.ExtractionResult {
	return adapter.ExtractionResult{}
}

func TestNewRegistry_Empty(t *testing.T) {
	r := adapter.NewRegistry()
	_, err := r.Get("any-id")
	if err == nil {
		t.Fatal("expected error from Get on empty registry, got nil")
	}
}

func TestNewRegistry_WithAdapters(t *testing.T) {
	a1 := &stubAdapter{id: "provider-a"}
	a2 := &stubAdapter{id: "provider-b"}

	r := adapter.NewRegistry(a1, a2)

	got1, err := r.Get("provider-a")
	if err != nil {
		t.Fatalf("Get(provider-a): unexpected error: %v", err)
	}
	if got1.ID() != "provider-a" {
		t.Errorf("expected ID 'provider-a', got %q", got1.ID())
	}

	got2, err := r.Get("provider-b")
	if err != nil {
		t.Fatalf("Get(provider-b): unexpected error: %v", err)
	}
	if got2.ID() != "provider-b" {
		t.Errorf("expected ID 'provider-b', got %q", got2.ID())
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	r := adapter.NewRegistry(&stubAdapter{id: "exists"})

	const missingID = "does-not-exist"
	_, err := r.Get(missingID)
	if err == nil {
		t.Fatal("expected error for missing ID, got nil")
	}
	if !strings.Contains(err.Error(), missingID) {
		t.Errorf("expected error to contain %q, got: %v", missingID, err)
	}
}
