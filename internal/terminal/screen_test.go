package terminal

import (
	"fmt"
	"strings"
	"testing"
)

// buildSeq assembles the given escape sequences and literal strings into a
// single byte slice for writing to Screen.
func buildSeq(parts ...string) []byte {
	return []byte(strings.Join(parts, ""))
}

func TestScreen_BasicWrite(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("hello"))
	lines := scr.Lines()
	if len(lines) == 0 || lines[0] != "hello" {
		t.Fatalf("expected 'hello', got %v", lines)
	}
}

func TestScreen_CUP(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("AB"))
	// ESC[1;3H — move to row 1, col 3 (1-indexed) and write "X"
	scr.Write([]byte("\x1b[1;3HX"))
	lines := scr.Lines()
	if len(lines) == 0 || lines[0] != "ABX" {
		t.Fatalf("expected 'ABX', got %q", lines[0])
	}
}

func TestScreen_DECSTBM_NewlineAtBottom(t *testing.T) {
	// Create a 5-row terminal. Fill rows 0-4 (0-indexed), then write \n at
	// row 4 (bottom). The scroll region should shift rows 0-3 up, giving row 4
	// blank and keeping content at the expected rows.
	scr := NewScreen(10, 5)
	// Write A..E at rows 1-5 (1-indexed) via CUP.
	for i, ch := range []string{"A", "B", "C", "D", "E"} {
		scr.Write([]byte(fmt.Sprintf("\x1b[%d;1H%s", i+1, ch)))
	}
	// ESC[5;1H → position at row 5 explicitly, then write \n.
	scr.Write([]byte("\x1b[5;1H\n"))
	lines := scr.Lines()
	// After scroll: A (row 0) is gone; B→row 0, C→row 1, D→row 2, E→row 3; row 4 blank (trimmed).
	// Lines() trims trailing empty rows so we get exactly 4 lines.
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}
	for i, want := range []string{"B", "C", "D", "E"} {
		if lines[i] != want {
			t.Errorf("row %d: expected %q, got %q", i, want, lines[i])
		}
	}
}

func TestScreen_DECSTBM_ScrollRegionSet(t *testing.T) {
	// Set scroll region to rows 2-4 (1-indexed) in a 5-row terminal.
	// Content above/below the region must not move.
	scr := NewScreen(10, 5)
	// Write letters at all 5 rows via CUP using multi-char params.
	letters := []string{"A", "B", "C", "D", "E"}
	for i, ch := range letters {
		row := i + 1
		scr.Write([]byte(fmt.Sprintf("\x1b[%d;1H%s", row, ch)))
	}
	// Set scroll region to rows 2-4 (1-indexed).
	// ESC[2;4r — sets scroll region; also moves cursor to home (row 1).
	scr.Write([]byte("\x1b[2;4r"))
	// Move cursor to row 4 (bottom of scroll region) and write \n.
	scr.Write([]byte("\x1b[4;1H\n"))
	lines := scr.Lines()
	// Row 0 (A) must be unchanged (above scroll region).
	if lines[0] != "A" {
		t.Errorf("row 0 (above region): expected 'A', got %q", lines[0])
	}
	// Scroll region was rows 1-3 (0-indexed) = rows 2-4 (1-indexed).
	// After one scroll: row 1 (B, old top) is gone; C→1, D→2, row 3 blank.
	if lines[1] != "C" {
		t.Errorf("row 1: expected 'C' after scroll, got %q", lines[1])
	}
	if lines[2] != "D" {
		t.Errorf("row 2: expected 'D' after scroll, got %q", lines[2])
	}
	// Row 4 (E) must be unchanged (below scroll region).
	if len(lines) < 5 || lines[4] != "E" {
		t.Errorf("row 4 (below region): expected 'E', got %v (lines=%v)", func() string {
			if len(lines) > 4 {
				return lines[4]
			}
			return "<missing>"
		}(), lines)
	}
}

func TestScreen_DECSTBM_CursorHomeOnSet(t *testing.T) {
	scr := NewScreen(10, 5)
	// Move cursor to row 3 col 5.
	scr.Write([]byte("\x1b[3;5H"))
	// Set scroll region: must reset cursor to home.
	scr.Write([]byte("\x1b[1;5r"))
	// Write 'X' — should appear at row 0 col 0.
	scr.Write([]byte("X"))
	lines := scr.Lines()
	if len(lines) == 0 || lines[0] != "X" {
		t.Errorf("after DECSTBM cursor should be at home; expected row 0 = 'X', got %v", lines)
	}
}

