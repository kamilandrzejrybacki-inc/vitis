package policy

import (
	"testing"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/stretchr/testify/require"
)

func TestParseNextTrailer(t *testing.T) {
	cases := []struct {
		name  string
		reply string
		want  *string
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
		{"CRLF ending", "text\r\n<<NEXT: bob>>\r\n", strPtr("bob")},
		{"empty reply", "", nil},
		{"only whitespace", "   \n  \n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseNextTrailer(c.reply)
			if c.want == nil {
				require.Nil(t, got, "expected nil, got %v", got)
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

func TestAddressedPolicyTwoPeerLegacyBehavior(t *testing.T) {
	// The 2-peer "a"/"b" legacy case must behave identically to strict
	// alternation under both addressed and fallback paths.
	p := NewAddressedPolicy()
	peers := []model.PeerID{"a", "b"}

	d := p.Next("a", "no trailer", peers)
	require.Equal(t, model.PeerID("b"), d.Next)

	d = p.Next("b", "no trailer", peers)
	require.Equal(t, model.PeerID("a"), d.Next)

	d = p.Next("a", "text\n<<NEXT: b>>", peers)
	require.Equal(t, model.PeerID("b"), d.Next)
	require.False(t, d.FallbackUsed)
}

func TestRoundRobinAfterUnknownCurrent(t *testing.T) {
	peers := []model.PeerID{"alice", "bob", "carol"}
	require.Equal(t, model.PeerID("alice"), roundRobinAfter("ghost", peers))
}

func TestRoundRobinAfterEmptyPanics(t *testing.T) {
	require.Panics(t, func() { roundRobinAfter("alice", nil) })
}
