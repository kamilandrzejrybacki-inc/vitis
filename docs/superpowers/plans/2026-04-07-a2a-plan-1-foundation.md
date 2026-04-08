# A2A Plan 1 — Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the pure-logic foundation for vitis's A2A conversation feature: data model, event bus (in-process backend), envelope/marker/briefing builders, sentinel terminator, file-store conversation persistence, and a Broker state machine that runs end-to-end against scripted mock peer transports — all with zero PTY changes and no CLI yet.

**Architecture:** New top-level packages (`internal/model` additions, `internal/bus`, `internal/bus/inproc`, `internal/conversation`, `internal/peer`, `internal/terminator`) communicate exclusively through narrow interfaces. The Broker subscribes to a `Bus` to receive turns and control messages, dispatches envelopes through `PeerTransport.Deliver`, runs `Terminator` as a bus subscriber, and persists via the existing `Store` interface (extended additively). The single-shot `vitis run` path is unchanged.

**Tech Stack:** Go 1.22+, standard library only. Tests use `go test` with table-driven patterns and `-race`. No new third-party dependencies in this plan.

**Scope:** Foundation only. Plan 2 adds PersistentPseudoTerminalProcess + adapter `TurnBoundaryDetector` + provider transport + `vitis converse` CLI. Plans 3–5 add judge, Postgres, NATS, stdio.

**Existing-codebase notes the engineer needs:**

- Module path is `github.com/kamilandrzejrybacki-inc/vitis`. Always import via this path.
- Existing `internal/model` types live in `result.go`, `session.go`, `turn.go`, `status.go`, `errors.go`, `events.go`. New conversation types go in a new file `conversation.go` in the same package.
- Existing `internal/store/store.go` defines the `Store` interface; the file backend lives at `internal/store/file/file_store.go` and uses sync.Mutex + atomic JSON file writes + JSONL append. Follow the same patterns.
- Tests use `testify` (`github.com/stretchr/testify/require` and `assert`) — already in `go.sum`. Verify by grepping existing tests.
- Run a single test with `go test -race -run TestName ./path/to/pkg/...`. Run the whole suite with `go test -race ./...`.
- Commit messages use Conventional Commits (`feat:`, `test:`, `refactor:`, `chore:`). Project rule: do NOT add Co-Authored-By trailers (the global `~/.claude/settings.json` strips them).
- After every code change, run `gofmt -w <file>` and `go vet ./...` before committing.
- The branch is `feat/fill-plan-gaps`. All commits land here. Do not push during plan execution; the orchestrator pushes once at the very end.

---

## File Structure (Plan 1)

| File | Status | Responsibility |
|---|---|---|
| `internal/model/conversation.go` | Create | Conversation, ConversationTurn, ConversationStatus, PeerSlot, PeerSpec, TerminatorSpec, ConversationPatch, Verdict, Envelope value types |
| `internal/model/conversation_test.go` | Create | Round-trip JSON for Conversation and ConversationTurn; status enum coverage |
| `internal/bus/bus.go` | Create | `Bus` interface, `BusMessage`, `ControlMsg`, topic helper functions |
| `internal/bus/inproc/inproc.go` | Create | In-process channel-based Bus implementation |
| `internal/bus/inproc/inproc_test.go` | Create | Pub/sub fan-out, multiple subscribers, full-channel drop, close cleanup |
| `internal/conversation/marker.go` | Create | Random per-turn marker token generator + parser helpers |
| `internal/conversation/marker_test.go` | Create | Uniqueness, format, parse round-trip |
| `internal/conversation/briefing.go` | Create | Briefing template renderer (sentinel and judge variants) |
| `internal/conversation/envelope.go` | Create | Envelope builder for turn 1 and turn N |
| `internal/conversation/envelope_test.go` | Create | Turn-1 includes briefing, turn-N omits, marker is embedded |
| `internal/conversation/result.go` | Create | `FinalResult` and helpers |
| `internal/peer/transport.go` | Create | `PeerTransport` interface + `Envelope` re-exported (so peer transports never import `conversation`) |
| `internal/peer/mock/mock.go` | Create | Scripted mock peer transport for tests (lives in test-only package) |
| `internal/terminator/terminator.go` | Create | `Terminator` interface |
| `internal/terminator/sentinel.go` | Create | Sentinel terminator implementation |
| `internal/terminator/sentinel_test.go` | Create | Sentinel detected → verdict published; sentinel stripped from response |
| `internal/conversation/broker.go` | Create | Broker state machine |
| `internal/conversation/broker_test.go` | Create | Alternation, max-turns cap, sentinel termination, peer-crash control message, context cancellation |
| `internal/store/store.go` | Modify | Add `CreateConversation`, `UpdateConversation`, `AppendConversationTurn`, `PeekConversationTurns` to `Store` interface |
| `internal/store/file/file_store.go` | Modify | Implement the new methods using the same atomic-write / JSONL-append patterns; add `conversationsDir()` |
| `internal/store/file/file_store_conversation_test.go` | Create | Round-trip persistence; PeekConversationTurns ordering; failure-policy verification |

**Total: 13 source files (11 new + 2 modified) + 7 test files.**

---

## Task 0 — Branch hygiene and pre-flight

**Files:** none (working directory state)

- [ ] **Step 1: Confirm branch and clean state**

```bash
git rev-parse --abbrev-ref HEAD
git status --short
```

Expected: branch `feat/fill-plan-gaps`. Status may show pending modifications from prior work (this is fine; do NOT stash or revert them).

- [ ] **Step 2: Verify build is currently green**

```bash
go build ./...
go test -race -count=1 ./internal/model/... ./internal/store/...
```

Expected: build succeeds. Existing model and store tests pass. If they don't, STOP and report — Plan 1 cannot proceed against a broken baseline.

- [ ] **Step 3: Create the plan-execution working note**

Append a single line to `docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md` at the very bottom (after the final task) marking "execution started <timestamp>". This is so the orchestrator can verify the plan was actually opened.

```bash
echo "" >> docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md
echo "<!-- execution-started: $(date -Is) -->" >> docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md
```

No commit yet — this marker rides along with Task 1's commit.

---

## Task 1 — Conversation data model

**Files:**
- Create: `internal/model/conversation.go`
- Test: `internal/model/conversation_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/model/conversation_test.go`:

```go
package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConversationStatusValues(t *testing.T) {
	values := []ConversationStatus{
		ConvRunning,
		ConvCompletedSentinel,
		ConvCompletedJudge,
		ConvMaxTurnsHit,
		ConvPeerCrashed,
		ConvPeerBlocked,
		ConvTimeout,
		ConvInterrupted,
		ConvError,
	}
	for _, v := range values {
		require.NotEmpty(t, string(v))
	}
}

func TestPeerSlotOther(t *testing.T) {
	require.Equal(t, PeerSlotB, PeerSlotA.Other())
	require.Equal(t, PeerSlotA, PeerSlotB.Other())
}

func TestConversationJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 7, 18, 0, 0, 0, time.UTC)
	end := now.Add(5 * time.Minute)
	conv := Conversation{
		ID:             "conv-test-1",
		CreatedAt:      now,
		EndedAt:        &end,
		Status:         ConvCompletedSentinel,
		MaxTurns:       50,
		PerTurnTimeout: 300 * time.Second,
		OverallTimeout: 3600 * time.Second,
		Terminator: TerminatorSpec{
			Kind:     "sentinel",
			Sentinel: "<<END>>",
		},
		PeerA:         PeerSpec{URI: "provider:claude-code", Options: map[string]string{"model": "claude-sonnet-4-6"}},
		PeerB:         PeerSpec{URI: "provider:codex", Options: map[string]string{"model": "gpt-5"}},
		SeedA:         "Discuss X",
		SeedB:         "Discuss X",
		Opener:        PeerSlotA,
		TurnsConsumed: 7,
	}
	data, err := json.Marshal(conv)
	require.NoError(t, err)

	var decoded Conversation
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, conv.ID, decoded.ID)
	require.Equal(t, conv.Status, decoded.Status)
	require.Equal(t, conv.PeerA.URI, decoded.PeerA.URI)
	require.Equal(t, conv.PeerA.Options["model"], decoded.PeerA.Options["model"])
	require.Equal(t, conv.Opener, decoded.Opener)
	require.Equal(t, conv.TurnsConsumed, decoded.TurnsConsumed)
	require.WithinDuration(t, *conv.EndedAt, *decoded.EndedAt, time.Second)
}

func TestConversationTurnJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 7, 18, 0, 0, 0, time.UTC)
	turn := ConversationTurn{
		ConversationID:       "conv-test-1",
		Index:                3,
		From:                 PeerSlotA,
		Envelope:             "[conversation: ...] hello",
		Response:             "hi back",
		MarkerToken:          "TURN_END_a7f3c1",
		StartedAt:            now,
		EndedAt:              now.Add(2 * time.Second),
		CompletionConfidence: 0.99,
		ParserConfidence:     0.97,
		Warnings:             []string{"marker_missing"},
	}
	data, err := json.Marshal(turn)
	require.NoError(t, err)
	var decoded ConversationTurn
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, turn, decoded)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -race -run "TestConversation|TestPeerSlot" ./internal/model/...
```

