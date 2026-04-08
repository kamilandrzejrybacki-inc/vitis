package terminal

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Screen is a minimal VT100/ANSI screen buffer. It replays raw PTY bytes and
// maintains a 2D grid of visible characters. Unlike NormalizePTYText (which
// strips control codes and preserves a linear text stream), Screen correctly
// handles cursor-positioned writes such as those used by full-screen TUIs.
//
// When height > 0, the screen implements DECSTBM scroll regions: a newline at
// the bottom of the scroll region scrolls that region up instead of growing
// the buffer. This matches real terminal behaviour for full-screen TUIs like
// Claude Code that use scroll regions for their conversation pane.
//
// The buffer still grows vertically without limit for absolute CUP writes
// outside the current scroll region.
type Screen struct {
	cols      int
	height    int      // terminal height; 0 = unbounded (no scroll region logic)
	rows      [][]rune // grows as needed
	curRow    int      // 0-indexed absolute row
	curCol    int      // 0-indexed column
	saved     [2]int   // saved cursor (row, col) for \x1b[s / \x1b[u
	scrollTop int      // 0-indexed top of scroll region (valid when height > 0)
	scrollBot int      // 0-indexed bottom of scroll region (valid when height > 0)
}

// NewScreen creates a Screen with the given column width and optional terminal
// height. Pass height=0 for a legacy unbounded buffer with no scroll region
// handling. Pass height=24 (or the actual PTY rows) to enable DECSTBM support.
func NewScreen(cols, height int) *Screen {
	if cols <= 0 {
		cols = 80
	}
	s := &Screen{cols: cols, height: height}
	if height > 0 {
		s.scrollTop = 0
		s.scrollBot = height - 1
	}
	return s
}

