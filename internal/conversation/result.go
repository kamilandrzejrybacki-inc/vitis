package conversation

import "github.com/kamilandrzejrybacki-inc/vitis/internal/model"

// FinalResult is the JSON shape returned by `vitis converse` after a
// conversation reaches a terminal status. It bundles the conversation
// summary, the full turn log, a human-readable terminator note, and any
// warnings collected during the run.
type FinalResult struct {
	Conversation   model.Conversation       `json:"conversation"`
	Turns          []model.ConversationTurn `json:"turns"`
	TerminatorNote string                   `json:"terminator_note,omitempty"`
	Warnings       []string                 `json:"warnings,omitempty"`
}
