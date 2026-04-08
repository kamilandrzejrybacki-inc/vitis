# Vitis N-Peer Conversations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `vitis converse` from 2-peer strict alternation to N-peer (2–16) conversations with addressed routing via `<<NEXT: peer-id>>` trailer and round-robin fallback, preserving full back-compat for existing `--peer-a/b` scripts.

**Architecture:** Introduce a stable `PeerID` string type alongside the existing `PeerSlot` enum. Extend the bus topic conventions to key by peer id. Rewrite the orchestrator turn loop around a `TurnPolicy` interface, shipping `AddressedPolicy` as the sole v1 implementation. Cut persistence over to schema v2 (peer-id-keyed) with a read-only v1 compat reader for `vitis peek`. The CLI grows a repeatable `--peer id=...,provider=...` flag and a repeatable `--seed id=...,content="..."` flag, with a translation layer that folds `--peer-a/b`, `--peer-a/b-opt`, `--seed-a/b`, and `--opener a|b` into the new shape.

**Tech Stack:** Go 1.22+, Cobra CLI, existing `internal/bus`, `internal/store/{file,postgres}`, `internal/orchestrator`, `internal/conversation`, `internal/peer`, `internal/terminator`, `internal/cli`, `mockagent/`.

---

## Spec-to-codebase reconciliation

The design spec (`2026-04-08-vitis-n-peer-conversations-design.md`) was written with generic package names. The actual codebase uses these paths, which override the spec where they differ:

| Spec says | Reality in this repo |
|---|---|
| `internal/bus/` is new | Already exists at `internal/bus/` (`bus.go`, `inproc/`, with NATS backend). We **extend** it, not create it. |
| `internal/persistence/` | Use `internal/store/` (with `file/` and `postgres/` subpackages). |
| `internal/persistence/v1compat/` | Use `internal/store/v1compat/`. |
| `internal/conversation/envelope.go` is the Envelope type | Envelope is declared in `internal/model/envelope.go`. `internal/conversation/envelope.go` is the builder. We modify both. |
| "Scheduler" package | Put `TurnPolicy` + addressed policy + loop under `internal/orchestrator/policy/` (new subpackage of the existing orchestrator). The orchestrator already owns the loop. |
| New bus `Envelope{FromID, ToID}` | Extend the existing `BusMessage` topic routing; add peer-id-keyed topic helpers alongside the slot-keyed ones. Keep the old helpers working during transition. |

The PeerSlot enum stays in `internal/model` during all phases. A new `PeerID` string type is introduced alongside it. The orchestrator, store, and bus gain peer-id-aware code paths. The CLI translates old slot-based flags into `PeerID` values `"a"` and `"b"`. Existing tests keep passing because the alias layer produces byte-identical behavior for 2-peer inputs.

## File structure

### New files

- `internal/model/peer_id.go` — `type PeerID string`, validation regex `^[a-z][a-z0-9_-]{0,31}$`, `Validate()`, `String()`, helpers.
- `internal/model/peer_id_test.go` — unit tests for validation and round-trip JSON.
- `internal/orchestrator/policy/policy.go` — `TurnPolicy` interface and common types.
- `internal/orchestrator/policy/addressed.go` — `AddressedPolicy` implementation.
- `internal/orchestrator/policy/addressed_test.go` — trailer parser table tests.
- `internal/orchestrator/policy/roundrobin.go` — round-robin helper used by fallback.
- `internal/store/v1compat/reader.go` — read-only v1 → v2 upgrader.
- `internal/store/v1compat/reader_test.go` — golden v1 fixture tests.
- `internal/store/v1compat/testdata/` — frozen v1 conversation fixtures.
- `tests/manual/13_converse_three_peer_addressed.sh`
- `tests/manual/14_converse_three_peer_fallback.sh`
- `tests/manual/15_converse_backcompat_aliases.sh`
- `docs/superpowers/adrs/2026-04-08-n-peer-addressed-routing.md` — ADR capturing decisions from the spec.

### Modified files

- `internal/model/model.go` — `Conversation` gains `Peers []PeerParticipant`, `Seeds map[PeerID]string`, `OpenerID PeerID`, `SchemaVersion int`; keeps `PeerA/B/SeedA/B/Opener` as deprecated mirrors, populated only on v1 read or back-compat CLI path.
- `internal/model/model.go` — `ConversationTurn` gains `FromID PeerID`, `ToID PeerID`, `Reason TurnReason`, `NextIDParsed *PeerID`, `FallbackUsed bool`.
- `internal/model/envelope.go` — `Envelope` gains `FromID PeerID`, `ToID PeerID`; `From PeerSlot` retained for back-compat write path.
- `internal/bus/bus.go` — new `TopicEnvelopeInID(convID, peerID)`, `TopicTurnID(convID)` (turn fan-out is already per-conv), kept slot helpers as wrappers.
- `internal/bus/inproc/inproc.go` — no code change expected (topic-agnostic); add tests that exercise peer-id topics.
- `internal/conversation/envelope.go` — `BuildEnvelopeTurnN` takes `PeerID` from and writes both `FromID` and the legacy `From` field; seed lookup goes through `Conversation.Seeds`.
- `internal/conversation/briefing.go` — briefing template takes `PeerID` (falls back to slot-equivalent for display).
- `internal/orchestrator/orchestrator.go` — turn loop delegates to `policy.TurnPolicy.Next(...)` for next-peer selection; publishes envelopes on `TopicEnvelopeInID`; records enriched turns.
- `internal/orchestrator/completion_loop.go` — per-peer tracking moves from slot-keyed map to `map[PeerID]...`.
- `internal/store/file/file.go` — writer emits `schema_version: 2` header + enriched turn records; reader detects v1 and delegates to `v1compat`.
- `internal/store/postgres/postgres.go` — schema migration to add `from_id`, `to_id`, `reason`, `next_id_parsed`, `fallback_used`, `schema_version`; writer populates new columns; reader unchanged beyond the new fields.
- `internal/cli/converse.go` — repeatable `--peer`, `--seed`, `--opener <id>`; `--peer-a/b`, `--peer-a/b-opt`, `--seed-a/b`, `--opener a|b` translated to the new shape; full validation block.
- `internal/cli/converse_test.go` — new cases for repeatable flags, alias translation, validation errors, parser edge cases.
- `internal/peer/provider/...` — per-peer log paths become `logs/<conv_id>/<peer_id>/...` (currently `logs/<conv_id>/peer-<slot>/...`).
- `mockagent/main.go` — `--next-trailer <id>`, `--no-trailer`, `--bad-trailer <value>` directives.
- `README.md` + `docs/` — N-peer guide, migration note, flag reference update.

