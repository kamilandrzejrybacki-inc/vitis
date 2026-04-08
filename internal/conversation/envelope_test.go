package conversation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func TestBuildEnvelopeTurn1IncludesBriefing(t *testing.T) {
	conv := model.Conversation{
		ID:         "conv-1",
		MaxTurns:   50,
		Terminator: model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
		Opener:     model.PeerSlotA,
		SeedA:      "Discuss X",
		SeedB:      "Discuss X",
	}
	env := BuildEnvelopeTurn1(conv, model.PeerSlotA, "TURN_END_abc123def456")

	require.Equal(t, "conv-1", env.ConversationID)
	require.Equal(t, 1, env.TurnIndex)
	require.Equal(t, 50, env.MaxTurns)
	require.Equal(t, model.PeerSlot("seed"), env.From) // synthetic "seed" sender on turn 1
	require.True(t, env.IncludeBriefing)
	require.NotEmpty(t, env.Briefing)
	require.Contains(t, env.Briefing, "peer-a")
	require.Contains(t, env.Briefing, "<<END>>")
	require.Contains(t, env.Body, "Discuss X")
	require.Contains(t, env.Body, "TURN_END_abc123def456")
	require.Contains(t, env.Body, "[conversation: conv-1  turn 1 of 50  from: seed]")
	// H1: briefing must be present in the body so PTY transports deliver it.
	require.Contains(t, env.Body, "peer-a")
	require.Contains(t, env.Body, "<<END>>")
}

func TestBuildEnvelopeTurnNOmitsBriefing(t *testing.T) {
	conv := model.Conversation{
		ID:       "conv-1",
		MaxTurns: 50,
	}
	env := BuildEnvelopeTurnN(conv, 3, model.PeerSlotA, "previous response", "TURN_END_xyz999000111")
	require.Equal(t, 3, env.TurnIndex)
	require.Equal(t, model.PeerSlotA, env.From)
	require.False(t, env.IncludeBriefing)
	require.Empty(t, env.Briefing)
	require.Contains(t, env.Body, "previous response")
	require.Contains(t, env.Body, "TURN_END_xyz999000111")
	require.Contains(t, env.Body, "[conversation: conv-1  turn 3 of 50  from: peer-a]")
}

func TestBuildEnvelopeRendersBriefingForBoth(t *testing.T) {
	conv := model.Conversation{
		ID:         "conv-1",
		MaxTurns:   10,
		Terminator: model.TerminatorSpec{Kind: "sentinel"},
		Opener:     model.PeerSlotA,
		SeedA:      "you are A",
		SeedB:      "you are B",
	}
	envA := BuildEnvelopeTurn1(conv, model.PeerSlotA, "TURN_END_aaaaaaaaaaaa")
	envB := BuildEnvelopeTurn1(conv, model.PeerSlotB, "TURN_END_bbbbbbbbbbbb")
	require.Contains(t, envA.Briefing, "peer-a")
	require.Contains(t, envB.Briefing, "peer-b")
	require.Contains(t, envA.Body, "you are A")
	require.Contains(t, envB.Body, "you are B")
}

func TestEnvelopeBodyEndsWithMarkerInstruction(t *testing.T) {
	conv := model.Conversation{ID: "conv-1", MaxTurns: 10}
	env := BuildEnvelopeTurnN(conv, 2, model.PeerSlotB, "hello", "TURN_END_xxxxxxxxxxxx")
	require.True(t, strings.HasSuffix(strings.TrimSpace(env.Body),
		"When you finish your reply, output the token TURN_END_xxxxxxxxxxxx on its own line."))
}

// P1-2 regression: the envelope body must end with a newline so that
// line-buffered peer stdin readers (mock-agent's bufio.ReadString('\n'))
// actually receive the final marker-instruction line.
func TestEnvelopeBodyEndsWithNewline(t *testing.T) {
	conv := model.Conversation{
		ID:         "conv-1",
		MaxTurns:   10,
		Terminator: model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
		SeedA:      "hi",
		SeedB:      "hi",
	}
	env1 := BuildEnvelopeTurn1(conv, model.PeerSlotA, "TURN_END_aaaaaaaaaaaa")
	require.True(t, strings.HasSuffix(env1.Body, "\n"), "turn-1 body must end with newline; got %q", env1.Body[len(env1.Body)-min(40, len(env1.Body)):])

	envN := BuildEnvelopeTurnN(conv, 2, model.PeerSlotA, "previous", "TURN_END_bbbbbbbbbbbb")
	require.True(t, strings.HasSuffix(envN.Body, "\n"), "turn-N body must end with newline; got %q", envN.Body[len(envN.Body)-min(40, len(envN.Body)):])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
