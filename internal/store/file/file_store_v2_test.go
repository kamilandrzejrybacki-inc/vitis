package file

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/require"
)

func TestStampSchemaVersionV2(t *testing.T) {
	conv := model.Conversation{
		ID: "c1",
		Peers: []model.PeerParticipant{
			{ID: "alice", Spec: model.PeerSpec{URI: "provider:claude-code"}},
		},
	}
	stamped := stampSchemaVersion(conv)
	require.Equal(t, 2, stamped.SchemaVersion)
}

func TestStampSchemaVersionLeavesV1Untouched(t *testing.T) {
	conv := model.Conversation{
		ID:    "c1",
		PeerA: model.PeerSpec{URI: "provider:claude-code"},
		PeerB: model.PeerSpec{URI: "provider:codex"},
	}
	stamped := stampSchemaVersion(conv)
	require.Equal(t, 0, stamped.SchemaVersion, "pure 2-peer legacy write should not stamp v2")
}

func TestStampSchemaVersionPreservesExplicit(t *testing.T) {
	conv := model.Conversation{ID: "c1", SchemaVersion: 2}
	require.Equal(t, 2, stampSchemaVersion(conv).SchemaVersion)
}

func TestCreateConversationWritesSchemaVersionForV2(t *testing.T) {
	s := newTestConvStore(t)
	conv := model.Conversation{
		ID:     "conv-v2",
		Status: model.ConvRunning,
		Peers: []model.PeerParticipant{
			{ID: "alice", Spec: model.PeerSpec{URI: "x"}},
			{ID: "bob", Spec: model.PeerSpec{URI: "y"}},
		},
		Seeds:    map[model.PeerID]string{"alice": "go", "bob": "go"},
		OpenerID: "alice",
	}
	require.NoError(t, s.CreateConversation(context.Background(), conv))

	data, err := os.ReadFile(s.conversationPath("conv-v2"))
	require.NoError(t, err)
	require.True(t, strings.Contains(string(data), `"schema_version": 2`),
		"v2 conversation should have schema_version: 2 in JSON, got: %s", string(data))

	// Round-trip back through Go.
	var decoded model.Conversation
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, 2, decoded.SchemaVersion)
	require.Len(t, decoded.Peers, 2)
}

func TestCreateConversationLegacyOmitsSchemaVersion(t *testing.T) {
	s := newTestConvStore(t)
	conv := model.Conversation{
		ID:     "conv-legacy",
		Status: model.ConvRunning,
		PeerA:  model.PeerSpec{URI: "provider:claude-code"},
		PeerB:  model.PeerSpec{URI: "provider:codex"},
		Opener: model.PeerSlotA,
	}
	require.NoError(t, s.CreateConversation(context.Background(), conv))

	data, err := os.ReadFile(s.conversationPath("conv-legacy"))
	require.NoError(t, err)
	require.False(t, strings.Contains(string(data), `"schema_version"`),
		"legacy 2-peer conversation should NOT have schema_version field in JSON")
}