---

## Phase 1 — PeerID type + bus topic extensions

**Files:**
- Create: `internal/model/peer_id.go`
- Create: `internal/model/peer_id_test.go`
- Modify: `internal/bus/bus.go`
- Modify: `internal/bus/inproc/inproc_test.go` (add peer-id topic tests)

### Task 1.1: Add `PeerID` type

- [ ] **Step 1: Write the failing test**

`internal/model/peer_id_test.go`:
```go
package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPeerIDValidate(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		name string
	}{
		{"a", true, "single letter"},
		{"alice", true, "simple word"},
		{"peer-1", true, "hyphen and digit"},
		{"p_q_r", true, "underscores"},
		{"", false, "empty"},
		{"A", false, "uppercase"},
		{"1peer", false, "leading digit"},
		{"-peer", false, "leading hyphen"},
		{"peer!", false, "invalid char"},
		{"this_id_is_way_too_long_to_be_accepted_as_a_peer_identifier", false, "too long"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := PeerID(c.in).Validate()
			if c.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestPeerIDFromSlot(t *testing.T) {
	require.Equal(t, PeerID("a"), PeerIDFromSlot(PeerSlotA))
	require.Equal(t, PeerID("b"), PeerIDFromSlot(PeerSlotB))
}
```

- [ ] **Step 2: Verify it fails**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/model/ -run TestPeerID -v`
Expected: FAIL with undefined `PeerID`, `PeerIDFromSlot`.

- [ ] **Step 3: Implement**

`internal/model/peer_id.go`:
```go
package model

import (
	"errors"
	"regexp"
)

// PeerID is a stable identifier for a peer within a conversation.
// It is referenced in <<NEXT: peer-id>> trailers and in the bus topic
// layout. PeerID is case-sensitive and must match the validation regex.
type PeerID string

var peerIDRegex = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// ErrInvalidPeerID is returned by PeerID.Validate for ids that do not
// match the required pattern.
var ErrInvalidPeerID = errors.New("invalid peer id: must match ^[a-z][a-z0-9_-]{0,31}$")

// Validate returns nil if the id is well-formed.
func (p PeerID) Validate() error {
	if !peerIDRegex.MatchString(string(p)) {
		return ErrInvalidPeerID
	}
	return nil
}

// String returns the id as a plain string.
func (p PeerID) String() string { return string(p) }

// PeerIDFromSlot maps a legacy PeerSlot to the equivalent PeerID ("a" or "b").
// This supports the back-compat alias path in the CLI.
func PeerIDFromSlot(s PeerSlot) PeerID { return PeerID(s) }
```

- [ ] **Step 4: Verify test passes**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/model/ -run TestPeerID -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis && git add internal/model/peer_id.go internal/model/peer_id_test.go && git commit -m "feat(model): add PeerID type with validation"
```

### Task 1.2: Add peer-id-keyed bus topic helpers

- [ ] **Step 1: Write the failing test**

Append to `internal/bus/inproc/inproc_test.go` (or create a new `peer_id_topics_test.go` in `internal/bus/`):
```go
package bus

import (
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/require"
)

func TestTopicEnvelopeInID(t *testing.T) {
	require.Equal(t, "conv/c1/peer/alice/in", TopicEnvelopeInID("c1", model.PeerID("alice")))
}

func TestTopicEnvelopeInIDSlotParity(t *testing.T) {
	// Back-compat: the legacy slot-keyed helper and the new id-keyed helper
	// agree for the reserved slot ids "a" and "b" via the aliasing layer,
	// but emit distinguishable topics so subscribers can opt in.
	slotTopic := TopicEnvelopeIn("c1", model.PeerSlotA)
	idTopic := TopicEnvelopeInID("c1", model.PeerID("a"))
	require.Equal(t, "conv/c1/peer-a/in", slotTopic)
	require.Equal(t, "conv/c1/peer/a/in", idTopic)
}
```

