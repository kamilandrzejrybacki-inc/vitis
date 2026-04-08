package terminator

import (
	"context"
	"testing"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func TestNewSentinel_EmptyDefaults(t *testing.T) {
	s := NewSentinel("")
	if s.token != defaultSentinel {
		t.Errorf("expected default token, got %q", s.token)
	}
}

func TestNewSentinel_CustomToken(t *testing.T) {
	s := NewSentinel("DONE")
	if s.token != "DONE" {
		t.Errorf("expected DONE, got %q", s.token)
	}
}

func TestSentinel_StopBeforeStart(t *testing.T) {
	s := NewSentinel("END")
	if err := s.Stop(context.Background()); err != nil {
		t.Errorf("Stop before Start should be no-op: %v", err)
	}
}

func TestSentinel_StartIdempotent(t *testing.T) {
	s := NewSentinel("END")
	b := inproc.New()
	defer b.Close()
	conv := model.Conversation{ID: "c1"}
	ctx := context.Background()
	if err := s.Start(ctx, conv, b); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	// Second Start should silently no-op (returns nil) per current contract.
	if err := s.Start(ctx, conv, b); err != nil {
		t.Errorf("second Start should not error, got %v", err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestSentinel_StopIdempotent(t *testing.T) {
	s := NewSentinel("END")
	b := inproc.New()
	defer b.Close()
	conv := model.Conversation{ID: "c2"}
	ctx := context.Background()
	if err := s.Start(ctx, conv, b); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Errorf("second Stop should be no-op: %v", err)
	}
}

func TestContainsOnOwnLine_EmptyToken(t *testing.T) {
	if containsOnOwnLine("anything", "") {
		t.Error("empty token should never match")
	}
}

func TestContainsOnOwnLine_NotMatching(t *testing.T) {
	if containsOnOwnLine("hello world", "<<END>>") {
		t.Error("should not match")
	}
}

func TestContainsOnOwnLine_EmbeddedDoesntMatch(t *testing.T) {
	if containsOnOwnLine("prefix <<END>> suffix", "<<END>>") {
		t.Error("token embedded inline should not match")
	}
}

func TestContainsOnOwnLine_LeadingTrailingWhitespace(t *testing.T) {
	if !containsOnOwnLine("foo\n  <<END>>  \nbar", "<<END>>") {
		t.Error("token with surrounding whitespace should match")
	}
}

func TestStripSentinel_NoMatch(t *testing.T) {
	in := "no token here\nstill nothing"
	if got := StripSentinel(in, "<<END>>"); got != in {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestStripSentinel_RemovesLineAndAfter(t *testing.T) {
	in := "line1\nline2\n<<END>>\nleftover"
	got := StripSentinel(in, "<<END>>")
	want := "line1\nline2"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestStripSentinel_DefaultToken(t *testing.T) {
	got := StripSentinel("a\n<<END>>\nb", "")
	if got != "a" {
		t.Errorf("expected 'a', got %q", got)
	}
}
