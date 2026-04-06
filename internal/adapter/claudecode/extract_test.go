package claudecode

import (
	"strings"
	"testing"
)

func TestExtractResponse(t *testing.T) {
	a := New()
	result := a.ExtractResponse(nil, "system noise\n\nFinal answer line 1\nFinal answer line 2\n")
	if result.Response != "Final answer line 1\nFinal answer line 2" {
		t.Fatalf("unexpected response: %q", result.Response)
	}
}

func TestExtract_EmptyTranscript(t *testing.T) {
	a := New()

	result := a.ExtractResponse(nil, "")
	if result.Response != "" {
		t.Errorf("expected empty response, got %q", result.Response)
	}
	if result.ParserConfidence != 0.1 {
		t.Errorf("expected low confidence for empty transcript, got %f", result.ParserConfidence)
	}

	result = a.ExtractResponse([]byte{}, "")
	if result.Response != "" {
		t.Errorf("expected empty response for empty raw+normalized, got %q", result.Response)
	}
}

func TestExtract_ANSIOnlyContent(t *testing.T) {
	a := New()
	// ANSI escape codes only — normalized transcript is effectively empty
	ansiOnly := "\x1b[0m\x1b[1;32m\x1b[0m"
	result := a.ExtractResponse([]byte(ansiOnly), "")
	if result.Response != "" {
		t.Errorf("expected empty response for ANSI-only content, got %q", result.Response)
	}
}

func TestExtract_MultipleBlankLineSeparators(t *testing.T) {
	a := New()
	// Three blocks separated by blank lines; extraction should pick last non-noise block
	transcript := "First block line 1\nFirst block line 2\n\nSecond block line\n\nThird block final answer"
	result := a.ExtractResponse(nil, transcript)
	if result.Response != "Third block final answer" {
		t.Errorf("expected last block, got %q", result.Response)
	}
	// Multiple blocks → higher confidence
	if result.ParserConfidence < 0.75 {
		t.Errorf("expected higher confidence for multi-block, got %f", result.ParserConfidence)
	}
}

func TestExtract_SingleLineResponse(t *testing.T) {
	a := New()
	result := a.ExtractResponse(nil, "Hello, world!")
	if result.Response != "Hello, world!" {
		t.Errorf("expected single-line response, got %q", result.Response)
	}
}

func TestExtract_NonResponseBlocksSkipped(t *testing.T) {
	a := New()

	noiseInputs := []string{
		"login",
		"Press enter to continue",
		"permission denied",
		"approve this action",
		"rate limit exceeded",
		"claude>",
	}

	for _, noise := range noiseInputs {
		result := a.ExtractResponse(nil, noise)
		// All noise blocks should be skipped; falls through to empty result
		if result.Response == noise {
			t.Errorf("noise %q should have been filtered, but was returned as response", noise)
		}
	}
}

func TestExtract_FallsBackToLastNonEmptyLine(t *testing.T) {
	a := New()
	// All blocks look like noise except one line that should be the fallback
	// Use a transcript where the block-level scan filters everything, but line scan finds something
	// "login" is a non-response prefix, but "my answer" is not
	transcript := "login\n\nmy answer"
	result := a.ExtractResponse(nil, transcript)
	// "my answer" is in the second block and is not noise — expect it to be returned from block scan
	if !strings.Contains(result.Response, "my answer") {
		t.Errorf("expected 'my answer' in response, got %q", result.Response)
	}
}

func TestExtract_CleanBlockStripsPromptLines(t *testing.T) {
	a := New()
	// Lines starting with ">" or "›" are stripped by cleanBlock
	transcript := "> prompt line\n› another prompt\nActual response text"
	result := a.ExtractResponse(nil, transcript)
	if result.Response != "Actual response text" {
		t.Errorf("expected prompt lines stripped, got %q", result.Response)
	}
}
