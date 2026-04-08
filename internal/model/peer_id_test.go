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

func TestPeerIDString(t *testing.T) {
	require.Equal(t, "alice", PeerID("alice").String())
}
