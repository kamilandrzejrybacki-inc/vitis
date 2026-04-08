package conversation

import (
	"fmt"
	"strings"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

// BuildEnvelopeTurn1 constructs the very first envelope for a peer entering
// a conversation. It includes the per-peer briefing and the seed text. The
// turn index is 1; the synthetic sender label is "seed".
func BuildEnvelopeTurn1(conv model.Conversation, slot model.PeerSlot, marker string) model.Envelope {
	briefing := RenderBriefing(BriefingInput{
		Slot:       slot,
		MaxTurns:   conv.MaxTurns,
		Terminator: conv.Terminator,
		Style:      ReplyStyle(conv.ReplyStyle),
	})
	seed := seedFor(conv, slot)
	seedBody := renderBody(conv.ID, 1, conv.MaxTurns, "seed", seed, marker)
	// Prepend the briefing to the body so PTY transports deliver it in-band.
	body := briefing + "\n\n" + seedBody
	return model.Envelope{
		ConversationID:  conv.ID,
		TurnIndex:       1,
		MaxTurns:        conv.MaxTurns,
		From:            model.PeerSlot("seed"),
		Body:            body,
		MarkerToken:     marker,
		IncludeBriefing: true,
		Briefing:        briefing,
	}
}

// BuildEnvelopeTurnN constructs an envelope for turn N (N > 1). The body
// wraps the previous response with a header and the per-turn marker
// instruction. No briefing is included.
//
// The "from" header reflects whose turn it is now (i.e. the slot of the
// peer that produced the response we're delivering). The recipient is the
// other peer.
func BuildEnvelopeTurnN(conv model.Conversation, turnIndex int, from model.PeerSlot, previousResponse, marker string) model.Envelope {
	body := renderBody(conv.ID, turnIndex, conv.MaxTurns, "peer-"+string(from), previousResponse, marker)
	return model.Envelope{
		ConversationID: conv.ID,
		TurnIndex:      turnIndex,
		MaxTurns:       conv.MaxTurns,
		From:           from,
		Body:           body,
		MarkerToken:    marker,
	}
}

func seedFor(conv model.Conversation, slot model.PeerSlot) string {
	if slot == model.PeerSlotA {
		return conv.SeedA
	}
	return conv.SeedB
}

func renderBody(conversationID string, turnIndex, maxTurns int, fromLabel, content, marker string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[conversation: %s  turn %d of %d  from: %s]\n", conversationID, turnIndex, maxTurns, fromLabel)
	b.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n")
	// The trailing newline is REQUIRED: line-buffered peer stdin readers
	// (mock-agent's bufio.ReadString('\n'), interactive Claude/Codex sessions)
	// won't see the marker-instruction line until it is terminated.
	fmt.Fprintf(&b, "When you finish your reply, output the token %s on its own line.\n", marker)
	return b.String()
}
