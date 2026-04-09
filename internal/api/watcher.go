package api

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// WatchEventKind identifies what changed.
type WatchEventKind int

const (
	WatchEventConversationUpdated WatchEventKind = iota
	WatchEventTurnAdded
)

// WatchEvent is emitted when a watched file changes.
type WatchEvent struct {
	Kind           WatchEventKind
	ConversationID string
	FilePath       string
}

// Watcher watches conversation directories for file changes.
type Watcher struct {
	root    string
	fsw     *fsnotify.Watcher
	eventCh chan WatchEvent
}

// NewWatcher creates a Watcher rooted at dir (the store root).
func NewWatcher(root string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		root:    root,
		fsw:     fsw,
		eventCh: make(chan WatchEvent, 64),
	}
	go w.loop()
	return w, nil
}

// WatchConversation adds watch on conversations/<id>/ directory.
func (w *Watcher) WatchConversation(_ context.Context, conversationID string) error {
	dir := filepath.Join(w.root, "conversations", conversationID)
	return w.fsw.Add(dir)
}

// Events returns the channel of watch events.
func (w *Watcher) Events() <-chan WatchEvent {
	return w.eventCh
}

// Close stops the watcher.
func (w *Watcher) Close() error {
	return w.fsw.Close()
}

func (w *Watcher) loop() {
	defer close(w.eventCh)
	convDir := filepath.Join(w.root, "conversations")
	for {
		select {
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}
			base := filepath.Base(event.Name)
			// Extract conversation ID using filepath.Rel for robustness on all platforms.
			rel, err := filepath.Rel(convDir, filepath.Dir(event.Name))
			if err != nil || strings.Contains(rel, "..") {
				continue
			}
			convID := rel
			var kind WatchEventKind
			switch base {
			case "conversation.json":
				kind = WatchEventConversationUpdated
			case "turns.jsonl":
				kind = WatchEventTurnAdded
			default:
				continue
			}
			select {
			case w.eventCh <- WatchEvent{Kind: kind, ConversationID: convID, FilePath: event.Name}:
			default: // drop if buffer full
			}
		case _, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
		}
	}
}
