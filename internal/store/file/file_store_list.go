package file

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func (s *Store) ListSessions(_ context.Context, filter model.SessionFilter) ([]model.Session, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, nil
		}
		return nil, 0, err
	}

	var all []model.Session
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			continue
		}
		var sess model.Session
		if jsonErr := json.Unmarshal(data, &sess); jsonErr != nil {
			continue
		}
		if filter.Status != nil && sess.Status != *filter.Status {
			continue
		}
		all = append(all, sess)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].StartedAt.After(all[j].StartedAt)
	})

	total := len(all)
	if filter.Offset >= len(all) {
		return nil, total, nil
	}
	if filter.Offset > 0 {
		all = all[filter.Offset:]
	}
	if filter.Limit > 0 && len(all) > filter.Limit {
		all = all[:filter.Limit]
	}

	return all, total, nil
}

func (s *Store) ListConversations(_ context.Context, filter model.ConversationFilter) ([]model.Conversation, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.conversationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, nil
		}
		return nil, 0, err
	}

	var all []model.Conversation
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			continue
		}
		var conv model.Conversation
		if jsonErr := json.Unmarshal(data, &conv); jsonErr != nil {
			continue
		}
		if filter.Status != nil && conv.Status != *filter.Status {
			continue
		}
		all = append(all, conv)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	total := len(all)
	if filter.Offset >= len(all) {
		return nil, total, nil
	}
	if filter.Offset > 0 {
		all = all[filter.Offset:]
	}
	if filter.Limit > 0 && len(all) > filter.Limit {
		all = all[:filter.Limit]
	}

	return all, total, nil
}

func (s *Store) GetSession(_ context.Context, sessionID string) (*model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.sessionPath(sessionID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var sess model.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) GetConversation(_ context.Context, conversationID string) (*model.Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.conversationPath(conversationID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var conv model.Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, err
	}
	return &conv, nil
}
