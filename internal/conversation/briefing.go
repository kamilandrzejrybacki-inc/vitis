package conversation

import (
	"fmt"
	"strings"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

// BriefingInput captures the per-peer information needed to render a turn-1
// briefing. The briefing is injected exactly once per peer (on its first
// envelope) and tells the peer:
//   - it is in a multi-turn conversation
//   - which slot it occupies (a or b)
//   - the maximum number of turns
//   - the terminator strategy (sentinel mode includes the <<END>> instruction;
//     judge mode omits it)
//   - the marker discipline (per-turn token must be emitted to end the turn)
//   - the optional reply style (caveman variants compress reply tokens)
type BriefingInput struct {
	Slot       model.PeerSlot
	MaxTurns   int
	Terminator model.TerminatorSpec
	Style      ReplyStyle
}

// RenderBriefing produces the system briefing text injected at the top of
// a peer's first envelope. Pure function; no side effects.
func RenderBriefing(in BriefingInput) string {
	var b strings.Builder
	b.WriteString("You are participating in a multi-turn conversation with another AI agent through Clank.\n")
	b.WriteString("The other agent's messages will be delivered to you as plain text wrapped in a header\n")
	b.WriteString("line indicating the turn number and sender. You should reply as if speaking to a\n")
	b.WriteString("collaborator.\n\n")
	fmt.Fprintf(&b, "You are: peer-%s.\n", string(in.Slot))
	fmt.Fprintf(&b, "Maximum turns in this conversation: %d.\n\n", in.MaxTurns)

	if in.Terminator.Kind == "sentinel" {
		sentinel := in.Terminator.Sentinel
		if sentinel == "" {
			sentinel = "<<END>>"
		}
		fmt.Fprintf(&b, "When you believe the conversation has reached its goal or natural end, end your final\n")
		fmt.Fprintf(&b, "reply with the literal token %s on its own line BEFORE the turn-end marker.\n\n", sentinel)
	}

	b.WriteString("After every reply, you MUST output a per-turn marker token as instructed in the\n")
	b.WriteString("incoming message. This marker tells the broker your turn is complete. If you forget\n")
	b.WriteString("the marker, your turn will time out.\n")

	if styleBlock := RenderStyleInstructions(in.Style); styleBlock != "" {
		b.WriteString(styleBlock)
	}

	return b.String()
}