func TestScreen_NoScrollWhenUnbounded(t *testing.T) {
	// height=0 (unbounded): \n just increments curRow; use \r\n to also reset col.
	scr := NewScreen(10, 0)
	scr.Write([]byte("A\r\nB\r\nC\r\nD\r\nE"))
	lines := scr.Lines()
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	for i, want := range []string{"A", "B", "C", "D", "E"} {
		if lines[i] != want {
			t.Errorf("row %d: expected %q, got %q", i, want, lines[i])
		}
	}
}

func TestScreen_ScrollUpCSI(t *testing.T) {
	// ESC[S scrolls up the scroll region by N lines.
	scr := NewScreen(10, 4)
	for i, ch := range []string{"A", "B", "C", "D"} {
		scr.Write([]byte(fmt.Sprintf("\x1b[%d;1H%s", i+1, ch)))
	}
	// Scroll up 2 lines.
	scr.Write([]byte("\x1b[2S"))
	lines := scr.Lines()
	// A and B scroll off; C→row0, D→row1, rows 2-3 blank.
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "C" {
		t.Errorf("after SU 2: row 0 expected 'C', got %q", lines[0])
	}
	if lines[1] != "D" {
		t.Errorf("after SU 2: row 1 expected 'D', got %q", lines[1])
	}
}

func TestScreen_TUILayout_ResponseVisible(t *testing.T) {
	// Simulate a minimal Claude Code TUI sequence:
	//  - 24-row terminal, scroll region rows 1-20 (1-indexed)
	//  - Welcome box fills rows 1-10
	//  - Prompt at row 13
	//  - User sends prompt, user separator rendered at row 14
	//  - Response at row 15
	//  - Status bar at rows 21-22 (outside scroll region)
	//
	// This tests that the response is findable AFTER the user separator even
	// when the welcome box has accumulated extra content.
	const cols = 80
	scr := NewScreen(cols, 24)

	// Scroll region rows 1-20.
	scr.Write([]byte("\x1b[1;20r"))

	// Write a "welcome box" at rows 1-10 via CUP (1-indexed).
	for r := 1; r <= 10; r++ {
		scr.Write([]byte{0x1b, '['})
		scr.Write([]byte{byte('0' + r), ';', '1', 'H'})
		scr.Write([]byte("│ welcome │"))
	}

	// Prompt at row 13.
	scr.Write([]byte("\x1b[13;1H❯ "))

	// User separator at row 14 (box-drawing chars + embedded text).
	sep := "──what is 2+2?──────────────────────────────────────────────────────────────────"
	scr.Write([]byte("\x1b[14;1H" + sep))

	// Response at row 15.
	scr.Write([]byte("\x1b[15;1H4"))

	// Status bar at row 21.
	scr.Write([]byte("\x1b[21;1HCtx(u): 0.0% Session: 85%"))

	lines := scr.Lines()

	// Find the user separator and confirm response follows.
	sepIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "what is 2+2?") {
			sepIdx = i
			break
		}
	}
	if sepIdx == -1 {
		t.Fatal("user separator not found in screen lines")
	}
	if sepIdx+1 >= len(lines) {
		t.Fatalf("no line after user separator (sepIdx=%d, len=%d)", sepIdx, len(lines))
	}
	if strings.TrimSpace(lines[sepIdx+1]) != "4" {
		t.Errorf("expected '4' immediately after user separator, got %q (lines around sep: %v)",
			lines[sepIdx+1], lines[max(0, sepIdx-1):min(len(lines), sepIdx+4)])
	}
}

func TestScreen_Content_JoinsLines(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("A\r\nB\r\nC"))
	got := scr.Content()
	if got != "A\nB\nC" {
		t.Errorf("Content(): expected 'A\\nB\\nC', got %q", got)
	}
}

func TestScreen_Content_Empty(t *testing.T) {
	scr := NewScreen(10, 0)
	if scr.Content() != "" {
		t.Errorf("empty screen Content should be empty, got %q", scr.Content())
	}
}

func TestScreen_ScrollDownCSI(t *testing.T) {
	scr := NewScreen(10, 4)
	for i, ch := range []string{"A", "B", "C", "D"} {
		scr.Write([]byte(fmt.Sprintf("\x1b[%d;1H%s", i+1, ch)))
	}
	// SD 2: bottom 2 rows fall off; A,B shift to rows 2,3; rows 0,1 blank.
	scr.Write([]byte("\x1b[2T"))
	lines := scr.Lines()
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "" || lines[1] != "" {
		t.Errorf("top rows should be blank after SD, got %q,%q", lines[0], lines[1])
	}
	if lines[2] != "A" || lines[3] != "B" {
		t.Errorf("expected A,B shifted down; got %q,%q", lines[2], lines[3])
	}
}

