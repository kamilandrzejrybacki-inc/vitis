package codex

import (
	"strings"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
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

	// Look for the last "codex" prefix line and extract content until "tokens used".
	lines := strings.Split(text, "\n")
	codexStart := -1
	tokensLine := -1

	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if tokensLine == -1 && tokenUsageLine.MatchString(trimmed) {
			tokensLine = i
		}
		if codexResponsePrefix.MatchString(trimmed) {
			codexStart = i
			break
		}
	}

	if codexStart != -1 {
		end := len(lines)
		if tokensLine != -1 && tokensLine > codexStart {
			end = tokensLine
		}
		responseLines := lines[codexStart+1 : end]
		response := strings.TrimSpace(strings.Join(responseLines, "\n"))
		if response != "" {
			return adapter.ExtractionResult{
				Response:         response,
				ParserConfidence: 0.9,
				Notes:            nil,
			}
		}
	}

	// Fallback: return the last non-empty paragraph.
	paragraphs := strings.Split(text, "\n\n")
	for i := len(paragraphs) - 1; i >= 0; i-- {
		paragraph := strings.TrimSpace(paragraphs[i])
		if paragraph == "" {
			continue
		}
		return adapter.ExtractionResult{
			Response:         paragraph,
			ParserConfidence: 0.4,
			Notes:            []string{"fell back to last non-empty paragraph"},
		}
	}

	return adapter.ExtractionResult{
		Response:         "",
		ParserConfidence: 0.1,
		Notes:            []string{"no extractable response found"},
	}
}