Expected: build fails with `undefined: ConversationStatus` (or similar). This proves the test is wired before the implementation exists.

- [ ] **Step 3: Write the model file**

Create `internal/model/conversation.go`:

```go
package model

import "time"

// ConversationStatus represents the lifecycle state of an A2A conversation.
type ConversationStatus string

const (
	ConvRunning           ConversationStatus = "running"
	ConvCompletedSentinel ConversationStatus = "completed_sentinel"
	ConvCompletedJudge    ConversationStatus = "completed_judge"
	ConvMaxTurnsHit       ConversationStatus = "max_turns_hit"
	ConvPeerCrashed       ConversationStatus = "peer_crashed"
	ConvPeerBlocked       ConversationStatus = "peer_blocked"
	ConvTimeout           ConversationStatus = "timeout"
	ConvInterrupted       ConversationStatus = "interrupted"
	ConvError             ConversationStatus = "error"
)

// PeerSlot identifies one of the two peers in a conversation.
type PeerSlot string

const (
	PeerSlotA PeerSlot = "a"
	PeerSlotB PeerSlot = "b"
)

// Other returns the opposite slot.
func (s PeerSlot) Other() PeerSlot {
	if s == PeerSlotA {
		return PeerSlotB
	}
	return PeerSlotA
}

// PeerSpec describes a peer at the URI level. The URI scheme determines which
// PeerTransport implementation handles it.
type PeerSpec struct {
	URI     string            `json:"uri"`
	Options map[string]string `json:"options,omitempty"`
}

// TerminatorSpec configures how a conversation decides when to end.
type TerminatorSpec struct {
	Kind     string `json:"kind"`               // "sentinel" | "judge"
	Sentinel string `json:"sentinel,omitempty"` // sentinel mode token, default "<<END>>"
	JudgeURI string `json:"judge_uri,omitempty"` // judge mode URI: bus://<topic> or provider:<id>
}

// Conversation is the top-level entity for an A2A run.
type Conversation struct {
	ID             string             `json:"conversation_id"`
	CreatedAt      time.Time          `json:"created_at"`
	EndedAt        *time.Time         `json:"ended_at,omitempty"`
	Status         ConversationStatus `json:"status"`
	MaxTurns       int                `json:"max_turns"`
	PerTurnTimeout time.Duration      `json:"per_turn_timeout"`
	OverallTimeout time.Duration      `json:"overall_timeout"`
	Terminator     TerminatorSpec     `json:"terminator"`
	PeerA          PeerSpec           `json:"peer_a"`
	PeerB          PeerSpec           `json:"peer_b"`
	SeedA          string             `json:"seed_a"`
	SeedB          string             `json:"seed_b"`
	Opener         PeerSlot           `json:"opener"`
	TurnsConsumed  int                `json:"turns_consumed"`
}

// ConversationPatch is the partial update set for an existing conversation.
type ConversationPatch struct {
	Status        *ConversationStatus `json:"status,omitempty"`
	EndedAt       *time.Time          `json:"ended_at,omitempty"`
	TurnsConsumed *int                `json:"turns_consumed,omitempty"`
}

// ConversationTurn is one exchange in the conversation log.
type ConversationTurn struct {
	ConversationID       string    `json:"conversation_id"`
	Index                int       `json:"index"`
	From                 PeerSlot  `json:"from"`
	Envelope             string    `json:"envelope"`
	Response             string    `json:"response"`
	MarkerToken          string    `json:"marker_token"`
	StartedAt            time.Time `json:"started_at"`
	EndedAt              time.Time `json:"ended_at"`
	CompletionConfidence float64   `json:"completion_confidence"`
	ParserConfidence     float64   `json:"parser_confidence"`
	Warnings             []string  `json:"warnings,omitempty"`
}

// Verdict is published by terminators to end a conversation.
type Verdict struct {
	ConversationID string             `json:"conversation_id"`
	Decision       string             `json:"decision"` // "continue" | "terminate"
	Reason         string             `json:"reason"`
	Status         ConversationStatus `json:"status"`
}

// Envelope is the structured input handed to a peer for one turn.
// The Body field is the literal text the peer sees on stdin/PTY (or the
// 'body' field of a stdio frame). MarkerToken is the per-turn termination
// marker the peer is instructed to emit.
type Envelope struct {
	ConversationID  string   `json:"conversation_id"`
	TurnIndex       int      `json:"turn_index"`
	MaxTurns        int      `json:"max_turns"`
	From            PeerSlot `json:"from"`
	Body            string   `json:"body"`
	MarkerToken     string   `json:"marker_token"`
	IncludeBriefing bool     `json:"include_briefing"`
	Briefing        string   `json:"briefing,omitempty"`
}
```

- [ ] **Step 4: Format, vet, run test**

```bash
gofmt -w internal/model/conversation.go internal/model/conversation_test.go
go vet ./internal/model/...
go test -race -run "TestConversation|TestPeerSlot" ./internal/model/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/model/conversation.go internal/model/conversation_test.go docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md
git commit -m "feat(model): add A2A Conversation, ConversationTurn, Envelope types"
```

---

## Task 2 — Bus interface

**Files:**
- Create: `internal/bus/bus.go`

This task has no test of its own — it's pure interface definition. Tests come in Task 3 (inproc backend).

- [ ] **Step 1: Write the bus package**

Create `internal/bus/bus.go`:

```go
// Package bus defines the Bus interface that backs the A2A conversation
// runtime. The Broker, peer transports, terminators, and store all
// communicate exclusively through Bus implementations.
//
// Two backends ship with vitis: an in-process channel-based Bus
// (internal/bus/inproc, default) and a NATS-backed Bus
// (internal/bus/nats, opt-in via --bus nats://...). Bus is the only
// abstraction the broker depends on; swapping backends requires no
// broker code changes.
package bus

import (
	"context"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// MessageKind tags a BusMessage payload type.
type MessageKind string

const (
	KindEnvelope MessageKind = "envelope"
	KindTurn     MessageKind = "turn"
	KindControl  MessageKind = "control"
)

// BusMessage is the on-the-wire envelope for everything published to a topic.
// Payload is JSON-encoded; the concrete type depends on Kind:
//   - KindEnvelope -> model.Envelope
//   - KindTurn     -> model.ConversationTurn
//   - KindControl  -> ControlMsg
type BusMessage struct {
	ConversationID string      `json:"conversation_id"`
	Topic          string      `json:"topic"`
	Kind           MessageKind `json:"kind"`
	Payload        []byte      `json:"payload"`
	Timestamp      time.Time   `json:"timestamp"`
}

// ControlKind tags a ControlMsg.
type ControlKind string

const (
	ControlVerdict      ControlKind = "verdict"
	ControlPeerCrashed  ControlKind = "peer_crashed"
	ControlPeerBlocked  ControlKind = "peer_blocked"
	ControlFinalize     ControlKind = "finalize"
)

// ControlMsg is the payload for KindControl bus messages.
type ControlMsg struct {
	ConversationID string                   `json:"conversation_id"`
	Kind           ControlKind              `json:"kind"`
	Slot           model.PeerSlot           `json:"slot,omitempty"`
	Reason         string                   `json:"reason,omitempty"`
	Status         model.ConversationStatus `json:"status,omitempty"`
	Detail         string                   `json:"detail,omitempty"`
	Verdict        *model.Verdict           `json:"verdict,omitempty"`
}

// Bus is a topic-based publish/subscribe interface. Implementations MUST
// honor the topic conventions documented in the A2A design spec
// (docs/superpowers/specs/2026-04-07-vitis-a2a-conversations-design.md):
//
//   conv/<id>/peer-a/in     envelope -> peer A transport
//   conv/<id>/peer-b/in     envelope -> peer B transport
//   conv/<id>/turn          turn responses, fan-out
//   conv/<id>/control       control messages, broker-authoritative
//
// Subscribe returns a channel of incoming messages and a cancel function.
// The cancel function MUST be called to release resources when the
// subscriber is done; failing to call it leaks goroutines and channels.
type Bus interface {
	Publish(ctx context.Context, topic string, msg BusMessage) error
	Subscribe(ctx context.Context, topic string) (<-chan BusMessage, func(), error)
	Close() error
}

// TopicEnvelopeIn returns the inbox topic for the named slot.
func TopicEnvelopeIn(conversationID string, slot model.PeerSlot) string {
	return "conv/" + conversationID + "/peer-" + string(slot) + "/in"
}

// TopicTurn returns the turn fan-out topic for a conversation.
func TopicTurn(conversationID string) string {
	return "conv/" + conversationID + "/turn"
}

// TopicControl returns the control topic for a conversation.
func TopicControl(conversationID string) string {
	return "conv/" + conversationID + "/control"
}
```