- [ ] **Step 2: Verify it fails**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/bus/ -run TopicEnvelopeInID -v`
Expected: FAIL (`TopicEnvelopeInID` undefined).

- [ ] **Step 3: Implement**

Append to `internal/bus/bus.go`:
```go
// TopicEnvelopeInID returns the inbox topic for a peer identified by PeerID.
// This is the N-peer replacement for TopicEnvelopeIn; both coexist while the
// orchestrator migrates.
func TopicEnvelopeInID(conversationID string, peerID model.PeerID) string {
	return "conv/" + conversationID + "/peer/" + string(peerID) + "/in"
}
```

- [ ] **Step 4: Verify test passes**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/bus/ -v`
Expected: PASS, including existing tests.

- [ ] **Step 5: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis && git add internal/bus/bus.go internal/bus/peer_id_topics_test.go && git commit -m "feat(bus): add peer-id-keyed envelope topic helper"
```

### Task 1.3: Phase 1 gate — build + full test suite green

- [ ] **Step 1: Build**

Run: `cd /home/kamil-rybacki/Code/vitis && go build ./...`
Expected: no errors.

- [ ] **Step 2: Full test suite**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./...`
Expected: all green.

- [ ] **Step 3: Vet**

Run: `cd /home/kamil-rybacki/Code/vitis && go vet ./...`
Expected: no warnings.

---

## Phase 2 — Envelope + Conversation model schema v2

**Files:**
- Modify: `internal/model/envelope.go`
- Modify: `internal/model/model.go`
- Modify: `internal/model/*_test.go` (add v2 round-trip tests)

### Task 2.1: Extend `Envelope` with peer-id fields

- [ ] **Step 1: Write the failing test**

Append to `internal/model/envelope_test.go` (create if missing):
```go
package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvelopeV2JSONRoundTrip(t *testing.T) {
	env := Envelope{
		ConversationID: "c1",
		TurnIndex:      3,
		MaxTurns:       10,
		From:           PeerSlotA,      // legacy
		FromID:         PeerID("alice"),
		ToID:           PeerID("bob"),
		Body:           "hello",
		MarkerToken:    "<<END_T_3>>",
	}
	data, err := json.Marshal(env)
	require.NoError(t, err)

	var decoded Envelope
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, env.FromID, decoded.FromID)
	require.Equal(t, env.ToID, decoded.ToID)
	require.Equal(t, env.From, decoded.From)
}
```

- [ ] **Step 2: Verify it fails**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/model/ -run TestEnvelopeV2 -v`
Expected: FAIL (`FromID`, `ToID` undefined on Envelope).

- [ ] **Step 3: Implement**

Modify `internal/model/envelope.go` to add two fields (omitempty so existing JSON fixtures keep parsing):
```go
type Envelope struct {
	ConversationID  string   `json:"conversation_id"`
	TurnIndex       int      `json:"turn_index"`
	MaxTurns        int      `json:"max_turns"`
	From            PeerSlot `json:"from"`               // legacy slot; still populated on write
	FromID          PeerID   `json:"from_id,omitempty"`  // v2 peer id
	ToID            PeerID   `json:"to_id,omitempty"`    // v2 peer id (target of this envelope)
	Body            string   `json:"body"`
	MarkerToken     string   `json:"marker_token"`
	IncludeBriefing bool     `json:"include_briefing"`
	Briefing        string   `json:"briefing,omitempty"`
}
```

- [ ] **Step 4: Verify test passes**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/model/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis && git add internal/model/envelope.go internal/model/envelope_test.go && git commit -m "feat(model): add PeerID fields to Envelope"
```

### Task 2.2: Add `PeerParticipant`, `Conversation.Peers`, `Conversation.Seeds`, `OpenerID`

- [ ] **Step 1: Write the failing test**

Append to `internal/model/model_test.go`:
```go
func TestConversationV2Participants(t *testing.T) {
	conv := Conversation{
		ID:            "c1",
		SchemaVersion: 2,
		Peers: []PeerParticipant{
			{ID: "alice", Spec: PeerSpec{URI: "provider:claude-code"}},
			{ID: "bob",   Spec: PeerSpec{URI: "provider:codex"}},
			{ID: "carol", Spec: PeerSpec{URI: "provider:claude-code"}},
		},
		Seeds: map[PeerID]string{
			"alice": "you are the optimist",
			"bob":   "you are the pessimist",
			"carol": "you are the moderator",
		},
		OpenerID: "alice",
		MaxTurns: 12,
	}
	data, err := json.Marshal(conv)
	require.NoError(t, err)

	var decoded Conversation
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, 2, decoded.SchemaVersion)
	require.Len(t, decoded.Peers, 3)
	require.Equal(t, PeerID("alice"), decoded.Peers[0].ID)
	require.Equal(t, "you are the moderator", decoded.Seeds[PeerID("carol")])
	require.Equal(t, PeerID("alice"), decoded.OpenerID)
}
```

- [ ] **Step 2: Verify it fails**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/model/ -run TestConversationV2Participants -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Add to `internal/model/model.go`:
```go
// PeerParticipant is the v2 representation of a conversation participant.
type PeerParticipant struct {
	ID   PeerID   `json:"id"`
	Spec PeerSpec `json:"spec"`
}
```

