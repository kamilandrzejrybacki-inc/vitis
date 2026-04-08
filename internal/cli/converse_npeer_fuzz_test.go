package cli

import (
	"testing"
)

// FuzzParseKeyValueList exercises the hand-written --peer flag parser
// with random inputs. The property we assert is bounded behavior:
//
//   - parseKeyValueList must never panic
//   - on success, every returned key/value pair must be non-empty in
//     the key field (the parser explicitly rejects empty keys)
//   - on error, the returned slice may be partial but must still be
//     well-formed (no panic during accessing it)
//
// This catches edge cases in the quote/escape handling, lonely '='
// boundaries, and trailing-comma corner cases that the table-driven
// tests might miss under random byte sequences.
func FuzzParseKeyValueList(f *testing.F) {
	seeds := []string{
		"id=alice,provider=claude-code",
		`id=alice,provider=claude-code,seed="hello, world."`,
		`id=alice,provider=claude-code,seed="say \"hi\""`,
		"id=,provider=x",
		"key=",
		"=value",
		"k1=v1,k2=",
		`k="unterminated`,
		`k="\\\\"`,
		"",
		",,,",
		"a=b,c=d,e=f",
		"key with space=value",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		pairs, err := parseKeyValueList(in)
		if err != nil {
			// Error path: pairs may be nil or partial; just make sure
			// we can read it without panicking.
			for _, p := range pairs {
				_ = p.key
				_ = p.value
			}
			return
		}
		// Success path: every pair must have a non-empty key.
		for i, p := range pairs {
			if p.key == "" {
				t.Fatalf("pair %d has empty key in input %q (parsed: %+v)", i, in, pairs)
			}
		}
	})
}
