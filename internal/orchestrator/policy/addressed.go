package policy

import (
	"regexp"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// AddressedPolicy selects the next speaker from a <<NEXT: peer-id>> trailer
// on the last non-empty line of the reply. If the trailer is missing,
// unparseable, addresses an unknown peer, or addresses the current peer
// itself, the policy falls back to round-robin in the declared peer order.
//
// A <<END>> trailer on the last non-empty line takes precedence over any
// <<NEXT: ...>> trailer and causes parseNextTrailer to return nil (the
// orchestrator handles <<END>> separately as a terminator signal).
type AddressedPolicy struct{}

// NewAddressedPolicy returns a ready-to-use AddressedPolicy.
func NewAddressedPolicy() *AddressedPolicy { return &AddressedPolicy{} }

var (
	nextTrailerRegex = regexp.MustCompile(`^<<NEXT:\s*([a-z][a-z0-9_-]{0,31})\s*>>$`)
	endTrailerRegex  = regexp.MustCompile(`^<<END>>$`)
)

// parseNextTrailer returns the id captured from a <<NEXT: id>> trailer on
// the last non-empty line of reply. <<END>> on the last non-empty line
// returns nil so the orchestrator's sentinel path handles it.
func parseNextTrailer(reply string) *model.PeerID {
	lines := strings.Split(reply, "\n")
	var last string
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimRight(lines[i], " \t\r")
		if trimmed != "" {
			last = trimmed
			break
		}
	}
	if last == "" {
		return nil
	}
	if endTrailerRegex.MatchString(last) {
		return nil
	}
	m := nextTrailerRegex.FindStringSubmatch(last)
	if m == nil {
		return nil
	}
	id := model.PeerID(m[1])
	return &id
}

// Next implements TurnPolicy.
func (p *AddressedPolicy) Next(current model.PeerID, reply string, peers []model.PeerID) Decision {
	parsed := parseNextTrailer(reply)
	if parsed != nil && *parsed != current && contains(peers, *parsed) {
		return Decision{Next: *parsed, Parsed: parsed, FallbackUsed: false}
	}
	return Decision{
		Next:         roundRobinAfter(current, peers),
		Parsed:       parsed,
		FallbackUsed: true,
	}
}

func contains(peers []model.PeerID, id model.PeerID) bool {
	for _, p := range peers {
		if p == id {
			return true
		}
	}
	return false
}