func TestScreen_EraseDisplay_FromCursor(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))
	// Move to row 2 col 3 (1-indexed), erase to end.
	scr.Write([]byte("\x1b[2;3H\x1b[0J"))
	lines := scr.Lines()
	if lines[0] != "AAAAA" {
		t.Errorf("row 0 should be unchanged, got %q", lines[0])
	}
	if lines[1] != "BB" {
		t.Errorf("row 1 should be truncated to BB, got %q", lines[1])
	}
	if len(lines) > 2 && lines[2] != "" {
		t.Errorf("row 2 should be cleared, got %q", lines[2])
	}
}

func TestScreen_EraseDisplay_ToCursor(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("AAAAA\r\nBBBBB\r\nCCCCC"))
	scr.Write([]byte("\x1b[2;3H\x1b[1J"))
	lines := scr.Lines()
	if lines[0] != "" {
		t.Errorf("row 0 should be erased, got %q", lines[0])
	}
	// Row 1 columns 0..2 erased, tail preserved.
	if lines[1] != "   BB" {
		t.Errorf("row 1 expected '   BB', got %q", lines[1])
	}
}

func TestScreen_EraseDisplay_All(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("AAA\r\nBBB"))
	scr.Write([]byte("\x1b[2J"))
	if scr.Content() != "" {
		t.Errorf("ED 2 should clear all; got %q", scr.Content())
	}
}

func TestScreen_EraseInLine(t *testing.T) {
	cases := []struct {
		name   string
		seq    string
		expect string
	}{
		{"EL 0 to end", "\x1b[1;3H\x1b[0K", "AB"},
		{"EL 1 to cursor", "\x1b[1;3H\x1b[1K", "   DE"},
		{"EL 2 entire", "\x1b[1;3H\x1b[2K", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scr := NewScreen(10, 0)
			scr.Write([]byte("ABCDE"))
			scr.Write([]byte(tc.seq))
			lines := scr.Lines()
			got := ""
			if len(lines) > 0 {
				got = lines[0]
			}
			if got != tc.expect {
				t.Errorf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}

func TestScreen_DeleteChars(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("ABCDE"))
	// Move to col 2 (1-indexed), DCH 2 → remove BC.
	scr.Write([]byte("\x1b[1;2H\x1b[2P"))
	lines := scr.Lines()
	if lines[0] != "ADE" {
		t.Errorf("expected 'ADE', got %q", lines[0])
	}
}

func TestScreen_CursorMovements(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("AB\r\nCD\r\nEF"))
	// CUU 1 from row 2 → row 1, then CUF 1 → col 1, then write X overwrites D.
	scr.Write([]byte("\x1b[3;1H\x1b[A\x1b[CX"))
	lines := scr.Lines()
	if lines[1] != "CX" {
		t.Errorf("CUU+CUF+write: expected row1 'CX', got %q", lines[1])
	}
	// CUB from col 1 to col 0, write Y.
	scr.Write([]byte("\x1b[2;2H\x1b[DY"))
	lines = scr.Lines()
	if lines[1] != "YX" {
		t.Errorf("CUB: expected 'YX', got %q", lines[1])
	}
}

func TestScreen_CursorNextPrevLine(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("AAA"))
	// CNL 1 → col 0, next row.
	scr.Write([]byte("\x1b[1EBBB"))
	// CPL 1 → col 0, prev row — overwrite row 0.
	scr.Write([]byte("\x1b[1FCC"))
	lines := scr.Lines()
	if lines[0] != "CCA" {
		t.Errorf("CPL overwrite: expected 'CCA', got %q", lines[0])
	}
	if lines[1] != "BBB" {
		t.Errorf("row 1 expected 'BBB', got %q", lines[1])
	}
}

func TestScreen_CHA_VPA(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("ABCDE"))
	// CHA col 2 (1-indexed) then write X → overwrite B.
	scr.Write([]byte("\x1b[2GX"))
	// VPA row 3 then write Y.
	scr.Write([]byte("\x1b[3dY"))
	lines := scr.Lines()
	if lines[0] != "AXCDE" {
		t.Errorf("CHA: expected 'AXCDE', got %q", lines[0])
	}
	if len(lines) < 3 || !strings.Contains(lines[2], "Y") {
		t.Errorf("VPA row 3: expected 'Y', got %v", lines)
	}
}

func TestScreen_SaveRestoreCursor(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("AB"))
	scr.Write([]byte("\x1b[s"))      // save at (0,2)
	scr.Write([]byte("\r\nCD"))      // move away
	scr.Write([]byte("\x1b[uZ"))     // restore, write Z
	lines := scr.Lines()
	if lines[0] != "ABZ" {
		t.Errorf("restore cursor: expected 'ABZ', got %q", lines[0])
	}
}

func TestScreen_Backspace(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("ABC\bX"))
	lines := scr.Lines()
	if lines[0] != "ABX" {
		t.Errorf("BS: expected 'ABX', got %q", lines[0])
	}
}

