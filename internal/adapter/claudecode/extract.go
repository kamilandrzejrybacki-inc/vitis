package claudecode

import (
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter"
)

func (a *Adapter) ExtractResponse(_ []byte, normalizedTranscript string) adapter.ExtractionResult {
	text := strings.TrimSpace(normalizedTranscript)
	if text == "" {
		return adapter.ExtractionResult{
			Response:         "",
			ParserConfidence: 0.1,
			Notes:            []string{"empty transcript"},
		}
	}

	blocks := splitBlocks(text)
	for i := len(blocks) - 1; i >= 0; i-- {
		block := strings.TrimSpace(cleanBlock(blocks[i]))
		if block == "" {
			continue
		}
		if looksLikeNonResponse(block) {
			continue
		}
		confidence := 0.65
		if len(blocks) > 1 {
			confidence = 0.8
		}
		return adapter.ExtractionResult{
			Response:         block,
			ParserConfidence: confidence,
			Notes:            nil,
		}
	}

	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || looksLikeNonResponse(line) {
			continue
		}
		return adapter.ExtractionResult{
			Response:         line,
			ParserConfidence: 0.4,
			Notes:            []string{"fell back to last non-empty line"},
		}
	}

	return adapter.ExtractionResult{
		Response:         "",
		ParserConfidence: 0.1,
		Notes:            []string{"no extractable response found"},
	}
}

func splitBlocks(text string) []string {
	return strings.Split(text, "\n\n")
}

func cleanBlock(block string) string {
	lines := strings.Split(block, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Strip Claude Code's TUI prompt glyphs: ">" (ASCII fallback),
		// "›" (U+203A, single right-pointing angle quotation mark), and
		// "❯" (U+276F, heavy right-pointing angle quotation mark ornament
		// — the default ready-prompt glyph emitted by the PTY runtime).
		if strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "›") || strings.HasPrefix(trimmed, "❯") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func looksLikeNonResponse(block string) bool {
	lower := strings.ToLower(strings.TrimSpace(block))
	switch {
	case lower == "":
		return true
	case strings.HasPrefix(lower, "login"):
		return true
	case strings.Contains(lower, "press enter"):
		return true
	case strings.Contains(lower, "permission"):
		return true
	case strings.Contains(lower, "approve"):
		return true
	case strings.Contains(lower, "rate limit"):
		return true
	case strings.HasPrefix(lower, "claude>"):
		return true
	default:
		return false
	}
}