- [ ] **Step 2: Format and vet**

```bash
gofmt -w internal/bus/bus.go
go vet ./internal/bus/...
go build ./internal/bus/...
```

Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/bus/bus.go
git commit -m "feat(bus): define Bus interface, BusMessage, ControlMsg, topic helpers"
```

---

## Task 3 — In-process Bus backend

**Files:**
- Create: `internal/bus/inproc/inproc.go`
- Test: `internal/bus/inproc/inproc_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/bus/inproc/inproc_test.go`:

```go
package inproc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
)

func TestPublishSubscribeFanOut(t *testing.T) {
	b := New()
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	subA, cancelA, err := b.Subscribe(ctx, "conv/x/turn")
	require.NoError(t, err)
	defer cancelA()

	subB, cancelB, err := b.Subscribe(ctx, "conv/x/turn")
	require.NoError(t, err)
	defer cancelB()

	msg := bus.BusMessage{
		ConversationID: "x",
		Topic:          "conv/x/turn",
		Kind:           bus.KindTurn,
		Payload:        []byte(`{"hello":"world"}`),
		Timestamp:      time.Unix(0, 0),
	}
	require.NoError(t, b.Publish(ctx, "conv/x/turn", msg))

	for _, sub := range []<-chan bus.BusMessage{subA, subB} {
		select {
		case got := <-sub:
			require.Equal(t, msg.ConversationID, got.ConversationID)
			require.Equal(t, msg.Kind, got.Kind)
			require.Equal(t, msg.Payload, got.Payload)
		case <-time.After(time.Second):
			t.Fatal("expected message on subscriber")
		}
	}
}

func TestSubscribeIsolatedByTopic(t *testing.T) {
	b := New()
	defer b.Close()
	ctx := context.Background()

	subTurn, cancelTurn, err := b.Subscribe(ctx, "conv/x/turn")
	require.NoError(t, err)
	defer cancelTurn()
	subCtl, cancelCtl, err := b.Subscribe(ctx, "conv/x/control")
	require.NoError(t, err)
	defer cancelCtl()

	require.NoError(t, b.Publish(ctx, "conv/x/turn", bus.BusMessage{Topic: "conv/x/turn"}))

	select {
	case <-subTurn:
	case <-time.After(time.Second):
		t.Fatal("turn subscriber should have received")
	}
	select {
	case msg := <-subCtl:
		t.Fatalf("control subscriber should not receive turn topic: got %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCancelStopsDelivery(t *testing.T) {
	b := New()
	defer b.Close()
	ctx := context.Background()

	sub, cancel, err := b.Subscribe(ctx, "topic")
	require.NoError(t, err)

	cancel()

	require.NoError(t, b.Publish(ctx, "topic", bus.BusMessage{Topic: "topic"}))
	select {
	case _, open := <-sub:
		require.False(t, open, "expected closed channel after cancel")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscribe channel should be closed after cancel")
	}
}

func TestPublishToNoSubscribersIsNoOp(t *testing.T) {
	b := New()
	defer b.Close()
	require.NoError(t, b.Publish(context.Background(), "nobody", bus.BusMessage{Topic: "nobody"}))
}

func TestCloseClosesAllSubscribers(t *testing.T) {
	b := New()
	ctx := context.Background()
	sub, _, err := b.Subscribe(ctx, "topic")
	require.NoError(t, err)
	require.NoError(t, b.Close())
	select {
	case _, open := <-sub:
		require.False(t, open)
	case <-time.After(time.Second):
		t.Fatal("close should close all subscribers")
	}
}

func TestFullSubscriberDoesNotBlockPublisher(t *testing.T) {
	b := New(WithBufferSize(1))
	defer b.Close()
	ctx := context.Background()

	_, _, err := b.Subscribe(ctx, "topic")
	require.NoError(t, err)

	// Publish 50 messages without anyone draining; the publisher must not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			_ = b.Publish(ctx, "topic", bus.BusMessage{Topic: "topic"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publisher blocked on full subscriber")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -race -run "TestPublishSubscribeFanOut|TestSubscribeIsolatedByTopic|TestCancelStopsDelivery|TestPublishToNoSubscribersIsNoOp|TestCloseClosesAllSubscribers|TestFullSubscriberDoesNotBlockPublisher" ./internal/bus/inproc/...
```

Expected: build fails with `undefined: New` / `undefined: WithBufferSize`.

- [ ] **Step 3: Write the implementation**

Create `internal/bus/inproc/inproc.go`:

```go
// Package inproc is the default in-process Bus backend. It is a
// channel-fanout broker with no external dependencies. It is the
// correct choice for single-machine, single-process vitis converse
// runs. For distributed peers, observability, or external judges,
// use internal/bus/nats instead.
package inproc

import (
	"context"
	"errors"
	"sync"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
)

// Default subscriber buffer size. Tunable per-Bus via WithBufferSize.
const defaultBufferSize = 64

// Option configures a Bus at construction time.
type Option func(*Bus)

// WithBufferSize overrides the per-subscriber channel buffer size.
func WithBufferSize(n int) Option {
	return func(b *Bus) {
		if n > 0 {
			b.bufferSize = n
		}
	}
}

// Bus is the in-process Bus implementation.
type Bus struct {
	mu         sync.RWMutex
	closed     bool
	bufferSize int
	subs       map[string][]*subscription
}

type subscription struct {
	ch     chan bus.BusMessage
	closed bool
	mu     sync.Mutex
}

// New constructs an in-process Bus.
func New(opts ...Option) *Bus {
	b := &Bus{
		bufferSize: defaultBufferSize,
		subs:       make(map[string][]*subscription),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Publish fans the message out to every current subscriber on topic.
// A subscriber whose channel is full has the message dropped silently
// (a warning is the caller's responsibility — bus implementations must
// not block on slow consumers).
func (b *Bus) Publish(_ context.Context, topic string, msg bus.BusMessage) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errors.New("inproc bus: publish on closed bus")
	}
	for _, sub := range b.subs[topic] {
		sub.deliver(msg)
	}
	return nil
}

// Subscribe registers a new subscriber on topic and returns its channel
// plus a cancel function. The cancel function unsubscribes and closes
// the channel; it is safe to call multiple times.
func (b *Bus) Subscribe(_ context.Context, topic string) (<-chan bus.BusMessage, func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, nil, errors.New("inproc bus: subscribe on closed bus")
	}
	sub := &subscription{ch: make(chan bus.BusMessage, b.bufferSize)}
	b.subs[topic] = append(b.subs[topic], sub)

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		// Remove sub from b.subs[topic]
		list := b.subs[topic]
		for i, s := range list {
			if s == sub {
				b.subs[topic] = append(list[:i], list[i+1:]...)
				break
			}
		}
		sub.close()
	}
	return sub.ch, cancel, nil
}

// Close closes every subscription and marks the bus closed. After Close,
// Publish and Subscribe both return errors.
func (b *Bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, list := range b.subs {
		for _, sub := range list {
			sub.close()
		}
	}
	b.subs = make(map[string][]*subscription)
	return nil
}

func (s *subscription) deliver(msg bus.BusMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- msg:
	default:
		// Slow consumer: drop. The publisher does not block.
	}
}

func (s *subscription) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
}
```

- [ ] **Step 4: Format, vet, run tests**

```bash
gofmt -w internal/bus/inproc/inproc.go internal/bus/inproc/inproc_test.go
go vet ./internal/bus/...
go test -race -count=1 ./internal/bus/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bus/inproc/
git commit -m "feat(bus): in-process channel-fanout Bus backend"
```

---

## Task 4 — Marker token generator

**Files:**
- Create: `internal/conversation/marker.go`
- Test: `internal/conversation/marker_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/conversation/marker_test.go`:

```go
package conversation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewMarkerTokenFormat(t *testing.T) {
	tok := NewMarkerToken()
	require.True(t, strings.HasPrefix(tok, "TURN_END_"), "got %q", tok)
	require.Len(t, tok, len("TURN_END_")+12)
}

func TestNewMarkerTokenUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		tok := NewMarkerToken()
		require.False(t, seen[tok], "duplicate marker token: %s", tok)
		seen[tok] = true
	}
}

func TestContainsMarker(t *testing.T) {
	tok := "TURN_END_abc123def456"
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"plain", tok, true},
		{"newline before", "hello world\n" + tok, true},
		{"newline after", tok + "\n", true},
		{"surrounded", "stuff\n" + tok + "\nmore", true},
		{"absent", "no marker here", false},
		{"different marker", "TURN_END_xxxxxxxxxxxx", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ContainsMarker(tc.body, tok))
		})
	}
}

func TestStripMarkerAndAfter(t *testing.T) {
	tok := "TURN_END_abc123def456"
	body := "hello world\nmore content\n" + tok + "\ntrailing chatter"
	got, found := StripMarkerAndAfter(body, tok)
	require.True(t, found)
	require.Equal(t, "hello world\nmore content", strings.TrimRight(got, "\n"))
}