Extend `Conversation`:
```go
type Conversation struct {
	ID             string             `json:"conversation_id"`
	SchemaVersion  int                `json:"schema_version,omitempty"` // 2 for v2; 0/absent for v1
	CreatedAt      time.Time          `json:"created_at"`
	// ... existing fields ...

	// v2 fields
	Peers    []PeerParticipant `json:"peers,omitempty"`
	Seeds    map[PeerID]string `json:"seeds,omitempty"`
	OpenerID PeerID            `json:"opener_id,omitempty"`

	// v1 back-compat mirrors (still populated on legacy write path)
	PeerA  PeerSpec `json:"peer_a"`
	PeerB  PeerSpec `json:"peer_b"`
	SeedA  string   `json:"seed_a"`
	SeedB  string   `json:"seed_b"`
	Opener PeerSlot `json:"opener"`
	// ... rest unchanged ...
}
```

- [ ] **Step 4: Verify test passes**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/model/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis && git add internal/model/model.go internal/model/model_test.go && git commit -m "feat(model): add v2 Conversation participants, seeds, opener id"
```

### Task 2.3: Extend `ConversationTurn` with v2 fields

- [ ] **Step 1: Write the failing test**

Append to `internal/model/model_test.go`:
```go
func TestConversationTurnV2Fields(t *testing.T) {
	parsed := PeerID("bob")
	turn := ConversationTurn{
		ConversationID: "c1",
		Index:          4,
		From:           PeerSlotA,
		FromID:         PeerID("alice"),
		ToID:           PeerID("bob"),
		Reason:         TurnReasonAddressed,
		NextIDParsed:   &parsed,
		FallbackUsed:   false,
	}
	data, err := json.Marshal(turn)
	require.NoError(t, err)
	var decoded ConversationTurn
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, TurnReasonAddressed, decoded.Reason)
	require.NotNil(t, decoded.NextIDParsed)
	require.Equal(t, PeerID("bob"), *decoded.NextIDParsed)
}
```

- [ ] **Step 2: Verify it fails**

Run: `go test ./internal/model/ -run TestConversationTurnV2 -v` → FAIL.

- [ ] **Step 3: Implement**

Add to `internal/model/model.go`:
```go
// TurnReason describes why the turn went to its recipient.
type TurnReason string

const (
	TurnReasonOpener              TurnReason = "opener"
	TurnReasonAddressed           TurnReason = "addressed"
	TurnReasonFallbackRoundRobin  TurnReason = "fallback_roundrobin"
	TurnReasonEnd                 TurnReason = "end"
)
```

Extend `ConversationTurn`:
```go
type ConversationTurn struct {
	// ... existing fields ...
	FromID       PeerID     `json:"from_id,omitempty"`
	ToID         PeerID     `json:"to_id,omitempty"`
	Reason       TurnReason `json:"reason,omitempty"`
	NextIDParsed *PeerID    `json:"next_id_parsed,omitempty"`
	FallbackUsed bool       `json:"fallback_used,omitempty"`
}
```

- [ ] **Step 4: Verify test passes**

Run: `go test ./internal/model/ -v` → PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis && git add internal/model/model.go internal/model/model_test.go && git commit -m "feat(model): add v2 turn reason and peer id fields to ConversationTurn"
```

### Task 2.4: Phase 2 gate

- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `go vet ./...`
All must pass. Existing 2-peer tests still green because legacy fields are untouched and new fields are `omitempty`.

---

## Phase 3 — TurnPolicy package (addressed + round-robin fallback)

**Files:**
- Create: `internal/orchestrator/policy/policy.go`
- Create: `internal/orchestrator/policy/addressed.go`
- Create: `internal/orchestrator/policy/addressed_test.go`
- Create: `internal/orchestrator/policy/roundrobin.go`

### Task 3.1: Define `TurnPolicy` interface

- [ ] **Step 1: Create `internal/orchestrator/policy/policy.go`**

```go
// Package policy implements turn-ordering policies for multi-peer conversations.
package policy

import "github.com/kamilandrzejrybacki-inc/vitis/internal/model"

// Decision is the output of TurnPolicy.Next for a single turn.
type Decision struct {
	Next         model.PeerID  // the peer that speaks next
	Parsed       *model.PeerID // what was parsed from the trailer, if anything
	FallbackUsed bool          // true if we fell back to round-robin
}

// TurnPolicy selects the next speaker given the current peer and their reply.
type TurnPolicy interface {
	// Next returns the id of the next peer to speak. The peers slice is
	// the declared peer order (used for round-robin fallback). The current
	// peer must be present in peers.
	Next(current model.PeerID, reply string, peers []model.PeerID) Decision
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/orchestrator/policy/policy.go && git commit -m "feat(policy): add TurnPolicy interface"
```

### Task 3.2: Addressed policy + trailer parser (TDD)

- [ ] **Step 1: Write the failing tests**

