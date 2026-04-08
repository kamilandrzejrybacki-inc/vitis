package codex

import (
	"strings"
	"testing"
)

func TestExtract_HappyPath(t *testing.T) {
	content := loadFixture(t, "happy_path.txt")
	a := New()
	result := a.ExtractResponse(nil, content)
	if result.Response != "The answer is 4." {
		t.Errorf("unexpected response: %q", result.Response)
	}
	if result.ParserConfidence < 0.8 {
		t.Errorf("expected high confidence, got %f", result.ParserConfidence)
	}
}

func TestExtract_MultiTurn(t *testing.T) {
	content := loadFixture(t, "multi_turn.txt")
	a := New()
	result := a.ExtractResponse(nil, content)
	if result.Response != "The directory contains one file: README.md" {
		t.Errorf("unexpected response: %q", result.Response)
	}
	if result.ParserConfidence < 0.8 {
		t.Errorf("expected high confidence for multi-turn, got %f", result.ParserConfidence)
	}
}

func TestExtract_EmptyTranscript(t *testing.T) {
	a := New()
	result := a.ExtractResponse(nil, "")
	if result.Response != "" {
		t.Errorf("expected empty response, got %q", result.Response)
	}
	if result.ParserConfidence != 0.1 {
		t.Errorf("expected low confidence 0.1 for empty transcript, got %f", result.ParserConfidence)
	}
	if len(result.Notes) == 0 {
		t.Error("expected notes explaining empty result")
	}
}

func TestExtract_NoCodexSection(t *testing.T) {
	// Transcript without a "codex" section header — should fall back to last paragraph.
	transcript := "OpenAI Codex v0.1.2025060501 (research preview)\n--------\nuser\nWhat is 2+2?\n\nSome standalone answer without codex prefix"
	a := New()
	result := a.ExtractResponse(nil, transcript)
	if result.Response == "" {
		t.Error("expected fallback extraction to find last paragraph")
	}
	if !strings.Contains(result.Response, "Some standalone answer") {
		t.Errorf("expected fallback to last paragraph, got %q", result.Response)
	}
	// Fallback confidence should be lower.
	if result.ParserConfidence >= 0.8 {
		t.Errorf("expected lower confidence for fallback extraction, got %f", result.ParserConfidence)
	}
}

func TestExtract_MultiLineCodexResponse(t *testing.T) {
	transcript := "user\nSome question\n\ncodex\nLine one of response.\nLine two of response.\n\ntokens used\n99\n"
	a := New()
	result := a.ExtractResponse(nil, transcript)
	if !strings.Contains(result.Response, "Line one of response.") {
		t.Errorf("expected multi-line response, got %q", result.Response)
	}
	if !strings.Contains(result.Response, "Line two of response.") {
		t.Errorf("expected second line in response, got %q", result.Response)
	}
}

func TestExtract_TokensUsedTerminator(t *testing.T) {
	// "tokens used" should not appear in extracted response.
	transcript := "user\nA question\n\ncodex\nMy response text\n\ntokens used\n42\n"
	a := New()
	result := a.ExtractResponse(nil, transcript)
	if strings.Contains(result.Response, "tokens used") {
		t.Errorf("response should not include 'tokens used' line, got %q", result.Response)
	}
	if strings.Contains(result.Response, "42") {
		t.Errorf("response should not include token count, got %q", result.Response)
	}
}

func TestExtract_RawBytesIgnored(t *testing.T) {
	// ExtractResponse ignores the raw []byte and works on the normalizedTranscript string.
	transcript := "user\nQuestion\n\ncodex\nNormalized response\n\ntokens used\n10\n"
	rawJunk := []byte("completely different raw bytes")
	a := New()
	result := a.ExtractResponse(rawJunk, transcript)
	if result.Response != "Normalized response" {
		t.Errorf("expected response from normalized transcript, got %q", result.Response)
	}
}

func TestExtract_AuthErrorContent(t *testing.T) {
	content := loadFixture(t, "auth_required.txt")
	a := New()
	result := a.ExtractResponse(nil, content)
	// Auth error output has no "codex" section — falls back to last paragraph.
	// Should not return empty.
	if result.Response == "" {
		t.Error("expected non-empty fallback extraction from auth_required.txt")
	}
}

func TestExtract_CrashContent(t *testing.T) {
	content := loadFixture(t, "crash.txt")
	a := New()
	result := a.ExtractResponse(nil, content)
	// Crash output has no "codex" section — falls back to last paragraph.
	if result.Response == "" {
		t.Error("expected non-empty fallback extraction from crash.txt")
	}
}

func TestExtract_WhitespaceOnlyTranscript(t *testing.T) {
	a := New()
	result := a.ExtractResponse(nil, "   \n\n   \n")
	if result.Response != "" {
		t.Errorf("expected empty response for whitespace-only transcript, got %q", result.Response)
	}
	if result.ParserConfidence > 0.2 {
		t.Errorf("expected very low confidence for whitespace-only, got %f", result.ParserConfidence)
	}
}