func TestStripMarkerAbsent(t *testing.T) {
	got, found := StripMarkerAndAfter("no marker", "TURN_END_abc123def456")
	require.False(t, found)
	require.Equal(t, "no marker", got)
}
```

- [ ] **Step 2: Run test (expect fail)**

```bash
go test -race -run "TestNewMarkerToken|TestContainsMarker|TestStripMarker" ./internal/conversation/...
```

Expected: build fails.

- [ ] **Step 3: Write the implementation**

Create `internal/conversation/marker.go`:

```go
// Package conversation contains the broker, envelope builder, and result
// types for vitis's A2A multi-turn conversations. It depends on internal/bus
// and internal/model. It does NOT depend on internal/peer to avoid an
// import cycle (peer transports import this package).
package conversation

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// MarkerPrefix is the literal prefix for every per-turn marker token.
const MarkerPrefix = "TURN_END_"

// markerSuffixBytes is the number of random bytes encoded into the suffix.
// 6 bytes -> 12 hex chars -> ~2.8e14 possible values per turn.
const markerSuffixBytes = 6

// NewMarkerToken returns a new randomized marker token of the form
// "TURN_END_<12 hex chars>". Crypto-random; unique per turn with vanishing
// collision probability.
func NewMarkerToken() string {
	buf := make([]byte, markerSuffixBytes)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is essentially unrecoverable; if it ever
		// happens we'd rather crash than emit a non-random token that
		// could collide with conversation content.
		panic("crypto/rand failed: " + err.Error())
	}
	return MarkerPrefix + hex.EncodeToString(buf)
}

// ContainsMarker reports whether body contains the literal marker token.
// Returns false if either argument is empty.
func ContainsMarker(body, token string) bool {
	if body == "" || token == "" {
		return false
	}
	return strings.Contains(body, token)
}

// StripMarkerAndAfter returns body truncated at the first occurrence of
// token. Trailing content (and the marker itself) is dropped. The boolean
// reports whether the marker was found.
func StripMarkerAndAfter(body, token string) (string, bool) {
	if token == "" {
		return body, false
	}
	idx := strings.Index(body, token)
	if idx < 0 {
		return body, false
	}
	return body[:idx], true
}
```

- [ ] **Step 4: Format, vet, test**

```bash
gofmt -w internal/conversation/marker.go internal/conversation/marker_test.go
go vet ./internal/conversation/...
go test -race -run "TestNewMarkerToken|TestContainsMarker|TestStripMarker" ./internal/conversation/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/conversation/marker.go internal/conversation/marker_test.go
git commit -m "feat(conversation): per-turn marker token generator and parser"
```

---

## Task 5 — Briefing template

**Files:**
- Create: `internal/conversation/briefing.go`

This task ships without its own test file — it's a small pure function tested in Task 6 (envelope) since the briefing is only meaningful in the context of an envelope.

- [ ] **Step 1: Write the briefing renderer**

Create `internal/conversation/briefing.go`:

```go
package conversation

import (
	"fmt"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
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
type BriefingInput struct {
	Slot       model.PeerSlot
	MaxTurns   int
	Terminator model.TerminatorSpec
}

// RenderBriefing produces the system briefing text injected at the top of
// a peer's first envelope. Pure function; no side effects.
func RenderBriefing(in BriefingInput) string {
	var b strings.Builder
	b.WriteString("You are participating in a multi-turn conversation with another AI agent through Vitis.\n")
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
	return b.String()
}
```

- [ ] **Step 2: Format and vet**

```bash
gofmt -w internal/conversation/briefing.go
go vet ./internal/conversation/...
go build ./internal/conversation/...
```

Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/conversation/briefing.go
git commit -m "feat(conversation): briefing template renderer for sentinel and judge modes"
```

---

## Task 6 — Envelope builder

**Files:**
- Create: `internal/conversation/envelope.go`
- Test: `internal/conversation/envelope_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/conversation/envelope_test.go`:

```go
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
```

- [ ] **Step 2: Run test (expect fail)**

```bash
go test -race -run "TestBuildEnvelope|TestEnvelopeBody" ./internal/conversation/...
```

Expected: build fails with `undefined: BuildEnvelopeTurn1` etc.

- [ ] **Step 3: Write the implementation**

Create `internal/conversation/envelope.go`:

```go
package conversation

import (
	"fmt"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// BuildEnvelopeTurn1 constructs the very first envelope for a peer entering
// a conversation. It includes the per-peer briefing and the seed text. The
// turn index is 1; the synthetic sender label is "seed".
func BuildEnvelopeTurn1(conv model.Conversation, slot model.PeerSlot, marker string) model.Envelope {
	briefing := RenderBriefing(BriefingInput{
		Slot:       slot,
		MaxTurns:   conv.MaxTurns,
		Terminator: conv.Terminator,
	})
	seed := seedFor(conv, slot)
	body := renderBody(conv.ID, 1, conv.MaxTurns, "seed", seed, marker)
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
	fmt.Fprintf(&b, "When you finish your reply, output the token %s on its own line.", marker)
	return b.String()
}
```

- [ ] **Step 4: Format, vet, test**

```bash
gofmt -w internal/conversation/envelope.go internal/conversation/envelope_test.go
go vet ./internal/conversation/...
go test -race -run "TestBuildEnvelope|TestEnvelopeBody" ./internal/conversation/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/conversation/envelope.go internal/conversation/envelope_test.go
git commit -m "feat(conversation): envelope builder for turn 1 and turn N"
```

---

## Task 7 — Final result type

**Files:**
- Create: `internal/conversation/result.go`

No test of its own — it's a value type used by Task 11 (broker) which has comprehensive tests covering it.

- [ ] **Step 1: Write the file**

Create `internal/conversation/result.go`:

```go
package conversation

import "github.com/kamilandrzejrybacki-inc/vitis/internal/model"

// FinalResult is the JSON shape returned by `vitis converse` after a
// conversation reaches a terminal status. It bundles the conversation
// summary, the full turn log, a human-readable terminator note, and any
// warnings collected during the run.
type FinalResult struct {
	Conversation   model.Conversation        `json:"conversation"`
	Turns          []model.ConversationTurn  `json:"turns"`
	TerminatorNote string                    `json:"terminator_note,omitempty"`
	Warnings       []string                  `json:"warnings,omitempty"`
}
```

- [ ] **Step 2: Format, vet**

```bash
gofmt -w internal/conversation/result.go
go vet ./internal/conversation/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/conversation/result.go
git commit -m "feat(conversation): FinalResult value type"
```

---

## Task 8 — Peer transport interface

**Files:**
- Create: `internal/peer/transport.go`
- Create: `internal/peer/mock/mock.go` (test-only mock used by Tasks 11 and beyond)

- [ ] **Step 1: Write the interface**

Create `internal/peer/transport.go`:

```go
// Package peer defines the PeerTransport interface used by the conversation
// broker. Concrete implementations live in subpackages:
//
//   internal/peer/provider     - local persistent PTY peer (Plan 2)
//   internal/peer/remote  - remote vitis peer over the bus (Plan 4)
//   internal/peer/stdio        - this process's stdin/stdout (Plan 5)
//   internal/peer/mock         - scripted in-memory transport for tests
//
// The broker only ever talks to PeerTransport. It never imports a concrete
// transport package; CLI wiring builds the transport and passes it in.
package peer

import (
	"context"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// PeerTransport is the broker's view of a participant in a conversation.
//
// Lifecycle:
//   1. Start brings the peer online and returns when it is ready to receive
//      its first envelope. For provider transports this means spawning a
//      PTY and waiting for the adapter's ready signal. For network transports
//      it means handshaking over the bus. Idempotent within a single
//      conversation; calling Start twice is a programming error.
//   2. Deliver hands one envelope to the peer and blocks until either the
//      response turn is captured (success) or an error occurs. Deliver is
//      called serially by the broker — at most one Deliver in flight at a
//      time per peer per conversation.
//   3. Stop terminates the peer with a grace period. After Stop, neither
//      Deliver nor Start may be called.
type PeerTransport interface {
	Start(ctx context.Context, spec model.PeerSpec, b bus.Bus, conversationID string, slot model.PeerSlot) error
	Deliver(ctx context.Context, env model.Envelope) (model.ConversationTurn, error)
	Stop(ctx context.Context, grace time.Duration) error
}
```

- [ ] **Step 2: Write the mock transport**

Create `internal/peer/mock/mock.go`:

```go
// Package mock provides a scripted in-memory PeerTransport used by broker
// tests. It is intentionally not in a _test.go file so that other test
// packages (e.g. integration tests) can import it.
package mock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// Script is the canned exchange this mock will execute. Responses are
// consumed in order; if Deliver is called more times than there are
// scripted responses, the mock returns an error.
type Script struct {
	Responses []string
	Err       error // if non-nil, Deliver returns this error on the first call
}

// PeerTransport is the scripted mock implementation.
type PeerTransport struct {
	mu              sync.Mutex
	script          Script
	delivered       int
	started         bool
	stopped         bool
	conversationID  string
	slot            model.PeerSlot
	envelopeHistory []model.Envelope
}

// New constructs a mock peer transport from a Script.
func New(script Script) *PeerTransport {
	return &PeerTransport{script: script}
}

// EnvelopeHistory returns a copy of every envelope delivered so far.
func (p *PeerTransport) EnvelopeHistory() []model.Envelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]model.Envelope, len(p.envelopeHistory))
	copy(out, p.envelopeHistory)
	return out
}

func (p *PeerTransport) Start(_ context.Context, _ model.PeerSpec, _ bus.Bus, conversationID string, slot model.PeerSlot) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.started {
		return errors.New("mock peer: already started")
	}
	p.started = true
	p.conversationID = conversationID
	p.slot = slot
	return nil
}

func (p *PeerTransport) Deliver(_ context.Context, env model.Envelope) (model.ConversationTurn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.started {
		return model.ConversationTurn{}, errors.New("mock peer: deliver before start")
	}
	if p.stopped {
		return model.ConversationTurn{}, errors.New("mock peer: deliver after stop")
	}
	if p.script.Err != nil {
		err := p.script.Err
		p.script.Err = nil
		return model.ConversationTurn{}, err
	}
	if p.delivered >= len(p.script.Responses) {
		return model.ConversationTurn{}, fmt.Errorf("mock peer: script exhausted at delivery %d", p.delivered+1)
	}
	resp := p.script.Responses[p.delivered]
	p.delivered++
	p.envelopeHistory = append(p.envelopeHistory, env)
	now := time.Now().UTC()
	return model.ConversationTurn{
		ConversationID:       p.conversationID,
		Index:                env.TurnIndex,
		From:                 p.slot,
		Envelope:             env.Body,
		Response:             resp,
		MarkerToken:          env.MarkerToken,
		StartedAt:            now,
		EndedAt:              now,
		CompletionConfidence: 1.0,
		ParserConfidence:     1.0,
	}, nil
}

func (p *PeerTransport) Stop(_ context.Context, _ time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopped = true
	return nil
}
```

- [ ] **Step 3: Format, vet, build**

```bash
gofmt -w internal/peer/transport.go internal/peer/mock/mock.go
go vet ./internal/peer/...
go build ./internal/peer/...
```

Expected: build succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/peer/
git commit -m "feat(peer): PeerTransport interface and scripted mock implementation"
```

---

## Task 9 — Terminator interface and sentinel implementation

**Files:**
- Create: `internal/terminator/terminator.go`
- Create: `internal/terminator/sentinel.go`
- Test: `internal/terminator/sentinel_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/terminator/sentinel_test.go`:

```go
package terminator

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func TestSentinelDetectsAndPublishesVerdict(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	conv := model.Conversation{
		ID:         "conv-1",
		Terminator: model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
	}
	term := NewSentinel("<<END>>")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, term.Start(ctx, conv, b))
	defer term.Stop(context.Background())

	ctlSub, ctlCancel, err := b.Subscribe(ctx, bus.TopicControl(conv.ID))
	require.NoError(t, err)
	defer ctlCancel()

	turn := model.ConversationTurn{
		ConversationID: conv.ID,
		Index:          2,
		Response:       "I think we're done here.\n<<END>>",
	}
	payload, _ := json.Marshal(turn)
	require.NoError(t, b.Publish(ctx, bus.TopicTurn(conv.ID), bus.BusMessage{
		ConversationID: conv.ID,
		Topic:          bus.TopicTurn(conv.ID),
		Kind:           bus.KindTurn,
		Payload:        payload,
		Timestamp:      time.Now(),
	}))

	select {
	case msg := <-ctlSub:
		require.Equal(t, bus.KindControl, msg.Kind)
		var ctl bus.ControlMsg
		require.NoError(t, json.Unmarshal(msg.Payload, &ctl))
		require.Equal(t, bus.ControlVerdict, ctl.Kind)
		require.NotNil(t, ctl.Verdict)
		require.Equal(t, "terminate", ctl.Verdict.Decision)
		require.Equal(t, model.ConvCompletedSentinel, ctl.Verdict.Status)
	case <-time.After(time.Second):
		t.Fatal("expected verdict on control topic")
	}
}