`internal/orchestrator/policy/addressed_test.go`:
```go
package policy

import (
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/require"
)

func TestParseNextTrailer(t *testing.T) {
	cases := []struct {
		name    string
		reply   string
		want    *string
	}{
		{"clean", "some text\n<<NEXT: bob>>", strPtr("bob")},
		{"trailing whitespace", "text\n<<NEXT: bob>>   \n", strPtr("bob")},
		{"no trailer", "just a reply", nil},
		{"end wins over next", "text\n<<NEXT: bob>>\n<<END>>", nil},
		{"end only", "text\n<<END>>", nil},
		{"trailer inside code fence ignored", "```\n<<NEXT: bob>>\n```\nlast line", nil},
		{"uppercase id rejected", "text\n<<NEXT: Bob>>", nil},
		{"digit leading id rejected", "text\n<<NEXT: 1bob>>", nil},
		{"id with hyphen", "text\n<<NEXT: peer-1>>", strPtr("peer-1")},
		{"extra spaces inside trailer", "text\n<<NEXT:   bob   >>", strPtr("bob")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseNextTrailer(c.reply)
			if c.want == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, *c.want, string(*got))
		})
	}
}

func strPtr(s string) *string { return &s }

func TestAddressedPolicyHappyPath(t *testing.T) {
	p := NewAddressedPolicy()
	peers := []model.PeerID{"alice", "bob", "carol"}
	d := p.Next("alice", "hi bob\n<<NEXT: bob>>", peers)
	require.Equal(t, model.PeerID("bob"), d.Next)
	require.False(t, d.FallbackUsed)
	require.NotNil(t, d.Parsed)
	require.Equal(t, model.PeerID("bob"), *d.Parsed)
}

func TestAddressedPolicyFallbackMissing(t *testing.T) {
	p := NewAddressedPolicy()
	peers := []model.PeerID{"alice", "bob", "carol"}
	d := p.Next("alice", "just a reply", peers)
	require.Equal(t, model.PeerID("bob"), d.Next)
	require.True(t, d.FallbackUsed)
	require.Nil(t, d.Parsed)
}

func TestAddressedPolicyFallbackUnknown(t *testing.T) {
	p := NewAddressedPolicy()
	peers := []model.PeerID{"alice", "bob", "carol"}
	d := p.Next("alice", "text\n<<NEXT: ghost>>", peers)
	require.Equal(t, model.PeerID("bob"), d.Next)
	require.True(t, d.FallbackUsed)
	require.NotNil(t, d.Parsed)
	require.Equal(t, model.PeerID("ghost"), *d.Parsed)
}

func TestAddressedPolicyFallbackSelf(t *testing.T) {
	p := NewAddressedPolicy()
	peers := []model.PeerID{"alice", "bob", "carol"}
	d := p.Next("alice", "text\n<<NEXT: alice>>", peers)
	require.Equal(t, model.PeerID("bob"), d.Next)
	require.True(t, d.FallbackUsed)
}

func TestAddressedPolicyRoundRobinWraps(t *testing.T) {
	p := NewAddressedPolicy()
	peers := []model.PeerID{"alice", "bob", "carol"}
	d := p.Next("carol", "text", peers)
	require.Equal(t, model.PeerID("alice"), d.Next)
	require.True(t, d.FallbackUsed)
}
```

- [ ] **Step 2: Verify it fails**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/orchestrator/policy/... -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

`internal/orchestrator/policy/roundrobin.go`:
```go
package policy

import "github.com/kamilandrzejrybacki-inc/vitis/internal/model"

// roundRobinAfter returns the peer that follows current in peers, wrapping
// around. If current is not in peers, returns peers[0]. Panics on empty peers.
func roundRobinAfter(current model.PeerID, peers []model.PeerID) model.PeerID {
	if len(peers) == 0 {
		panic("roundRobinAfter: empty peer list")
	}
	for i, p := range peers {
		if p == current {
			return peers[(i+1)%len(peers)]
		}
	}
	return peers[0]
}
```

`internal/orchestrator/policy/addressed.go`:
```go
package policy

import (
	"regexp"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// AddressedPolicy selects the next speaker from a <<NEXT: peer-id>> trailer,
// falling back to round-robin in the declared peer order if the trailer is
// missing, unparseable, or addresses an unknown or self id.
type AddressedPolicy struct{}

func NewAddressedPolicy() *AddressedPolicy { return &AddressedPolicy{} }

var (
	nextTrailerRegex = regexp.MustCompile(`^<<NEXT:\s*([a-z][a-z0-9_-]{0,31})\s*>>$`)
	endTrailerRegex  = regexp.MustCompile(`^<<END>>$`)
)

// parseNextTrailer returns the id captured from a <<NEXT: id>> trailer on
// the last non-empty line of reply. <<END>> on the last non-empty line
// overrides and returns nil.
func parseNextTrailer(reply string) *model.PeerID {
	// Find the last non-empty line.
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
```

- [ ] **Step 4: Verify tests pass**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/orchestrator/policy/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis && git add internal/orchestrator/policy/ && git commit -m "feat(policy): add AddressedPolicy with <<NEXT>> trailer parser and round-robin fallback"
```

### Task 3.3: Phase 3 gate

- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `go vet ./...`

---

## Phase 4 — Orchestrator turn loop uses TurnPolicy

**Files:**
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `internal/orchestrator/orchestrator_test.go`
- Modify: `internal/orchestrator/completion_loop.go`
- Modify: `internal/conversation/envelope.go` (add peer-id-aware builder)

### Task 4.1: Add `peerIDsFromConversation` helper

- [ ] **Step 1: Failing test**

Add to `internal/orchestrator/orchestrator_test.go`:
```go
func TestPeerIDsFromConversationV2(t *testing.T) {
	conv := model.Conversation{
		SchemaVersion: 2,
		Peers: []model.PeerParticipant{
			{ID: "alice"}, {ID: "bob"}, {ID: "carol"},
		},
	}
	require.Equal(t, []model.PeerID{"alice", "bob", "carol"}, peerIDsFromConversation(conv))
}

