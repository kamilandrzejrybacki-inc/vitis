package store

import "github.com/kamilandrzejrybacki-inc/clank/internal/model"

type Store interface {
	CreateSession(session model.Session) error
	UpdateSession(sessionID string, patch model.SessionPatch) error
	AppendTurn(turn model.Turn) error
	PeekTurns(sessionID string, lastN int) ([]model.Turn, error)
	AppendStreamEvent(event model.StoredStreamEvent) error
	Close() error
}