func TestSentinelIgnoresAbsentSentinel(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	conv := model.Conversation{ID: "conv-1"}
	term := NewSentinel("<<END>>")
	ctx := context.Background()
	require.NoError(t, term.Start(ctx, conv, b))
	defer term.Stop(context.Background())

	ctlSub, ctlCancel, err := b.Subscribe(ctx, bus.TopicControl(conv.ID))
	require.NoError(t, err)
	defer ctlCancel()

	payload, _ := json.Marshal(model.ConversationTurn{Response: "still going"})
	require.NoError(t, b.Publish(ctx, bus.TopicTurn(conv.ID), bus.BusMessage{
		ConversationID: conv.ID,
		Topic:          bus.TopicTurn(conv.ID),
		Kind:           bus.KindTurn,
		Payload:        payload,
	}))

	select {
	case <-ctlSub:
		t.Fatal("should not have published a verdict")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSentinelStripFromResponse(t *testing.T) {
	require.Equal(t, "I'm done.", StripSentinel("I'm done.\n<<END>>", "<<END>>"))
	require.Equal(t, "still talking", StripSentinel("still talking", "<<END>>"))
	require.Equal(t, "before", StripSentinel("before<<END>>after", "<<END>>"))
}
```

- [ ] **Step 2: Run test (expect fail)**

```bash
go test -race -run "TestSentinel" ./internal/terminator/...
```

Expected: build fails (`undefined: NewSentinel`, `undefined: StripSentinel`).

- [ ] **Step 3: Write the terminator interface**

Create `internal/terminator/terminator.go`:

```go
// Package terminator defines the Terminator interface and ships built-in
// implementations. Terminators run as bus subscribers, watching the turn
// topic for a configured signal and publishing a Verdict (wrapped in a
// ControlMsg) to the control topic when they decide a conversation should
// end. Terminators do not interact with peers directly.
package terminator

import (
	"context"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// Terminator is the interface implemented by sentinel and judge strategies.
type Terminator interface {
	Start(ctx context.Context, conv model.Conversation, b bus.Bus) error
	Stop(ctx context.Context) error
}
```

- [ ] **Step 4: Write the sentinel implementation**

Create `internal/terminator/sentinel.go`:

```go
package terminator

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

const defaultSentinel = "<<END>>"

// Sentinel is a Terminator that publishes a terminate verdict the first
// time it sees the configured token in a turn response.
type Sentinel struct {
	token  string
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewSentinel returns a sentinel terminator using the given token. If the
// token is empty, the default "<<END>>" is used.
func NewSentinel(token string) *Sentinel {
	if token == "" {
		token = defaultSentinel
	}
	return &Sentinel{token: token}
}

// Start subscribes to the turn topic for the conversation and watches each
// turn response for the sentinel token. When found, it publishes a
// terminate verdict on the control topic.
func (s *Sentinel) Start(ctx context.Context, conv model.Conversation, b bus.Bus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return nil
	}
	turnSub, cancelSub, err := b.Subscribe(ctx, bus.TopicTurn(conv.ID))
	if err != nil {
		return err
	}
	runCtx, cancelRun := context.WithCancel(ctx)
	s.cancel = func() {
		cancelRun()
		cancelSub()
	}
	s.done = make(chan struct{})
	go s.loop(runCtx, conv.ID, b, turnSub)
	return nil
}

// Stop releases the subscription. Safe to call multiple times.
func (s *Sentinel) Stop(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel == nil {
		return nil
	}
	s.cancel()
	s.cancel = nil
	if s.done != nil {
		<-s.done
		s.done = nil
	}
	return nil
}

func (s *Sentinel) loop(ctx context.Context, conversationID string, b bus.Bus, turnSub <-chan bus.BusMessage) {
	defer close(s.done)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-turnSub:
			if !open {
				return
			}
			var turn model.ConversationTurn
			if err := json.Unmarshal(msg.Payload, &turn); err != nil {
				continue
			}
			if !strings.Contains(turn.Response, s.token) {
				continue
			}
			verdict := &model.Verdict{
				ConversationID: conversationID,
				Decision:       "terminate",
				Reason:         "sentinel token observed",
				Status:         model.ConvCompletedSentinel,
			}
			ctl := bus.ControlMsg{
				ConversationID: conversationID,
				Kind:           bus.ControlVerdict,
				Verdict:        verdict,
			}
			payload, err := json.Marshal(ctl)
			if err != nil {
				continue
			}
			_ = b.Publish(ctx, bus.TopicControl(conversationID), bus.BusMessage{
				ConversationID: conversationID,
				Topic:          bus.TopicControl(conversationID),
				Kind:           bus.KindControl,
				Payload:        payload,
			})
		}
	}
}

// StripSentinel removes the first occurrence of token from response and
// returns the result. Trailing whitespace introduced by the strip is
// trimmed. Used by the broker to clean a peer's response before
// constructing the next envelope so the sentinel never leaks across.
func StripSentinel(response, token string) string {
	if token == "" {
		token = defaultSentinel
	}
	idx := strings.Index(response, token)
	if idx < 0 {
		return response
	}
	return strings.TrimRight(response[:idx], " \t\r\n")
}
```

- [ ] **Step 5: Format, vet, test**

```bash
gofmt -w internal/terminator/
go vet ./internal/terminator/...
go test -race -run "TestSentinel" ./internal/terminator/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/terminator/
git commit -m "feat(terminator): Terminator interface and sentinel implementation"
```

---

## Task 10 — Store interface extension

**Files:**
- Modify: `internal/store/store.go`

This task is interface-only. The file backend implementation lands in Task 11. There is no test for this task; tests for the file backend in Task 11 cover the contract.

- [ ] **Step 1: Edit `internal/store/store.go`**

Add the four conversation methods to the `Store` interface. The final file should be:

```go
package store

import (
	"context"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

type Store interface {
	CreateSession(ctx context.Context, session model.Session) error
	UpdateSession(ctx context.Context, sessionID string, patch model.SessionPatch) error
	AppendTurn(ctx context.Context, turn model.Turn) error
	PeekTurns(ctx context.Context, sessionID string, lastN int) ([]model.Turn, error)
	AppendStreamEvent(ctx context.Context, event model.StoredStreamEvent) error

	// A2A conversation methods (additive — single-shot path is unaffected).
	CreateConversation(ctx context.Context, conv model.Conversation) error
	UpdateConversation(ctx context.Context, conversationID string, patch model.ConversationPatch) error
	AppendConversationTurn(ctx context.Context, turn model.ConversationTurn) error
	PeekConversationTurns(ctx context.Context, conversationID string, lastN int) ([]model.ConversationTurn, error)

	Close() error
}
```

- [ ] **Step 2: Format, vet**

```bash
gofmt -w internal/store/store.go
go vet ./internal/store/...
```

Note: this WILL break the build of `internal/store/file` and `internal/store/postgres` because the file Store no longer satisfies the interface. That is expected and is fixed in Task 11. Do NOT commit yet — Task 11 commits both together.

---

## Task 11 — File backend conversation persistence

**Files:**
- Modify: `internal/store/file/file_store.go`
- Test: `internal/store/file/file_store_conversation_test.go`
- Possibly: `internal/store/postgres/postgres_store.go` — add stub methods so the package still builds; full implementation is Plan 3

- [ ] **Step 1: Write the failing test**

Create `internal/store/file/file_store_conversation_test.go`:

```go
package file

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir, false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCreateAndUpdateConversation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	conv := model.Conversation{
		ID:        "conv-test",
		CreatedAt: now,
		Status:    model.ConvRunning,
		MaxTurns:  10,
		Opener:    model.PeerSlotA,
		PeerA:     model.PeerSpec{URI: "provider:claude-code"},
		PeerB:     model.PeerSpec{URI: "provider:codex"},
	}
	require.NoError(t, s.CreateConversation(ctx, conv))

	end := now.Add(time.Minute)
	status := model.ConvCompletedSentinel
	turns := 5
	require.NoError(t, s.UpdateConversation(ctx, "conv-test", model.ConversationPatch{
		Status:        &status,
		EndedAt:       &end,
		TurnsConsumed: &turns,
	}))
}

func TestAppendAndPeekConversationTurns(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	conv := model.Conversation{ID: "conv-x", Status: model.ConvRunning}
	require.NoError(t, s.CreateConversation(ctx, conv))

	for i := 1; i <= 5; i++ {
		require.NoError(t, s.AppendConversationTurn(ctx, model.ConversationTurn{
			ConversationID: "conv-x",
			Index:          i,
			From:           model.PeerSlotA,
			Envelope:       "env",
			Response:       "resp",
			MarkerToken:    "TURN_END_x",
			StartedAt:      time.Now().UTC(),
			EndedAt:        time.Now().UTC(),
		}))
	}

	all, err := s.PeekConversationTurns(ctx, "conv-x", 0)
	require.NoError(t, err)
	require.Len(t, all, 5)
	for i, turn := range all {
		require.Equal(t, i+1, turn.Index)
	}

	last2, err := s.PeekConversationTurns(ctx, "conv-x", 2)
	require.NoError(t, err)
	require.Len(t, last2, 2)
	require.Equal(t, 4, last2[0].Index)
	require.Equal(t, 5, last2[1].Index)
}

func TestPeekUnknownConversation(t *testing.T) {
	s := newTestStore(t)
	turns, err := s.PeekConversationTurns(context.Background(), "nope", 10)
	require.NoError(t, err)
	require.Empty(t, turns)
}
```

- [ ] **Step 2: Run test (expect fail)**

```bash
go test -race -run "TestCreateAndUpdateConversation|TestAppendAndPeekConversationTurns|TestPeekUnknownConversation" ./internal/store/file/...
```

Expected: build fails (interface unsatisfied + methods undefined).

- [ ] **Step 3: Edit `internal/store/file/file_store.go`**

Add the following methods to the `Store` struct, plus the helper paths. Leave existing code unchanged.

Append at the bottom of the file (before any final closing brace) — these are new methods on the existing `Store` type:

```go
// --- Conversation persistence (A2A) ---

func (s *Store) CreateConversation(_ context.Context, conv model.Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.conversationsDir(), 0o700); err != nil {
		return fmt.Errorf("mkdir conversations: %w", err)
	}
	return s.writeJSONAtomic(s.conversationPath(conv.ID), conv)
}