func TestPeerIDsFromConversationV1Legacy(t *testing.T) {
	conv := model.Conversation{
		PeerA: model.PeerSpec{URI: "provider:claude-code"},
		PeerB: model.PeerSpec{URI: "provider:codex"},
	}
	require.Equal(t, []model.PeerID{"a", "b"}, peerIDsFromConversation(conv))
}
```

- [ ] **Step 2: Verify fail**: `go test ./internal/orchestrator/ -run PeerIDsFromConversation -v`

- [ ] **Step 3: Implement** (append to `orchestrator.go`):
```go
// peerIDsFromConversation returns the declared peer order as PeerIDs.
// v2 conversations use Peers[]; v1 conversations synthesize ids "a","b".
func peerIDsFromConversation(conv model.Conversation) []model.PeerID {
	if len(conv.Peers) > 0 {
		ids := make([]model.PeerID, len(conv.Peers))
		for i, p := range conv.Peers {
			ids[i] = p.ID
		}
		return ids
	}
	return []model.PeerID{"a", "b"}
}
```

- [ ] **Step 4: Verify pass**

- [ ] **Step 5: Commit**: `git commit -m "feat(orchestrator): add peer id extraction helper"`

### Task 4.2: Delegate next-speaker selection to policy

> **Warning:** This is the largest edit in the plan. The orchestrator's existing `runTurnLoop` (or equivalent) currently switches between `PeerSlotA` and `PeerSlotB` after each response. Replace that switch with a `TurnPolicy.Next(...)` call, keeping the legacy `Slot` field on records populated via the alias mapping (`a`/`b` → `PeerSlotA`/`PeerSlotB`).

Read `internal/orchestrator/orchestrator.go` top-to-bottom before editing. The turn loop function and the completion-loop interaction are the only places that should change. Keep the rest of the orchestrator (timeouts, terminator dispatch, store writes) untouched except for populating the new turn record fields.

- [ ] **Step 1: Add policy dependency to orchestrator struct**
  Add field `policy policy.TurnPolicy` (defaulting to `policy.NewAddressedPolicy()` in the constructor).

- [ ] **Step 2: Replace the `nextSlot := current.Other()` line(s)** with:
```go
peers := peerIDsFromConversation(conv)
currentID := model.PeerIDFromSlot(currentSlot) // during transition
decision := o.policy.Next(currentID, response, peers)
nextID := decision.Next
nextSlot := legacySlotForID(nextID) // "a" -> PeerSlotA, "b" -> PeerSlotB, else PeerSlotA (only 2-peer legacy path uses this)
```
Add helper:
```go
func legacySlotForID(id model.PeerID) model.PeerSlot {
	if id == "b" {
		return model.PeerSlotB
	}
	return model.PeerSlotA
}
```

- [ ] **Step 3: Populate new fields on the recorded `ConversationTurn`:**
```go
turn.FromID = currentID
turn.ToID = nextID
if decision.FallbackUsed {
	turn.Reason = model.TurnReasonFallbackRoundRobin
	turn.FallbackUsed = true
} else {
	turn.Reason = model.TurnReasonAddressed
}
if decision.Parsed != nil {
	parsed := *decision.Parsed
	turn.NextIDParsed = &parsed
}
```

- [ ] **Step 4: Regression test — existing 2-peer orchestrator tests MUST still pass unchanged**

Run: `cd /home/kamil-rybacki/Code/vitis && go test ./internal/orchestrator/... -v`
Expected: all green. If any test fails because it asserts on `turn.From == PeerSlotA/B` — that still works, `From` is still populated. If any fails on `nextSlot := current.Other()` semantics, the legacy 2-peer path maps alice→bob identical to PeerSlotA→PeerSlotB.

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(orchestrator): delegate next-speaker selection to TurnPolicy"
```

### Task 4.3: Phase 4 gate

- [ ] `go build ./...` + `go test ./...` + `go vet ./...` all green.

---

## Phase 5 — Store schema v2 writer + v1 compat reader

**Files:**
- Modify: `internal/store/file/file.go` (+ its tests)
- Create: `internal/store/v1compat/reader.go` (+ tests + testdata)
- Modify: `internal/store/postgres/postgres.go` (migration + writer)

### Task 5.1: File store writes `schema_version: 2`

- [ ] **Step 1: Test** — update `internal/store/file/file_test.go` with a test that writes a v2 conversation (populated `Peers`, `Seeds`, `OpenerID`) and asserts the on-disk JSON contains `"schema_version": 2`.

- [ ] **Step 2: Fail**: `go test ./internal/store/file/ -v`

- [ ] **Step 3: Implement** — in the file-store writer, if `conv.SchemaVersion == 0` and `len(conv.Peers) > 0`, set `conv.SchemaVersion = 2` before marshal. For legacy v1 writes (no Peers), set `SchemaVersion = 0` so the JSON omits the field (preserving byte-level fixture parity with existing tests).

- [ ] **Step 4: Pass**

- [ ] **Step 5: Commit**: `git commit -am "feat(store/file): write schema_version 2 for v2 conversations"`

### Task 5.2: v1 compat reader

- [ ] **Step 1: Create `internal/store/v1compat/testdata/v1_conversation.json`** — a frozen copy of a current 2-peer conversation JSON (grab from any existing test fixture or synthesize one matching the current on-disk shape: `peer_a`, `peer_b`, `seed_a`, `seed_b`, `opener`, no `schema_version` field, `turns` array with `from: "a"|"b"`).

