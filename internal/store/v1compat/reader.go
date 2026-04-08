// Package v1compat reads legacy v1 vitis conversation JSON and upgrades
// it in-memory to the v2 model shape. It is read-only by design — there
// is no v1 writer. Callers that want a v2 copy of an old session must
// re-run the conversation through the v2 broker.
//
// Detection: a v1 file lacks the schema_version field (or carries 0/1).
// A v2 file carries schema_version: 2. The Detect helper makes this
// explicit so callers can branch without parsing the whole document.
package v1compat

import (
	"encoding/json"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// Detect returns true if data is a v1-shaped conversation JSON
// (schema_version absent or < 2). It returns false for v2 files and
// for malformed JSON (caller should report parse errors normally).
func Detect(data []byte) bool {
	var head struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return false
	}
	return head.SchemaVersion < 2
}

// UpgradeConversation parses v1 conversation JSON and returns a v2-shaped
// model.Conversation. It is a no-op for v2 input — callers should branch
// on Detect or schema_version. The returned conversation has:
//
//   - SchemaVersion = 2
//   - Peers = [{id: "a", spec: PeerA}, {id: "b", spec: PeerB}]
//   - Seeds = {"a": SeedA, "b": SeedB}
//   - OpenerID = PeerID(Opener)
//
// All legacy fields (PeerA/B, SeedA/B, Opener) are retained on the
// returned struct so legacy consumers and tests still work.
func UpgradeConversation(data []byte) (model.Conversation, error) {
	var conv model.Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return model.Conversation{}, err
	}
	if conv.SchemaVersion >= 2 {
		// Already v2; nothing to upgrade.
		return conv, nil
	}
	conv.SchemaVersion = 2
	conv.Peers = []model.PeerParticipant{
		{ID: "a", Spec: conv.PeerA},
		{ID: "b", Spec: conv.PeerB},
	}
	conv.Seeds = map[model.PeerID]string{
		"a": conv.SeedA,
		"b": conv.SeedB,
	}
	conv.OpenerID = model.PeerID(conv.Opener)
	if conv.OpenerID == "" {
		conv.OpenerID = "a"
	}
	return conv, nil
}

// UpgradeTurns parses a slice of v1 conversation turns (either as a
// JSON array or already-decoded structs) and back-fills the v2 fields:
//
//   - FromID mirrors From
//   - ToID is the other peer in the 2-peer legacy model
//   - Reason is "opener" for index 1 (the seed turn) and "addressed"
//     for every subsequent turn — legacy v1 alternated strictly, so
//     every turn was effectively addressed via the implicit alternation
//     contract. FallbackUsed stays false; NextIDParsed stays nil.
//
// The Index field on legacy turns is 1-based; the opener is index 1.
func UpgradeTurns(turns []model.ConversationTurn) []model.ConversationTurn {
	out := make([]model.ConversationTurn, len(turns))
	for i, t := range turns {
		t.FromID = model.PeerID(t.From)
		if t.FromID == "a" {
			t.ToID = "b"
		} else if t.FromID == "b" {
			t.ToID = "a"
		}
		if t.Reason == "" {
			if i == 0 {
				t.Reason = model.TurnReasonOpener
			} else {
				t.Reason = model.TurnReasonAddressed
			}
		}
		out[i] = t
	}
	return out
}