func (s *Store) UpdateConversation(_ context.Context, conversationID string, patch model.ConversationPatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var conv model.Conversation
	if err := s.readJSON(s.conversationPath(conversationID), &conv); err != nil {
		return err
	}
	if patch.Status != nil {
		conv.Status = *patch.Status
	}
	if patch.EndedAt != nil {
		conv.EndedAt = patch.EndedAt
	}
	if patch.TurnsConsumed != nil {
		conv.TurnsConsumed = *patch.TurnsConsumed
	}
	return s.writeJSONAtomic(s.conversationPath(conversationID), conv)
}

func (s *Store) AppendConversationTurn(_ context.Context, turn model.ConversationTurn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.conversationsDir(), 0o700); err != nil {
		return fmt.Errorf("mkdir conversations: %w", err)
	}
	return s.appendJSONL(s.conversationTurnPath(turn.ConversationID), turn)
}

func (s *Store) PeekConversationTurns(_ context.Context, conversationID string, lastN int) ([]model.ConversationTurn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.conversationTurnPath(conversationID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open conversation turns: %w", err)
	}
	defer file.Close()

	var turns []model.ConversationTurn
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var turn model.ConversationTurn
		if err := json.Unmarshal(scanner.Bytes(), &turn); err != nil {
			return nil, fmt.Errorf("decode conversation turn: %w", err)
		}
		turns = append(turns, turn)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan conversation turns: %w", err)
	}
	sort.Slice(turns, func(i, j int) bool { return turns[i].Index < turns[j].Index })
	if lastN > 0 && len(turns) > lastN {
		turns = turns[len(turns)-lastN:]
	}
	return turns, nil
}

func (s *Store) conversationsDir() string { return filepath.Join(s.root, "conversations") }
func (s *Store) conversationPath(id string) string {
	return filepath.Join(s.conversationsDir(), id+".json")
}
func (s *Store) conversationTurnPath(id string) string {
	return filepath.Join(s.conversationsDir(), id+".jsonl")
}
```

- [ ] **Step 4: Patch the postgres store with stub implementations**

The Postgres backend lives at `internal/store/postgres/postgres_store.go`. We need to add stubs that return `errors.New("conversation persistence not implemented in postgres backend yet")` so the build passes; full implementation is Plan 3. Read the file first, find the existing struct's method block, append:

```go
func (s *Store) CreateConversation(ctx context.Context, conv model.Conversation) error {
	return errors.New("postgres backend: CreateConversation not implemented in M1 (plan 3 adds it)")
}

func (s *Store) UpdateConversation(ctx context.Context, conversationID string, patch model.ConversationPatch) error {
	return errors.New("postgres backend: UpdateConversation not implemented in M1 (plan 3 adds it)")
}

func (s *Store) AppendConversationTurn(ctx context.Context, turn model.ConversationTurn) error {
	return errors.New("postgres backend: AppendConversationTurn not implemented in M1 (plan 3 adds it)")
}

