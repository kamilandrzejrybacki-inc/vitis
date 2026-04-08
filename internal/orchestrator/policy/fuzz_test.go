package policy

import "testing"

// FuzzParseNextTrailer exercises the last-line regex parser with random
// inputs to catch edge cases in the regex, line-splitting, and whitespace
// handling that the table-driven tests might miss.
//
// Property: parseNextTrailer must not panic on any input, and any non-nil
// return value must be a PeerID string that satisfies the same validation
// regex used by the model layer (lowercase ASCII start, bounded length).
func FuzzParseNextTrailer(f *testing.F) {
	// Seed corpus covers known-good and known-bad shapes.
	f.Add("text\n<<NEXT: bob>>")
	f.Add("text")
	f.Add("")
	f.Add("<<END>>")
	f.Add("<<NEXT: alice>>\n<<END>>")
	f.Add("```\n<<NEXT: bob>>\n```\nreal last line")
	f.Add("<<NEXT:    peer-1    >>")
	f.Add("<<NEXT: 1bob>>") // invalid id, must be rejected
	f.Add("<<NEXT: >>")     // empty id, must be rejected
	f.Add("<<NEXT: Uppercase>>")

	f.Fuzz(func(t *testing.T, in string) {
		got := parseNextTrailer(in)
		if got == nil {
			return
		}
		// Any returned id must match the peer id regex (lowercase start,
		// bounded length). The regex in parseNextTrailer already enforces
		// this; this fuzz property guards against regex drift.
		s := string(*got)
		if len(s) == 0 || len(s) > 32 {
			t.Fatalf("parsed id has invalid length: %q", s)
		}
		if s[0] < 'a' || s[0] > 'z' {
			t.Fatalf("parsed id must start with lowercase letter: %q", s)
		}
		for i := 0; i < len(s); i++ {
			c := s[i]
			ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
			if !ok {
				t.Fatalf("parsed id contains invalid char at %d: %q", i, s)
			}
		}
	})
}
