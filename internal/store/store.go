package store

import (
	"context"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

type Store interface {
	CreateSession(ctx context.Context, session model.Session) error
	UpdateSession(ctx context.Context, sessionID string, patch model.SessionPatch) error
	AppendTurn(ctx context.Context, turn model.Turn) error
	PeekTurns(ctx context.Context, sessionID string, lastN int) ([]model.Turn, error)
	AppendStreamEvent(ctx context.Context, event model.StoredStreamEvent) error
	Close() error
}
