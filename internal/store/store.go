package store

import (
	"context"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

type Store interface {
	CreateSession(ctx context.Context, session model.Session) error
	UpdateSession(ctx context.Context, sessionID string, patch model.SessionPatch) error
	AppendTurn(ctx context.Context, turn model.Turn) error
	PeekTurns(ctx context.Context, sessionID string, lastN int) ([]model.Turn, error)
	AppendStreamEvent(ctx context.Context, event model.StoredStreamEvent) error

	// A2A conversation methods (additive — single-shot path is unaffected).
	CreateConversation(ctx context.Context, conv model.Conversation) error
	UpdateConversation(ctx context.Context, conversationID string, patch model.ConversationPatch) error
	AppendConversationTurn(ctx context.Context, turn model.ConversationTurn) error
	PeekConversationTurns(ctx context.Context, conversationID string, lastN int) ([]model.ConversationTurn, error)

	Close() error
}
