package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter/claudecode"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/terminal"
)

type fakeStore struct {
	sessions map[string]model.Session
	turns    []model.Turn
}

func newFakeStore() *fakeStore {
	return &fakeStore{sessions: map[string]model.Session{}}
}

func (s *fakeStore) CreateSession(_ context.Context, session model.Session) error {
	s.sessions[session.ID] = session
	return nil
}

func (s *fakeStore) UpdateSession(_ context.Context, sessionID string, patch model.SessionPatch) error {
	return nil
}
func (s *fakeStore) AppendTurn(_ context.Context, turn model.Turn) error {
	s.turns = append(s.turns, turn)
	return nil
}
func (s *fakeStore) PeekTurns(_ context.Context, sessionID string, lastN int) ([]model.Turn, error) {
	return s.turns, nil
}
func (s *fakeStore) AppendStreamEvent(_ context.Context, event model.StoredStreamEvent) error {
	return nil
}
func (s *fakeStore) Close() error { return nil }

type fakeRuntime struct {
	process terminal.PseudoTerminalProcess
}

func (r *fakeRuntime) Spawn(spec adapter.SpawnSpec) (terminal.PseudoTerminalProcess, error) {
	return r.process, nil
}

type fakeProcess struct {
	output chan model.StreamEvent
	done   chan model.ExitResult
}

func newFakeProcess() *fakeProcess {
	return &fakeProcess{
		output: make(chan model.StreamEvent, 4),
		done:   make(chan model.ExitResult, 1),
	}
}

func (p *fakeProcess) Write(data []byte) (int, error) {
	p.output <- model.StreamEvent{Timestamp: time.Now(), Kind: model.StreamEventOutput, Data: []byte("answer\n")}
	p.done <- model.ExitResult{Code: 0}
	close(p.done)
	close(p.output)
	return len(data), nil
}

func (p *fakeProcess) Output() <-chan model.StreamEvent  { return p.output }
func (p *fakeProcess) Done() <-chan model.ExitResult     { return p.done }
func (p *fakeProcess) Terminate(gracePeriodMs int) error { return nil }

// errorRuntime returns an error from Spawn, simulating a missing/nonexistent binary.
type errorRuntime struct{}

func (e *errorRuntime) Spawn(_ adapter.SpawnSpec) (terminal.PseudoTerminalProcess, error) {
	return nil, fmt.Errorf("spawn: executable not found")
}

// blockingProcess never sends on Done() or Output(), simulating a hung process.
type blockingProcess struct {
	output chan model.StreamEvent
	done   chan model.ExitResult
}

func newBlockingProcess() *blockingProcess {
	return &blockingProcess{
		output: make(chan model.StreamEvent),
		done:   make(chan model.ExitResult),
	}
}

func (p *blockingProcess) Write(_ []byte) (int, error)   { return 0, nil }
func (p *blockingProcess) Output() <-chan model.StreamEvent { return p.output }
func (p *blockingProcess) Done() <-chan model.ExitResult  { return p.done }
func (p *blockingProcess) Terminate(_ int) error          { return nil }

func TestRunProviderNotFound(t *testing.T) {
	store := newFakeStore()
	process := newFakeProcess()
	deps := Dependencies{
		Adapters: adapter.NewRegistry(claudecode.New()),
		Runtime:  &fakeRuntime{process: process},
		Store:    store,
	}
	result, err := Run(context.Background(), model.RunRequest{
		Provider: "nonexistent-provider",
		Prompt:   "hello",
	}, deps)
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if !strings.Contains(err.Error(), "nonexistent-provider") {
		t.Fatalf("expected error to mention provider name; got: %v", err)
	}
	runErr, ok := err.(*model.RunError)
	if !ok {
		t.Fatalf("expected *model.RunError, got %T", err)
	}
	if runErr.Code != model.ErrorProvider {
		t.Fatalf("expected error code %s, got %s", model.ErrorProvider, runErr.Code)
	}
}

func TestRunSpawnFailure(t *testing.T) {
	store := newFakeStore()
	deps := Dependencies{
		Adapters: adapter.NewRegistry(claudecode.New()),
		Runtime:  &errorRuntime{},
		Store:    store,
	}
	result, err := Run(context.Background(), model.RunRequest{
		Provider: "claude-code",
		Prompt:   "hello",
	}, deps)
	if err == nil {
		t.Fatal("expected error when spawn fails, got nil")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	runErr, ok := err.(*model.RunError)
	if !ok {
		t.Fatalf("expected *model.RunError, got %T", err)
	}
	if runErr.Code != model.ErrorSpawn {
		t.Fatalf("expected error code %s, got %s", model.ErrorSpawn, runErr.Code)
	}
}

func TestRunTimeout(t *testing.T) {
	store := newFakeStore()
	process := newBlockingProcess()
	deps := Dependencies{
		Adapters: adapter.NewRegistry(claudecode.New()),
		Runtime:  &fakeRuntime{process: process},
		Store:    store,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	result, err := Run(ctx, model.RunRequest{
		Provider:   "claude-code",
		Prompt:     "hello",
		TimeoutSec: 600,
	}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Status != model.RunTimeout {
		t.Fatalf("expected status %s, got %s", model.RunTimeout, result.Status)
	}
}

func TestRunHappyPath(t *testing.T) {
	store := newFakeStore()
	process := newFakeProcess()
	deps := Dependencies{
		Adapters: adapter.NewRegistry(claudecode.New()),
		Runtime:  &fakeRuntime{process: process},
		Store:    store,
	}
	result, err := Run(context.Background(), model.RunRequest{
		Provider:   "claude-code",
		Prompt:     "hello",
		LogBackend: "file",
	}, deps)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != model.RunCompleted {
		t.Fatalf("unexpected status: %s", result.Status)
	}
	if result.Response != "answer" {
		t.Fatalf("unexpected response: %q", result.Response)
	}
}