- [ ] **Step 2: Test** — `internal/store/v1compat/reader_test.go`:
```go
package v1compat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/require"
)

func TestReadV1ConversationUpgradesToV2Shape(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "v1_conversation.json"))
	require.NoError(t, err)

	conv, turns, err := ReadConversation(data)
	require.NoError(t, err)
	require.Equal(t, 2, conv.SchemaVersion)
	require.Len(t, conv.Peers, 2)
	require.Equal(t, model.PeerID("a"), conv.Peers[0].ID)
	require.Equal(t, model.PeerID("b"), conv.Peers[1].ID)
	require.Equal(t, "a", string(conv.OpenerID))
	require.NotEmpty(t, turns)
	require.Equal(t, model.TurnReasonOpener, turns[0].Reason)
}
```

- [ ] **Step 3: Implement** — `internal/store/v1compat/reader.go`:
```go
// Package v1compat reads legacy v1 vitis conversation files and upgrades
// them in-memory to the v2 model shape. It never writes; attempts to do
// so should route through the v2 writer on a fresh session.
package v1compat

import (
	"encoding/json"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// ReadConversation parses v1 JSON bytes (as written by the pre-N-peer file
// store) and returns a v2-shaped Conversation plus the decoded turns.
// It is a no-op for v2 input — callers should branch on schema_version.
func ReadConversation(data []byte) (model.Conversation, []model.ConversationTurn, error) {
	var wrapper struct {
		model.Conversation
		Turns []model.ConversationTurn `json:"turns,omitempty"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return model.Conversation{}, nil, err
	}
	conv := wrapper.Conversation
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

	turns := wrapper.Turns
	for i := range turns {
		t := &turns[i]
		t.FromID = model.PeerID(t.From)
		if i == 0 {
			t.Reason = model.TurnReasonOpener
		} else {
			// Legacy alternation: from alternates a↔b.
			t.Reason = model.TurnReasonFallbackRoundRobin // parser had no signal; this is faithful
			t.FallbackUsed = false                         // legacy turns weren't fallbacks
			// Override: mark as addressed because the v1 alternation was the design
			t.Reason = model.TurnReasonAddressed
		}
		// Set ToID to the other peer in the 2-peer legacy model.
		if t.FromID == "a" {
			t.ToID = "b"
		} else {
			t.ToID = "a"
		}
	}
	return conv, turns, nil
}
```

- [ ] **Step 4: Pass tests** then **commit**: `git commit -m "feat(store/v1compat): add read-only v1→v2 upgrader"`

### Task 5.3: File reader dispatches v1 to compat

- [ ] In `internal/store/file/file.go`, after unmarshaling the top-level JSON, if `schema_version` is absent or `< 2`, delegate to `v1compat.ReadConversation` for the final model.
- [ ] Add a test that feeds the v1 fixture through the file store's public Read path and asserts v2 fields are populated.
- [ ] Commit.

### Task 5.4: Postgres schema migration

- [ ] Add a new migration (or extend the existing init schema) that adds columns `from_id TEXT`, `to_id TEXT`, `reason TEXT`, `next_id_parsed TEXT`, `fallback_used BOOLEAN DEFAULT FALSE`, `schema_version INTEGER DEFAULT 1` to the turns and conversations tables as appropriate.
- [ ] Writer populates the new columns; reader returns them if present.
- [ ] Tests against a test container (reuse existing postgres test infra).
- [ ] Commit.

### Task 5.5: Phase 5 gate

- [ ] `go build ./...` + `go test ./...` + `go vet ./...`.

---

## Phase 6 — CLI surface + back-compat alias layer

**Files:**
- Modify: `internal/cli/converse.go`
- Modify: `internal/cli/converse_test.go`

### Task 6.1: Parse repeatable `--peer` and `--seed` flags

- [ ] Add a `--peer` string slice flag: `cmd.Flags().StringArray("peer", nil, "repeatable: id=<id>,provider=<provider>[,key=val,...]")`.
- [ ] Add a `--seed` string slice flag: `cmd.Flags().StringArray("seed", nil, "repeatable: id=<id>,content=\"...\"; broadcast form: content=\"...\"")`.
- [ ] Add a `--opener` string flag (generalized from `a|b`).
- [ ] Keep `--peer-a`, `--peer-b`, `--peer-a-opt`, `--peer-b-opt`, `--seed-a`, `--seed-b` as-is.

Write a key=value parser that supports `content="..."` with embedded commas (double-quoted, `\"` escape). Unit tests cover: simple `id=a,provider=x`, quoted content with commas, escaped quotes, mixed order, unknown keys → error.

Unit test first, then implement, then commit per task-style above.

### Task 6.2: Alias translation layer

- [ ] Before building `model.Conversation`, if `--peer-a` is set, synthesize a `--peer id=a,provider=<parsed>` entry. Same for `--peer-b → id=b`. Reject if both `--peer-a` and `--peer id=a,...` were given (ambiguous).
- [ ] Translate `--seed-a` → `--seed id=a,content=...`, same for `b`.
- [ ] Translate bare `--seed content=...` (broadcast): fan out to every declared peer id.
- [ ] Translate `--opener a|b` → `--opener a|b` (still valid ids).

