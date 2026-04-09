package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

const maxLimit = 500

// isValidID returns false if the id looks like a path traversal attempt.
func isValidID(id string) bool {
	return id != "" && !strings.Contains(id, "/") && !strings.Contains(id, "..")
}

// ListResponse is the standard envelope for list endpoints.
type ListResponse[T any] struct {
	Success bool   `json:"success"`
	Data    []T    `json:"data"`
	Total   int    `json:"total"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	Error   string `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"success": false, "error": msg})
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > maxLimit {
				n = maxLimit
			}
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	filter := model.SessionFilter{Limit: limit, Offset: offset}
	if v := r.URL.Query().Get("status"); v != "" {
		rs := model.RunStatus(v)
		filter.Status = &rs
	}
	sessions, total, err := s.store.ListSessions(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sessions == nil {
		sessions = []model.Session{}
	}
	writeJSON(w, http.StatusOK, ListResponse[model.Session]{
		Success: true, Data: sessions, Total: total, Limit: limit, Offset: offset,
	})
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	sess, err := s.store.GetSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sess == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": sess})
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	filter := model.ConversationFilter{Limit: limit, Offset: offset}
	if v := r.URL.Query().Get("status"); v != "" {
		cs := model.ConversationStatus(v)
		filter.Status = &cs
	}
	convs, total, err := s.store.ListConversations(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if convs == nil {
		convs = []model.Conversation{}
	}
	writeJSON(w, http.StatusOK, ListResponse[model.Conversation]{
		Success: true, Data: convs, Total: total, Limit: limit, Offset: offset,
	})
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	conv, err := s.store.GetConversation(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conv == nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": conv})
}

func (s *Server) handleListTurns(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	turns, err := s.store.PeekConversationTurns(r.Context(), id, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if turns == nil {
		turns = []model.ConversationTurn{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    turns,
		"total":   len(turns),
	})
}
