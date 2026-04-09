package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const maxSSEConnections = 10

func (s *Server) handleConversationStream(w http.ResponseWriter, r *http.Request) {
	if s.sseCount.Load() >= maxSSEConnections {
		http.Error(w, "too many SSE connections", http.StatusServiceUnavailable)
		return
	}
	s.sseCount.Add(1)
	defer s.sseCount.Add(-1)

	id := r.PathValue("id")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connected event so clients know the stream is alive.
	sendSSEEvent(w, "connected", map[string]string{"status": "connected", "conversationId": id})
	flusher.Flush()

	// Set up watcher only if a store root is configured.
	var events <-chan WatchEvent
	if s.cfg.StoreRoot != "" {
		watcher, err := NewWatcher(s.cfg.StoreRoot)
		if err == nil {
			defer watcher.Close()
			// Ignore error — dir may not exist yet; heartbeats will still flow.
			_ = watcher.WatchConversation(r.Context(), id)
			events = watcher.Events()
		}
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sendSSEComment(w, "heartbeat")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			sendSSEEvent(w, sseEventType(event), event)
			flusher.Flush()
		}
	}
}

func (s *Server) handleConversationsLifecycleStream(w http.ResponseWriter, r *http.Request) {
	if s.sseCount.Load() >= maxSSEConnections {
		http.Error(w, "too many SSE connections", http.StatusServiceUnavailable)
		return
	}
	s.sseCount.Add(1)
	defer s.sseCount.Add(-1)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sendSSEEvent(w, "connected", map[string]string{"status": "connected"})
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sendSSEComment(w, "heartbeat")
			flusher.Flush()
		}
	}
}

func sendSSEComment(w http.ResponseWriter, comment string) {
	fmt.Fprintf(w, ": %s\n\n", comment)
}

func sendSSEEvent(w http.ResponseWriter, eventType string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, b)
}

func sseEventType(e WatchEvent) string {
	switch e.Kind {
	case WatchEventConversationUpdated:
		return "conversation_updated"
	case WatchEventTurnAdded:
		return "turn_added"
	default:
		return "unknown"
	}
}
