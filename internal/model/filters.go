// internal/model/filters.go
package model

// SessionFilter controls listing of stored sessions.
type SessionFilter struct {
	Status *RunStatus
	Limit  int
	Offset int
}

// ConversationFilter controls listing of stored conversations.
type ConversationFilter struct {
	Status *ConversationStatus
	Limit  int
	Offset int
}
