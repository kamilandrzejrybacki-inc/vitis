package bus

import (
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/require"
)

func TestTopicEnvelopeInID(t *testing.T) {
	require.Equal(t, "conv/c1/peer/alice/in", TopicEnvelopeInID("c1", model.PeerID("alice")))
}

func TestTopicEnvelopeInIDDistinctFromSlotTopic(t *testing.T) {
	// The legacy slot-keyed helper and the new id-keyed helper produce
	// distinguishable topics so subscribers can opt in to one or the other
	// during the migration window.
	slotTopic := TopicEnvelopeIn("c1", model.PeerSlotA)
	idTopic := TopicEnvelopeInID("c1", model.PeerID("a"))
	require.Equal(t, "conv/c1/peer-a/in", slotTopic)
	require.Equal(t, "conv/c1/peer/a/in", idTopic)
	require.NotEqual(t, slotTopic, idTopic)
}