func TestScreen_IgnoreControlAndBell(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte{'A', 0x07, 0x00, 0x01, 0x0e, 0x0f, 0x05, 'B'})
	lines := scr.Lines()
	if lines[0] != "AB" {
		t.Errorf("control chars should be ignored; got %q", lines[0])
	}
}

func TestScreen_OSCSequence_Skipped(t *testing.T) {
	scr := NewScreen(20, 0)
	// OSC terminated by BEL.
	scr.Write([]byte("\x1b]0;title\x07X"))
	// OSC terminated by ST (ESC \).
	scr.Write([]byte("\x1b]0;title\x1b\\Y"))
	lines := scr.Lines()
	if lines[0] != "XY" {
		t.Errorf("OSC not skipped: got %q", lines[0])
	}
}

func TestScreen_CharsetDesignation_Skipped(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("\x1b(BA\x1b)0B"))
	lines := scr.Lines()
	if lines[0] != "AB" {
		t.Errorf("charset designations should be skipped; got %q", lines[0])
	}
}

func TestScreen_TwoByteEscSkipped(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("\x1b=A\x1b>B"))
	lines := scr.Lines()
	if lines[0] != "AB" {
		t.Errorf("ESC =/> should skip; got %q", lines[0])
	}
}

func TestScreen_TrailingEscIgnored(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("A\x1b"))
	// Second write continues — dangling ESC shouldn't crash.
	scr.Write([]byte("B"))
	// Dangling ESC + incomplete CSI handled without panic.
	scr2 := NewScreen(10, 0)
	scr2.Write([]byte("\x1b["))
}

func TestScreen_InvalidUTF8Skipped(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte{'A', 0xff, 'B'})
	lines := scr.Lines()
	if lines[0] != "AB" {
		t.Errorf("invalid UTF-8 should be skipped; got %q", lines[0])
	}
}

func TestScreen_NewScreen_DefaultCols(t *testing.T) {
	scr := NewScreen(0, 0)
	if scr.cols != 80 {
		t.Errorf("expected default cols=80, got %d", scr.cols)
	}
}

func TestScreen_AutowrapOnOverflow(t *testing.T) {
	scr := NewScreen(3, 0)
	scr.Write([]byte("ABCDEF"))
	lines := scr.Lines()
	if len(lines) < 2 || lines[0] != "ABC" || lines[1] != "DEF" {
		t.Errorf("autowrap at cols=3: expected ABC/DEF, got %v", lines)
	}
}

func TestScreen_ParseHelpers_DefaultsAndZero(t *testing.T) {
	// Ensure parseOne/parseTwo defaults trigger via CSI with empty params.
	scr := NewScreen(10, 5)
	// CUU with empty param → default 1.
	scr.Write([]byte("\x1b[3;1HX\x1b[AY"))
	lines := scr.Lines()
	// Y should be at row 1 col 1 (after CUU moved from row 2 to row 1, X advanced col to 1).
	if len(lines) < 2 || !strings.Contains(lines[1], "Y") {
		t.Errorf("parseOne default: got %v", lines)
	}
	// parseOne with 0 should fall back to default.
	scr2 := NewScreen(10, 0)
	scr2.Write([]byte("A\x1b[0CB"))
	// CUF 0 → treated as 1, B at col 2.
	if scr2.Lines()[0] != "A B" {
		t.Errorf("CUF 0 default: expected 'A B', got %q", scr2.Lines()[0])
	}
}

func TestScreen_DECSTBM_InvalidRange(t *testing.T) {
	scr := NewScreen(10, 5)
	// top>=bot should fall back to full screen.
	scr.Write([]byte("\x1b[4;2r"))
	if scr.scrollTop != 0 || scr.scrollBot != 4 {
		t.Errorf("invalid DECSTBM should reset; got top=%d bot=%d", scr.scrollTop, scr.scrollBot)
	}
}

func TestScreen_DECSTBM_IgnoredWhenUnbounded(t *testing.T) {
	scr := NewScreen(10, 0)
	scr.Write([]byte("\x1b[2;4r"))
	// No crash, scrollTop/Bot remain default 0/0.
	if scr.scrollTop != 0 || scr.scrollBot != 0 {
		t.Errorf("DECSTBM should be ignored when height=0")
	}
}

func TestScreen_CUB_CUF_Clamp(t *testing.T) {
	scr := NewScreen(5, 0)
	// CUB beyond start clamps to 0.
	scr.Write([]byte("\x1b[99DA"))
	// CUF beyond end clamps to cols-1.
	scr.Write([]byte("\x1b[99CB"))
	lines := scr.Lines()
	if lines[0] != "A   B" {
		t.Errorf("CUB/CUF clamp: expected 'A   B', got %q", lines[0])
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
