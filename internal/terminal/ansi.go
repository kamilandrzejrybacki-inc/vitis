package terminal

import "strings"

// NormalizePTYText strips common terminal control sequences while keeping the
// visible text flow useful for transcript parsing.
func NormalizePTYText(raw []byte) string {
	var out strings.Builder
	data := raw
	for i := 0; i < len(data); i++ {
		b := data[i]
		switch b {
		case 0x1b:
			if i+1 >= len(data) {
				continue
			}
			switch data[i+1] {
			case '[':
				i += 2
				for ; i < len(data); i++ {
					if (data[i] >= 'A' && data[i] <= 'Z') || (data[i] >= 'a' && data[i] <= 'z') {
						break
					}
				}
			case ']':
				i += 2
				for ; i < len(data); i++ {
					if data[i] == 0x07 {
						break
					}
					if data[i] == 0x1b && i+1 < len(data) && data[i+1] == '\\' {
						i++
						break
					}
				}
			default:
				i++
			}
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				continue
			}
			out.WriteByte('\n')
		case 0x00:
			continue
		default:
			if b == '\t' || b == '\n' || (b >= 0x20 && b != 0x7f) {
				out.WriteByte(b)
			}
		}
	}
	return strings.ReplaceAll(out.String(), "\r\n", "\n")
}
