package orchestrator

import (
	"context"
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

func (s *fakeStore) CreateSession(session model.Session) error {
	s.sessions[session.ID] = session
	return nil
}

func (s *fakeStore) UpdateSession(sessionID string, patch model.SessionPatch) error { return nil }
func (s *fakeStore) AppendTurn(turn model.Turn) error {
	s.turns = append(s.turns, turn)
	return nil
}
func (s *fakeStore) PeekTurns(sessionID string, lastN int) ([]model.Turn, error) { return s.turns, nil }
func (s *fakeStore) AppendStreamEvent(event model.StoredStreamEvent) error       { return nil }
func (s *fakeStore) Close() error                                                { return nil }

type fakeRuntime struct {
	process *fakeProcess
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