func (s *Store) PeekConversationTurns(ctx context.Context, conversationID string, lastN int) ([]model.ConversationTurn, error) {
	return nil, errors.New("postgres backend: PeekConversationTurns not implemented in M1 (plan 3 adds it)")
}
```

If the postgres file does not already import `errors`, add it.

- [ ] **Step 5: Format, vet, build, test**

```bash
gofmt -w internal/store/file/file_store.go internal/store/file/file_store_conversation_test.go internal/store/postgres/postgres_store.go
go vet ./internal/store/...
go build ./...
go test -race -count=1 ./internal/store/...
```

Expected: build succeeds, all store tests (existing + new) pass.

- [ ] **Step 6: Commit**

```bash
git add internal/store/store.go internal/store/file/file_store.go internal/store/file/file_store_conversation_test.go internal/store/postgres/postgres_store.go
git commit -m "feat(store): conversation persistence on Store interface and file backend"
```

---

## Task 12 — Conversation Broker state machine

**Files:**
- Create: `internal/conversation/broker.go`
- Test: `internal/conversation/broker_test.go`

This is the core state machine. Tests cover: alternation, turn counter, max-turns hard cap, sentinel termination via real terminator, peer error → ConvError finalization, context cancellation → ConvInterrupted.

- [ ] **Step 1: Write the failing test**

Create `internal/conversation/broker_test.go`:

```go
package conversation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/peer/mock"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminator"
)

type discardStore struct{}

func (discardStore) CreateConversation(_ context.Context, _ model.Conversation) error {
	return nil
}
func (discardStore) UpdateConversation(_ context.Context, _ string, _ model.ConversationPatch) error {
	return nil
}
func (discardStore) AppendConversationTurn(_ context.Context, _ model.ConversationTurn) error {
	return nil
}

func newConv(maxTurns int) model.Conversation {
	return model.Conversation{
		ID:             "conv-test",
		CreatedAt:      time.Now().UTC(),
		Status:         model.ConvRunning,
		MaxTurns:       maxTurns,
		PerTurnTimeout: 10 * time.Second,
		OverallTimeout: 60 * time.Second,
		Terminator:     model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
		Opener:         model.PeerSlotA,
		PeerA:          model.PeerSpec{URI: "mock:a"},
		PeerB:          model.PeerSpec{URI: "mock:b"},
		SeedA:          "Discuss",
		SeedB:          "Discuss",
	}
}

func TestBrokerStrictAlternation(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Responses: []string{"a1", "a2", "a3"}})
	bb := mock.New(mock.Script{Responses: []string{"b1", "b2"}})
	conv := newConv(5)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, model.ConvMaxTurnsHit, res.Conversation.Status)
	require.Equal(t, 5, len(res.Turns))
	// Turns 1,3,5 from A; turns 2,4 from B (opener=A)
	require.Equal(t, model.PeerSlotA, res.Turns[0].From)
	require.Equal(t, model.PeerSlotB, res.Turns[1].From)
	require.Equal(t, model.PeerSlotA, res.Turns[2].From)
	require.Equal(t, model.PeerSlotB, res.Turns[3].From)
	require.Equal(t, model.PeerSlotA, res.Turns[4].From)
}

func TestBrokerSentinelTermination(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Responses: []string{"hello", "I think we agree.\n<<END>>"}})
	bb := mock.New(mock.Script{Responses: []string{"yes hello"}})
	conv := newConv(50)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, model.ConvCompletedSentinel, res.Conversation.Status)
	require.Equal(t, 3, len(res.Turns)) // a, b, a-with-sentinel
	require.Contains(t, res.Turns[2].Response, "<<END>>")
}

func TestBrokerOpenerB(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Responses: []string{"a-reply"}})
	bb := mock.New(mock.Script{Responses: []string{"b-opens"}})
	conv := newConv(2)
	conv.Opener = model.PeerSlotB
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx := context.Background()
	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, model.PeerSlotB, res.Turns[0].From)
	require.Equal(t, model.PeerSlotA, res.Turns[1].From)
}

func TestBrokerPeerErrorFinalizes(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Err: errForTesting("boom")})
	bb := mock.New(mock.Script{Responses: []string{"unused"}})
	conv := newConv(5)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx := context.Background()
	res, err := br.Run(ctx)
	require.NoError(t, err) // peer errors finalize the conversation, not bubble up
	require.Equal(t, model.ConvError, res.Conversation.Status)
	require.Empty(t, res.Turns)
}

func TestBrokerContextCancellation(t *testing.T) {
	b := inproc.New()
	defer b.Close()
	a := mock.New(mock.Script{Responses: []string{"a1", "a2", "a3", "a4"}})
	bb := mock.New(mock.Script{Responses: []string{"b1", "b2", "b3", "b4"}})
	conv := newConv(100)
	br := NewBroker(BrokerDeps{
		Conversation: conv,
		PeerA:        a,
		PeerB:        bb,
		Terminator:   terminator.NewSentinel("<<END>>"),
		Bus:          b,
		Store:        discardStore{},
	})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	res, err := br.Run(ctx)
	require.NoError(t, err)
	require.Equal(t, model.ConvInterrupted, res.Conversation.Status)
}

type stringErr string

func (e stringErr) Error() string { return string(e) }
func errForTesting(msg string) error { return stringErr(msg) }
```

- [ ] **Step 2: Run test (expect fail)**

```bash
go test -race -run "TestBroker" ./internal/conversation/...
```

Expected: build fails with `undefined: NewBroker` etc.

- [ ] **Step 3: Write the broker**

Create `internal/conversation/broker.go`:

```go
package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/peer"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminator"
)

// ConversationStore is the narrow store interface the broker depends on.
// It is a subset of the full store.Store; the broker takes only the
// conversation methods to keep the dependency narrow and to make
// broker tests trivial to mock.
type ConversationStore interface {
	CreateConversation(ctx context.Context, conv model.Conversation) error
	UpdateConversation(ctx context.Context, conversationID string, patch model.ConversationPatch) error
	AppendConversationTurn(ctx context.Context, turn model.ConversationTurn) error
}

// BrokerDeps bundles the dependencies needed to construct a Broker.
type BrokerDeps struct {
	Conversation model.Conversation
	PeerA        peer.PeerTransport
	PeerB        peer.PeerTransport
	Terminator   terminator.Terminator
	Bus          bus.Bus
	Store        ConversationStore
}

// Broker is the conversation state machine.
type Broker struct {
	deps BrokerDeps
}

// NewBroker constructs a Broker from its dependencies.
func NewBroker(deps BrokerDeps) *Broker {
	return &Broker{deps: deps}
}

// Run drives the conversation to completion. It returns a FinalResult with
// the conversation status and turn log. Errors are reflected in the
// conversation status (ConvError, ConvPeerCrashed, etc.); they are NOT
// returned as a Go error from Run unless something catastrophic happens
// during finalization (e.g. cannot publish to the bus at all).
func (b *Broker) Run(ctx context.Context) (FinalResult, error) {
	conv := b.deps.Conversation
	conv.Status = model.ConvRunning
	conv.CreatedAt = time.Now().UTC()

	// Best-effort store create. Failures are non-blocking warnings.
	warnings := []string{}
	if err := b.deps.Store.CreateConversation(ctx, conv); err != nil {
		warnings = append(warnings, fmt.Sprintf("store create_conversation: %v", err))
	}

	// Start both peers.
	if err := b.deps.PeerA.Start(ctx, conv.PeerA, b.deps.Bus, conv.ID, model.PeerSlotA); err != nil {
		return b.finalize(ctx, conv, nil, warnings, model.ConvError, fmt.Sprintf("peer A start: %v", err))
	}
	if err := b.deps.PeerB.Start(ctx, conv.PeerB, b.deps.Bus, conv.ID, model.PeerSlotB); err != nil {
		_ = b.deps.PeerA.Stop(ctx, time.Second)
		return b.finalize(ctx, conv, nil, warnings, model.ConvError, fmt.Sprintf("peer B start: %v", err))
	}
	defer func() {
		_ = b.deps.PeerA.Stop(ctx, time.Second)
		_ = b.deps.PeerB.Stop(ctx, time.Second)
	}()

	// Start the terminator.
	if err := b.deps.Terminator.Start(ctx, conv, b.deps.Bus); err != nil {
		warnings = append(warnings, fmt.Sprintf("terminator start: %v", err))
	}
	defer b.deps.Terminator.Stop(context.Background())

	// Subscribe to control topic.
	ctlSub, ctlCancel, err := b.deps.Bus.Subscribe(ctx, bus.TopicControl(conv.ID))
	if err != nil {
		return b.finalize(ctx, conv, nil, warnings, model.ConvError, fmt.Sprintf("control subscribe: %v", err))
	}
	defer ctlCancel()

	turns := make([]model.ConversationTurn, 0, conv.MaxTurns)
	active := conv.Opener
	if active != model.PeerSlotA && active != model.PeerSlotB {
		active = model.PeerSlotA
	}

	envelope := BuildEnvelopeTurn1(conv, active, NewMarkerToken())

	for {
		select {
		case <-ctx.Done():
			conv.TurnsConsumed = len(turns)
			return b.finalize(ctx, conv, turns, warnings, model.ConvInterrupted, "context cancelled")
		default:
		}

		turn, err := b.transportFor(active).Deliver(ctx, envelope)
		if err != nil {
			conv.TurnsConsumed = len(turns)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return b.finalize(ctx, conv, turns, warnings, model.ConvInterrupted, err.Error())
			}
			return b.finalize(ctx, conv, turns, warnings, model.ConvError, fmt.Sprintf("peer %s deliver: %v", active, err))
		}

		// Persist & publish the turn.
		if err := b.deps.Store.AppendConversationTurn(ctx, turn); err != nil {
			warnings = append(warnings, fmt.Sprintf("store append_turn: %v", err))
		}
		if err := b.publishTurn(ctx, turn); err != nil {
			warnings = append(warnings, fmt.Sprintf("publish turn: %v", err))
		}

		turns = append(turns, turn)
		conv.TurnsConsumed = len(turns)

		// Hard max-turns cap.
		if conv.TurnsConsumed >= conv.MaxTurns {
			return b.finalize(ctx, conv, turns, warnings, model.ConvMaxTurnsHit, "max turns reached")
		}

		// Drain control non-blocking.
		drained := drainControl(ctlSub)
		for _, ctl := range drained {
			switch ctl.Kind {
			case bus.ControlVerdict:
				if ctl.Verdict != nil && ctl.Verdict.Decision == "terminate" {
					return b.finalize(ctx, conv, turns, warnings, ctl.Verdict.Status, ctl.Verdict.Reason)
				}
			case bus.ControlPeerCrashed:
				return b.finalize(ctx, conv, turns, warnings, model.ConvPeerCrashed, fmt.Sprintf("peer %s crashed: %s", ctl.Slot, ctl.Detail))
			case bus.ControlPeerBlocked:
				return b.finalize(ctx, conv, turns, warnings, model.ConvPeerBlocked, fmt.Sprintf("peer %s blocked: %s", ctl.Slot, ctl.Detail))
			}
		}

		// Build next envelope for the other peer using the (possibly stripped) response.
		nextResponse := turn.Response
		if conv.Terminator.Kind == "sentinel" {
			nextResponse = terminator.StripSentinel(nextResponse, conv.Terminator.Sentinel)
		}
		next := active.Other()
		envelope = BuildEnvelopeTurnN(conv, conv.TurnsConsumed+1, active, nextResponse, NewMarkerToken())
		active = next
	}
}