// Write processes raw PTY bytes and updates the screen state.
func (s *Screen) Write(data []byte) {
	i := 0
	for i < len(data) {
		b := data[i]
		switch {
		case b == 0x1b:
			if i+1 >= len(data) {
				i++
				continue
			}
			switch data[i+1] {
			case '[':
				// CSI sequence: ESC [ <params> <final>
				i += 2
				start := i
				for i < len(data) && data[i] < 0x40 {
					i++
				}
				if i >= len(data) {
					continue
				}
				params := string(data[start:i])
				final := data[i]
				i++
				s.handleCSI(params, final)
			case ']':
				// OSC: ESC ] ... ST  — skip
				i += 2
				for i < len(data) {
					if data[i] == 0x07 {
						i++
						break
					}
					if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
			case '(', ')':
				// Character set designations — skip 3 bytes
				i += 3
			default:
				// Two-byte ESC sequence (e.g. ESC = / ESC >) — skip
				i += 2
			}
		case b == '\r':
			s.curCol = 0
			i++
		case b == '\n':
			if s.height > 0 && s.curRow == s.scrollBot {
				s.doScrollUp(1)
			} else {
				s.curRow++
			}
			i++
		case b == 0x08: // BS
			if s.curCol > 0 {
				s.curCol--
			}
			i++
		case b == 0x07 || b == 0x00 || b == 0x0e || b == 0x0f || b == 0x01 || b == 0x02:
			// BEL, NUL, SO, SI, and other control chars — ignore
			i++
		case b < 0x20:
			i++
		default:
			r, size := utf8.DecodeRune(data[i:])
			if r == utf8.RuneError && size == 1 {
				i++
				continue
			}
			s.writeRune(r)
			i += size
		}
	}
}

func (s *Screen) ensureRow(row int) {
	for len(s.rows) <= row {
		line := make([]rune, s.cols)
		for j := range line {
			line[j] = ' '
		}
		s.rows = append(s.rows, line)
	}
}

func (s *Screen) writeRune(r rune) {
	if s.curRow < 0 {
		s.curRow = 0
	}
	s.ensureRow(s.curRow)
	if s.curCol >= 0 && s.curCol < s.cols {
		s.rows[s.curRow][s.curCol] = r
	}
	s.curCol++
	if s.curCol >= s.cols {
		s.curCol = 0
		s.curRow++
	}
}

// doScrollUp shifts the scroll region up by n lines: the top n lines are lost,
// n new blank lines appear at the bottom. The cursor row is unchanged.
func (s *Screen) doScrollUp(n int) {
	if n <= 0 || s.height == 0 {
		return
	}
	s.ensureRow(s.scrollBot)
	regionSize := s.scrollBot - s.scrollTop + 1
	if n >= regionSize {
		n = regionSize
	}
	// Shift rows up within the scroll region by moving slice references.
	// We must copy the shifted rows' content into new backing arrays for the
	// vacated bottom rows — otherwise the source and destination slices share
	// the same underlying array and blanking the new row also clears the row
	// that moved into it.
	for i := 0; i < regionSize-n; i++ {
		s.rows[s.scrollTop+i] = s.rows[s.scrollTop+i+n]
	}
	// Replace the newly exposed bottom rows with fresh blank lines.
	for i := regionSize - n; i < regionSize; i++ {
		s.rows[s.scrollTop+i] = s.newBlankRow()
	}
}

// doScrollDown shifts the scroll region down by n lines: the bottom n lines
// are lost, n new blank lines appear at the top.
func (s *Screen) doScrollDown(n int) {
	if n <= 0 || s.height == 0 {
		return
	}
	s.ensureRow(s.scrollBot)
	regionSize := s.scrollBot - s.scrollTop + 1
	if n >= regionSize {
		n = regionSize
	}
	// Shift rows down within the scroll region.
	for i := regionSize - 1; i >= n; i-- {
		s.rows[s.scrollTop+i] = s.rows[s.scrollTop+i-n]
	}
	// Replace the newly exposed top rows with fresh blank lines.
	for i := 0; i < n; i++ {
		s.rows[s.scrollTop+i] = s.newBlankRow()
	}
}

func (s *Screen) newBlankRow() []rune {
	line := make([]rune, s.cols)
	for j := range line {
		line[j] = ' '
	}
	return line
}

func (s *Screen) handleCSI(params string, final byte) {
	switch final {
	case 'H', 'f': // CUP / HVP — cursor position (1-indexed)
		row, col := parseTwo(params, 1, 1)
		s.curRow = clamp(row-1, 0, 65535)
		s.curCol = clamp(col-1, 0, s.cols-1)

	case 'A': // CUU — cursor up
		n := parseOne(params, 1)
		s.curRow = max(0, s.curRow-n)
	case 'B': // CUD — cursor down
		n := parseOne(params, 1)
		s.curRow += n
	case 'C': // CUF — cursor right
		n := parseOne(params, 1)
		s.curCol = min(s.cols-1, s.curCol+n)
	case 'D': // CUB — cursor left
		n := parseOne(params, 1)
		s.curCol = max(0, s.curCol-n)
	case 'E': // CNL — cursor next line
		n := parseOne(params, 1)
		s.curCol = 0
		s.curRow += n
	case 'F': // CPL — cursor previous line
		n := parseOne(params, 1)
		s.curCol = 0
		s.curRow = max(0, s.curRow-n)
	case 'G': // CHA — cursor horizontal absolute (1-indexed)
		n := parseOne(params, 1)
		s.curCol = clamp(n-1, 0, s.cols-1)
	case 'd': // VPA — vertical position absolute (1-indexed)
		n := parseOne(params, 1)
		s.curRow = clamp(n-1, 0, 65535)

	case 'J': // ED — erase in display
		n := parseOne(params, 0)
		switch n {
		case 0: // from cursor to end
			if s.curRow < len(s.rows) {
				for j := s.curCol; j < s.cols; j++ {
					s.rows[s.curRow][j] = ' '
				}
				for row := s.curRow + 1; row < len(s.rows); row++ {
					for j := range s.rows[row] {
						s.rows[row][j] = ' '
					}
				}
			}
		case 1: // from start to cursor
			for row := 0; row < s.curRow && row < len(s.rows); row++ {
				for j := range s.rows[row] {
					s.rows[row][j] = ' '
				}
			}
			if s.curRow < len(s.rows) {
				for j := 0; j <= s.curCol && j < s.cols; j++ {
					s.rows[s.curRow][j] = ' '
				}
			}
		case 2, 3: // entire screen
			for row := range s.rows {
				for j := range s.rows[row] {
					s.rows[row][j] = ' '
				}
			}
		}

	case 'K': // EL — erase in line
		n := parseOne(params, 0)
		if s.curRow < len(s.rows) {
			switch n {
			case 0: // from cursor to end of line
				for j := s.curCol; j < s.cols; j++ {
					s.rows[s.curRow][j] = ' '
				}
			case 1: // from start to cursor
				for j := 0; j <= s.curCol && j < s.cols; j++ {
					s.rows[s.curRow][j] = ' '
				}
			case 2: // entire line
				for j := range s.rows[s.curRow] {
					s.rows[s.curRow][j] = ' '
				}
			}
		}

	case 'P': // DCH — delete characters
		n := parseOne(params, 1)
		if s.curRow < len(s.rows) {
			row := s.rows[s.curRow]
			end := min(s.curCol+n, s.cols)
			copy(row[s.curCol:], row[end:])
			for j := s.cols - n; j < s.cols; j++ {
				if j >= 0 {
					row[j] = ' '
				}
			}
		}

	case 's': // DECSC — save cursor (also used as SCP in some terminals)
		s.saved[0] = s.curRow
		s.saved[1] = s.curCol
	case 'u': // DECRC — restore cursor
		s.curRow = s.saved[0]
		s.curCol = s.saved[1]

	case 'r': // DECSTBM — set top and bottom margins (scroll region)
		if s.height > 0 {
			top, bot := parseTwo(params, 1, s.height)
			s.scrollTop = clamp(top-1, 0, s.height-1)
			s.scrollBot = clamp(bot-1, 0, s.height-1)
			if s.scrollTop >= s.scrollBot {
				s.scrollTop = 0
				s.scrollBot = s.height - 1
			}
			// DECSTBM moves the cursor to the home position.
			s.curRow = 0
			s.curCol = 0
		}

	case 'S': // SU — scroll up N lines
		n := parseOne(params, 1)
		s.doScrollUp(n)

	case 'T': // SD — scroll down N lines
		n := parseOne(params, 1)
		s.doScrollDown(n)

		// Ignored: m (SGR), h/l (mode set/reset), n (DSR), etc.
	}
}

// Lines returns the non-empty screen content as a slice of strings, with
// trailing spaces trimmed from each line.
func (s *Screen) Lines() []string {
	lines := make([]string, 0, len(s.rows))
	for _, row := range s.rows {
		line := strings.TrimRight(string(row), " ")
		lines = append(lines, line)
	}
	// Trim trailing empty lines.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// Content returns the full screen state as a single newline-joined string.
func (s *Screen) Content() string {
	return strings.Join(s.Lines(), "\n")
}

func parseOne(params string, def int) int {
	p := strings.TrimLeft(params, "?")
	if idx := strings.Index(p, ";"); idx >= 0 {
		p = p[:idx]
	}
	if p == "" {
		return def
	}
	v, err := strconv.Atoi(p)
	if err != nil || v == 0 {
		return def
	}
	return v
}

func parseTwo(params string, defA, defB int) (int, int) {
	parts := strings.SplitN(params, ";", 2)
	a, b := defA, defB
	if len(parts) >= 1 && parts[0] != "" {
		if v, err := strconv.Atoi(parts[0]); err == nil && v > 0 {
			a = v
		}
	}
	if len(parts) >= 2 && parts[1] != "" {
		if v, err := strconv.Atoi(parts[1]); err == nil && v > 0 {
			b = v
		}
	}
	return a, b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
