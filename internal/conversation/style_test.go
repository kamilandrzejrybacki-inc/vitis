package conversation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func TestIsValidStyle(t *testing.T) {
	for _, s := range []string{"normal", "caveman-lite", "caveman-full", "caveman-ultra"} {
		require.True(t, IsValidStyle(s), "%q should be valid", s)
	}
	for _, s := range []string{"", "caveman", "lite", "ultra", "garbage", "CAVEMAN-FULL"} {
		require.False(t, IsValidStyle(s), "%q should be invalid", s)
	}
}

func TestRenderStyleInstructionsNormal(t *testing.T) {
	require.Empty(t, RenderStyleInstructions(StyleNormal))
	require.Empty(t, RenderStyleInstructions(""))
}

func TestRenderStyleInstructionsAllCavemanLevels(t *testing.T) {
	for name, style := range map[string]ReplyStyle{
		"lite":  StyleCavemanLite,
		"full":  StyleCavemanFull,
		"ultra": StyleCavemanUltra,
	} {
		t.Run(name, func(t *testing.T) {
			got := RenderStyleInstructions(style)
			require.NotEmpty(t, got, "%s should produce instructions", name)
			require.Contains(t, got, "caveman", "should mention caveman")
			require.Contains(t, got, "Drop:", "should include the Drop rule")
			require.Contains(t, got, "code blocks unchanged", "must preserve code-block escape clause")
		})
	}
}

func TestRenderStyleInstructionsLevelsDifferent(t *testing.T) {
	lite := RenderStyleInstructions(StyleCavemanLite)
	full := RenderStyleInstructions(StyleCavemanFull)
	ultra := RenderStyleInstructions(StyleCavemanUltra)
	require.NotEqual(t, lite, full)
	require.NotEqual(t, full, ultra)
	require.NotEqual(t, lite, ultra)

	require.Contains(t, lite, "LITE")
	require.Contains(t, full, "FULL")
	require.Contains(t, ultra, "ULTRA")
}

func TestRenderBriefingWithoutStyleHasNoCavemanRules(t *testing.T) {
	got := RenderBriefing(BriefingInput{
		Slot:       model.PeerSlotA,
		MaxTurns:   10,
		Terminator: model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
	})
	require.NotContains(t, got, "caveman")
	require.NotContains(t, got, "REPLY STYLE")
}

func TestRenderBriefingWithCavemanFull(t *testing.T) {
	got := RenderBriefing(BriefingInput{
		Slot:       model.PeerSlotA,
		MaxTurns:   10,
		Terminator: model.TerminatorSpec{Kind: "sentinel", Sentinel: "<<END>>"},
		Style:      StyleCavemanFull,
	})
	// The base briefing fields are still present.
	require.Contains(t, got, "peer-a")
	require.Contains(t, got, "<<END>>")
	require.Contains(t, got, "marker")
	// And the caveman block is appended.
	require.Contains(t, got, "REPLY STYLE")
	require.Contains(t, got, "caveman")
	require.Contains(t, got, "FULL")
}

func TestRenderBriefingCavemanIsAppendNotReplace(t *testing.T) {
	withStyle := RenderBriefing(BriefingInput{
		Slot:     model.PeerSlotA,
		MaxTurns: 10,
		Style:    StyleCavemanUltra,
	})
	withoutStyle := RenderBriefing(BriefingInput{
		Slot:     model.PeerSlotA,
		MaxTurns: 10,
	})
	require.True(t, strings.HasPrefix(withStyle, withoutStyle), "caveman block should append, not replace")
	require.True(t, len(withStyle) > len(withoutStyle))
}
