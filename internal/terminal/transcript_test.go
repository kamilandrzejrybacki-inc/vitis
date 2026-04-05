package terminal

import (
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func TestTranscriptPreservesRawAndNormalized(t *testing.T) {
	tr := NewTranscript(8)
	tr.Append(model.StreamEvent{
		Timestamp: time.Now(),
		Kind:      model.StreamEventOutput,
		Data:      []byte("\x1b[31mhello\x1b[0m\n"),
	})

	if got := string(tr.Raw()); got != "\x1b[31mhello\x1b[0m\n" {
		t.Fatalf("raw mismatch: %q", got)
	}
	if got := tr.Normalized(); got != "hello\n" {
		t.Fatalf("normalized mismatch: %q", got)
	}
}

func TestTranscriptTailWraps(t *testing.T) {
	tr := NewTranscript(4)
	tr.Append(model.StreamEvent{Timestamp: time.Now(), Kind: model.StreamEventOutput, Data: []byte("hello")})
	if got := string(tr.TailRaw()); got != "ello" {
		t.Fatalf("tail mismatch: %q", got)
	}
}