func (b *Broker) transportFor(slot model.PeerSlot) peer.PeerTransport {
	if slot == model.PeerSlotA {
		return b.deps.PeerA
	}
	return b.deps.PeerB
}

func (b *Broker) publishTurn(ctx context.Context, turn model.ConversationTurn) error {
	payload, err := json.Marshal(turn)
	if err != nil {
		return err
	}
	return b.deps.Bus.Publish(ctx, bus.TopicTurn(turn.ConversationID), bus.BusMessage{
		ConversationID: turn.ConversationID,
		Topic:          bus.TopicTurn(turn.ConversationID),
		Kind:           bus.KindTurn,
		Payload:        payload,
		Timestamp:      time.Now().UTC(),
	})
}

func (b *Broker) finalize(ctx context.Context, conv model.Conversation, turns []model.ConversationTurn, warnings []string, status model.ConversationStatus, note string) (FinalResult, error) {
	now := time.Now().UTC()
	conv.Status = status
	conv.EndedAt = &now
	if conv.TurnsConsumed == 0 && len(turns) > 0 {
		conv.TurnsConsumed = len(turns)
	}
	if err := b.deps.Store.UpdateConversation(ctx, conv.ID, model.ConversationPatch{
		Status:        &status,
		EndedAt:       &now,
		TurnsConsumed: &conv.TurnsConsumed,
	}); err != nil {
		warnings = append(warnings, fmt.Sprintf("store update_conversation: %v", err))
	}
	return FinalResult{
		Conversation:   conv,
		Turns:          turns,
		TerminatorNote: note,
		Warnings:       warnings,
	}, nil
}

// drainControl pulls every currently buffered control message off the
// channel without blocking and returns them in arrival order.
func drainControl(ch <-chan bus.BusMessage) []bus.ControlMsg {
	var out []bus.ControlMsg
	for {
		select {
		case msg, open := <-ch:
			if !open {
				return out
			}
			var ctl bus.ControlMsg
			if err := json.Unmarshal(msg.Payload, &ctl); err == nil {
				out = append(out, ctl)
			}
		default:
			return out
		}
	}
}
```

- [ ] **Step 4: Format, vet, build, test**

```bash
gofmt -w internal/conversation/broker.go internal/conversation/broker_test.go
go vet ./internal/conversation/...
go build ./...
go test -race -count=1 ./internal/conversation/...
```

Expected: PASS for all broker tests. (The sentinel termination test has a tricky timing element — the terminator subscriber must be subscribed before the broker publishes the third turn. This works because the broker calls `Terminator.Start` before its first `Deliver`.)

- [ ] **Step 5: Commit**

```bash
git add internal/conversation/broker.go internal/conversation/broker_test.go
git commit -m "feat(conversation): Broker state machine with strict alternation, max-turns cap, sentinel termination, error finalization"
```

---

## Task 13 — Whole-suite green check

**Files:** none

- [ ] **Step 1: Run the entire test suite with race detector**

```bash
go test -race -count=1 ./...
```

Expected: ALL packages PASS. If anything fails, STOP and fix it before declaring Plan 1 complete.

- [ ] **Step 2: Run go vet on the whole tree**

```bash
go vet ./...
```

Expected: no warnings.

- [ ] **Step 3: Run build to confirm**

```bash
go build ./...
```

Expected: success.

- [ ] **Step 4: Tag Plan 1 complete in the plan file**

```bash
echo "<!-- execution-completed: $(date -Is) -->" >> docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md
git add docs/superpowers/plans/2026-04-07-a2a-plan-1-foundation.md
git commit -m "chore(plan): mark A2A plan 1 foundation execution complete"
```

Plan 1 ships. Hand control back to the orchestrator for Plan 2.

---

## Self-Review

Walking the spec section by section:

| Spec section | Plan 1 coverage | Plan covering remainder |
|---|---|---|
| §1 Overview | Foundation in place; conversation runtime works against mock peers | Plans 2–5 |
| §1a Product Boundary | N/A — no I/O |  |
| §2 Architecture | Broker, Bus, Terminator, Store, ConversationStore subset all in place | — |
| §3 Data model | Conversation, ConversationTurn, ConversationPatch, PeerSpec, TerminatorSpec, Verdict, Envelope all in `internal/model/conversation.go` | — |
| §4 Peer transport | PeerTransport interface + mock impl. Real provider/remote/stdio in Plans 2/4/5 | Plans 2/4/5 |
| §5 Broker | Strict alternation, hard max-turns, sentinel termination via real Terminator, error/cancel/crash control draining, finalize semantics | — |
| §6 Bus | Bus interface + inproc backend; NATS in Plan 4 | Plan 4 |
| §7 CLI | NOT in Plan 1 — Plan 2 ships `vitis converse` | Plan 2 |
| §8 Project layout | All M1 directories created except `peer/provider`, `peer/remote`, `peer/stdio`, `terminator/judge`, `bus/nats`, `cli/converse*`, `terminal/persistent` | Plans 2/3/4/5 |
| §9 Error model | ConvError, ConvPeerCrashed, ConvPeerBlocked, ConvInterrupted, ConvMaxTurnsHit, ConvCompletedSentinel all reachable in broker; warnings collected | — |
| §10 Testing | Unit + integration via mock peers; full race-detector suite green | Real PTY integration tests in Plan 2 |
| §11 Milestones | Implements the broker + sentinel + bus + file persistence portion of M1 | — |
| §12 Open questions | Marker format chosen: hex 12-char suffix (`TURN_END_<12 hex>`) — committed. Other open questions deferred to later plans | — |

**Placeholder scan:** every step shows real code or real commands. No `TODO`, `TBD`, or "implement later". The postgres stub methods explicitly say "not implemented in M1 (plan 3 adds it)" which is a documented deferral, not a placeholder.

**Type consistency check:** `ConversationStore` (broker-narrow) is a subset of `store.Store`. `BrokerDeps` field names match across broker.go and broker_test.go. `BuildEnvelopeTurn1` and `BuildEnvelopeTurnN` signatures match call sites. `NewSentinel`/`NewMarkerToken` signatures match call sites. `model.PeerSlotA.Other()` method exists and is used. `bus.TopicTurn`/`TopicControl`/`TopicEnvelopeIn` signatures match every call site. `bus.KindTurn`/`KindControl` enum values match every comparison. `bus.ControlVerdict`/`ControlPeerCrashed`/`ControlPeerBlocked` enum values match every comparison.

**Dispatch:** Plan 1 is ready for subagent-driven execution. The recommended path is to dispatch a fresh general-purpose subagent on the `claude-sonnet-4-6` model with the full plan file path and the instruction "execute every task in order, do not skip the commit step".

<!-- end of plan 1 -->

<!-- execution-started: 2026-04-07T23:04:11+02:00 -->
<!-- execution-completed: 2026-04-07T23:19:56+02:00 -->