Unit tests on the translation function:
- All 2-peer invocations from existing test suite produce a valid v2 `model.Conversation` with `Peers = [{a,...},{b,...}]`, `Seeds = {a:..., b:...}`, `OpenerID = "a"`.
- 3-peer invocation produces a 3-element Peers slice.
- Mixing `--peer-a` with `--peer id=a,...` → error.
- Broadcast `--seed content=...` + any `--seed id=...` → error.
- Duplicate peer ids → error.
- N > 16 → error.
- N < 2 → error.
- Unknown `--opener <id>` → error.
- Missing seed for declared peer → error.

### Task 6.3: Validation consolidated

- [ ] Single `validateConvConfig(cfg)` function that runs all checks and returns a joined error (use `errors.Join`).
- [ ] Test each error path independently.

### Task 6.4: Phase 6 gate

- [ ] `go build ./...` + `go test ./...` + `go vet ./...`
- [ ] **Back-compat acceptance gate:** ALL existing tests in `internal/cli/converse_test.go`, `converse_e2e_test.go`, `extra_test.go` pass without modification. If any test file needed editing to keep passing, the alias layer is wrong — revert the test and fix the layer.

---

## Phase 7 — Mock agent extensions + N-peer integration tests + manual scripts

**Files:**
- Modify: `mockagent/main.go`
- Create: `tests/manual/13_converse_three_peer_addressed.sh`
- Create: `tests/manual/14_converse_three_peer_fallback.sh`
- Create: `tests/manual/15_converse_backcompat_aliases.sh`
- Modify: `internal/orchestrator/orchestrator_test.go` (add N-peer integration cases)

### Task 7.1: Mock agent trailer directives

- [ ] Add flags to mockagent: `--next-trailer <id>` (append `<<NEXT: id>>` to every reply), `--no-trailer` (default — no trailer), `--bad-trailer <raw>` (append `<<NEXT: raw>>` verbatim).
- [ ] Unit test the flag parsing.
- [ ] Commit.

### Task 7.2: 3-peer addressed integration test

- [ ] Add an integration test under `internal/orchestrator/` that spawns 3 mock agents with scripted `--next-trailer` values producing sequence alice→bob→carol→alice, runs a short conversation, asserts turn order and `reason=addressed` on every non-opener turn.
- [ ] Commit.

### Task 7.3: 3-peer fallback integration test

- [ ] Same setup, mocks emit `--no-trailer`; assert round-robin order and `fallback_used=true` on every turn.
- [ ] Commit.

### Task 7.4: 3-peer unknown-addressee integration test

- [ ] Mocks emit `--bad-trailer ghost`; assert `NextIDParsed == "ghost"`, `fallback_used=true`, conversation continues to terminator.
- [ ] Commit.

### Task 7.5: Manual scripts

- [ ] Model `13_`, `14_`, `15_` on existing `tests/manual/05_converse_mock_sentinel.sh`: set up fixture providers, run `vitis converse` with the new flags, assert JSONL outputs contain expected `from_id`/`to_id`/`reason`.
- [ ] Wire them into `tests/manual/run_all.sh`.
- [ ] Commit.

### Task 7.6: Phase 7 gate

- [ ] Full `go test ./...` + `go vet ./...`.
- [ ] `tests/manual/run_all.sh` (or at minimum scripts 01–15).

---

## Phase 8 — Docs + ADR

**Files:**
- Create: `docs/superpowers/adrs/2026-04-08-n-peer-addressed-routing.md`
- Modify: `README.md`
- Modify: `docs/` (site content: A2A conversation guide, flag reference, migration note)

### Task 8.1: ADR

- [ ] Write a single ADR that captures: decision (N-peer with addressed routing + RR fallback), context (2-peer limitation, future UI dependency), alternatives considered (round-robin only, explicit schedule, moderator-driven), consequences (schema v2 break, trailer parser surface, back-compat alias debt). Link to the spec and plan.
- [ ] Commit.

### Task 8.2: README

- [ ] Add an "N-peer conversations" section showing the 3-peer example from the spec, the trailer protocol, and a note that `--peer-a/b` still work.
- [ ] Commit.

### Task 8.3: Site docs

- [ ] Update the A2A conversation guide with the new flag reference and trailer protocol.
- [ ] Add a short "Migrating from 2-peer" note explaining that v1 sessions are readable via `vitis peek` and that new conversations are written as v2.
- [ ] Commit.

### Task 8.4: Final regression gate

- [ ] `go build ./...` + `go test ./...` + `go vet ./...` + `tests/manual/run_all.sh`
- [ ] All green.

---

## Self-review checklist (plan author)

- [x] Every spec section has at least one task.
- [x] No "TBD", "later", or "handle appropriately" in any task.
- [x] PeerID, TurnReason, Decision, TurnPolicy — all defined before use.
- [x] The orchestrator edit (Task 4.2) is flagged as the highest-risk change and explicitly asks the subagent to read the file first.
- [x] Back-compat acceptance gates (phases 4 and 6) forbid modifying existing tests.
- [x] Phase 5 v1 compat reader has a concrete fixture path and a concrete test.
- [x] CLI validation rules are enumerated, not hand-waved.

---

## Execution note

This plan is scoped to the `vitis` repo only. The `vitis-ui` web UI is a **separate spec**, not part of this plan. The `vitis serve` subcommand is out of scope here — it will be specified in the UI design document and depends on N-peer being merged first.
