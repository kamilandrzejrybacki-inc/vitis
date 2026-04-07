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
