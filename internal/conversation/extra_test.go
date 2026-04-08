package conversation

import (
	"strings"
	"testing"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func TestContainsMarker_EmptyArgs(t *testing.T) {
	if ContainsMarker("", "tok") {
		t.Error("empty body should not match")
	}
	if ContainsMarker("body", "") {
		t.Error("empty token should not match")
	}
}

func TestStripMarkerAndAfter_EmptyToken(t *testing.T) {
	got, found := StripMarkerAndAfter("body", "")
	if found {
		t.Error("empty token must not signal found")
	}
	if got != "body" {
		t.Errorf("expected unchanged body, got %q", got)
	}
}

func TestRenderBriefing_SentinelMode(t *testing.T) {
	out := RenderBriefing(BriefingInput{
		Slot:       model.PeerSlotA,
		MaxTurns:   10,
		Terminator: model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
	})
	if !strings.Contains(out, "<<END>>") {
		t.Errorf("expected sentinel instruction in briefing")
	}
	if !strings.Contains(out, "peer-a") {
		t.Errorf("expected peer-a in briefing")
	}
}

func TestRenderBriefing_JudgeModeOmitsSentinel(t *testing.T) {
	out := RenderBriefing(BriefingInput{
		Slot:       model.PeerSlotB,
		MaxTurns:   5,
		Terminator: model.TerminatorSpec{Kind: "judge"},
	})
	if strings.Contains(out, "<<END>>") {
		t.Errorf("judge mode should omit <<END>> instruction; got %q", out)
	}
	if !strings.Contains(out, "peer-b") {
		t.Errorf("expected peer-b in briefing")
	}
}

func TestRenderBriefing_SentinelDefaultToken(t *testing.T) {
	out := RenderBriefing(BriefingInput{
		Slot:       model.PeerSlotA,
		MaxTurns:   3,
		Terminator: model.TerminatorSpec{Kind: "sentinel"}, // empty Sentinel
	})
	if !strings.Contains(out, "<<END>>") {
		t.Errorf("expected default <<END>> token in briefing")
	}
}

func TestDrainControlTimed_Timeout(t *testing.T) {
	ch := make(chan bus.BusMessage)
	got := drainControlTimed(ch, 20*time.Millisecond)
	if len(got) != 0 {
		t.Errorf("expected empty result on timeout, got %d", len(got))
	}
}

func TestDrainControlTimed_ClosedChannel(t *testing.T) {
	ch := make(chan bus.BusMessage)
	close(ch)
	got := drainControlTimed(ch, 50*time.Millisecond)
	if len(got) != 0 {
		t.Errorf("expected empty result on closed channel, got %d", len(got))
	}
}
