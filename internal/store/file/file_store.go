package file

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

type Store struct {
	root     string
	debugRaw bool
	mu       sync.Mutex
}

func New(root string, debugRaw bool) (*Store, error) {
	if root == "" {
		return nil, fmt.Errorf("missing log path")
	}
	s := &Store{root: root, debugRaw: debugRaw}
	for _, dir := range []string{s.sessionsDir(), s.turnsDir(), s.rawDir()} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	return s, nil
}

func (s *Store) CreateSession(_ context.Context, session model.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSONAtomic(s.sessionPath(session.ID), session)
}

func (s *Store) UpdateSession(_ context.Context, sessionID string, patch model.SessionPatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var session model.Session
	if err := s.readJSON(s.sessionPath(sessionID), &session); err != nil {
		return err
	}

	if patch.Status != nil {
		session.Status = *patch.Status
	}
	if patch.EndedAt != nil {
		session.EndedAt = patch.EndedAt
	}
	if patch.DurationMs != nil {
		session.DurationMs = patch.DurationMs
	}
	if patch.ExitCode != nil {
		session.ExitCode = patch.ExitCode
	}
	if patch.ParserConfidence != nil {
		session.ParserConfidence = patch.ParserConfidence
	}
	if patch.ObservationConfidence != nil {
		session.ObservationConfidence = patch.ObservationConfidence
	}
	if patch.AuthMode != nil {
		session.AuthMode = *patch.AuthMode
	}
	if patch.BlockedReason != nil {
		session.BlockedReason = patch.BlockedReason
	}
	if patch.BytesCaptured != nil {
		session.BytesCaptured = patch.BytesCaptured
	}
	if patch.Warnings != nil {
		session.Warnings = append([]string(nil), patch.Warnings...)
	}
	if patch.TerminalCols != nil {
		session.TerminalCols = patch.TerminalCols
	}
	if patch.TerminalRows != nil {
		session.TerminalRows = patch.TerminalRows
	}

	return s.writeJSONAtomic(s.sessionPath(sessionID), session)
}

func (s *Store) AppendTurn(_ context.Context, turn model.Turn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendJSONL(s.turnPath(turn.SessionID), turn)
}

func (s *Store) PeekTurns(_ context.Context, sessionID string, lastN int) ([]model.Turn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.turnPath(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open turns: %w", err)
	}
	defer file.Close()

	var turns []model.Turn
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var turn model.Turn
		if err := json.Unmarshal(scanner.Bytes(), &turn); err != nil {
			return nil, fmt.Errorf("decode turn: %w", err)
		}
		turns = append(turns, turn)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan turns: %w", err)
	}

	sort.Slice(turns, func(i, j int) bool { return turns[i].Index < turns[j].Index })
	if lastN > 0 && len(turns) > lastN {
		turns = turns[len(turns)-lastN:]
	}
	return turns, nil
}

func (s *Store) AppendStreamEvent(_ context.Context, event model.StoredStreamEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.debugRaw {
		return nil
	}
	entry := struct {
		SessionID string                `json:"session_id"`
		Timestamp string                `json:"timestamp"`
		Kind      model.StreamEventKind `json:"kind"`
		Data      string                `json:"data_base64"`
	}{
		SessionID: event.SessionID,
		Timestamp: event.Timestamp.UTC().Format(time.RFC3339Nano),
		Kind:      event.Kind,
		Data:      base64.StdEncoding.EncodeToString(event.Data),
	}
	return s.appendJSONL(s.rawPath(event.SessionID), entry)
}

func (s *Store) Close() error { return nil }

func (s *Store) sessionPath(id string) string { return filepath.Join(s.sessionsDir(), id+".json") }
func (s *Store) turnPath(id string) string    { return filepath.Join(s.turnsDir(), id+".jsonl") }
func (s *Store) rawPath(id string) string     { return filepath.Join(s.rawDir(), id+".jsonl") }
func (s *Store) sessionsDir() string          { return filepath.Join(s.root, "sessions") }
func (s *Store) turnsDir() string             { return filepath.Join(s.root, "turns") }
func (s *Store) rawDir() string               { return filepath.Join(s.root, "raw") }

func (s *Store) writeJSONAtomic(path string, value any) error {
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func (s *Store) readJSON(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read json: %w", err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	return nil
}

func (s *Store) appendJSONL(path string, value any) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open jsonl: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal jsonl: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append jsonl: %w", err)
	}
	return nil
}
