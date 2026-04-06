package terminal

import (
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func makeEvent(data []byte) model.StreamEvent {
	return model.StreamEvent{
		Timestamp: time.Now(),
		Kind:      model.StreamEventOutput,
		Data:      data,
	}
}

func TestTranscript_RecordExitAndExitCode(t *testing.T) {
	tr := NewTranscript(1024)

	tr.RecordExit(42)
	code := tr.ExitCode()
	if code == nil {
		t.Fatal("ExitCode() returned nil after RecordExit(42)")
	}
	if *code != 42 {
		t.Errorf("ExitCode() = %d, want 42", *code)
	}

	tr.RecordExit(0)
	code = tr.ExitCode()
	if code == nil {
		t.Fatal("ExitCode() returned nil after RecordExit(0)")
	}
	if *code != 0 {
		t.Errorf("ExitCode() = %d, want 0", *code)
	}
}

func TestTranscript_TailNormalized(t *testing.T) {
	tr := NewTranscript(1024)
	tr.Append(makeEvent([]byte("\x1b[32mhello\x1b[0m world")))

	got := tr.TailNormalized()
	want := "hello world"
	if got != want {
		t.Errorf("TailNormalized() = %q, want %q", got, want)
	}
}

func TestTranscript_BytesSeen(t *testing.T) {
	tr := NewTranscript(1024)
	tr.Append(makeEvent(make([]byte, 100)))
	tr.Append(makeEvent(make([]byte, 200)))

	if got := tr.BytesSeen(); got != 300 {
		t.Errorf("BytesSeen() = %d, want 300", got)
	}
}

func TestTranscript_IdleSince(t *testing.T) {
	tr := NewTranscript(1024)
	tr.Append(makeEvent([]byte("data")))

	time.Sleep(50 * time.Millisecond)

	idle := tr.IdleSince(time.Now())
	if idle < 50*time.Millisecond {
		t.Errorf("IdleSince() = %v, want >= 50ms", idle)
	}
}

func TestTranscript_HasOutput(t *testing.T) {
	tr := NewTranscript(1024)
	if tr.HasOutput() {
		t.Error("HasOutput() = true before any data, want false")
	}

	tr.Append(makeEvent([]byte("something")))
	if !tr.HasOutput() {
		t.Error("HasOutput() = false after appending data, want true")
	}
}

func TestTranscript_ZeroTailCapacity(t *testing.T) {
	// Zero or negative capacity should not panic and should use a reasonable default.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewTranscript(0) panicked: %v", r)
		}
	}()

	tr := NewTranscript(0)
	tr.Append(makeEvent([]byte("hello")))
	if len(tr.TailRaw()) == 0 {
		t.Error("TailRaw() returned empty after append with default capacity")
	}
}

func TestTranscript_TailExceedsCapacity(t *testing.T) {
	tr := NewTranscript(10)
	tr.Append(makeEvent(make([]byte, 100)))

	tail := tr.TailRaw()
	if len(tail) != 10 {
		t.Errorf("TailRaw() length = %d, want 10", len(tail))
	}
}

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
