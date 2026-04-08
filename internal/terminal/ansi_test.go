package terminal

import (
	"testing"
)

func TestNormalizePTYText_CSISequences(t *testing.T) {
	input := []byte("\x1b[31mred text\x1b[0m")
	got := NormalizePTYText(input)
	want := "red text"
	if got != want {
		t.Errorf("CSI strip: got %q, want %q", got, want)
	}
}

func TestNormalizePTYText_OSCSequences(t *testing.T) {
	input := []byte("\x1b]0;window title\x07normal text")
	got := NormalizePTYText(input)
	want := "normal text"
	if got != want {
		t.Errorf("OSC strip: got %q, want %q", got, want)
	}
}

func TestNormalizePTYText_CarriageReturn(t *testing.T) {
	// CR without LF — overwrite behavior; should not preserve "first" and "second" on separate lines
	input := []byte("first\rsecond")
	got := NormalizePTYText(input)
	// The implementation converts bare CR to '\n', so we should NOT see "first\rsecond" intact
	// We just verify the raw CR is not present in the output
	for _, ch := range got {
		if ch == '\r' {
			t.Errorf("bare CR should not appear in normalized output, got %q", got)
			return
		}
	}
}

func TestNormalizePTYText_CRLFPreserved(t *testing.T) {
	// CRLF should become LF
	input := []byte("line1\r\nline2")
	got := NormalizePTYText(input)
	want := "line1\nline2"
	if got != want {
		t.Errorf("CRLF→LF: got %q, want %q", got, want)
	}
}

func TestNormalizePTYText_ControlChars(t *testing.T) {
	// null, DEL, bell should be stripped; tab should be preserved
	input := []byte("\x00hello\x7fwor\x07ld\there")
	got := NormalizePTYText(input)
	// null (0x00) stripped, DEL (0x7f) stripped, bell (0x07) stripped via OSC fallback
	// tab should be preserved
	for _, ch := range []byte(got) {
		switch ch {
		case 0x00:
			t.Errorf("null byte should be stripped, got %q", got)
		case 0x7f:
			t.Errorf("DEL should be stripped, got %q", got)
		}
	}
	// Tab must survive
	found := false
	for _, ch := range []byte(got) {
		if ch == '\t' {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tab should be preserved, got %q", got)
	}
}

func TestNormalizePTYText_ESCAtBufferEnd(t *testing.T) {
	// Incomplete escape sequence at end — must not panic
	input := []byte("hello\x1b")
	got := NormalizePTYText(input)
	// ESC at the very end is skipped (i+1 >= len check), "hello" should survive
	want := "hello"
	if got != want {
		t.Errorf("trailing ESC: got %q, want %q", got, want)
	}
}

func TestNormalizePTYText_EmptyInput(t *testing.T) {
	got := NormalizePTYText([]byte{})
	if got != "" {
		t.Errorf("empty input: got %q, want empty string", got)
	}
}

func TestNormalizePTYText_PlainText(t *testing.T) {
	input := []byte("hello world\n")
	got := NormalizePTYText(input)
	want := "hello world\n"
	if got != want {
		t.Errorf("plain text: got %q, want %q", got, want)
	}
}
