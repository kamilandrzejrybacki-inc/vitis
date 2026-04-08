package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestExtractMarker(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"normal", "please output the token TOK_123 on its own line.\n", "TOK_123"},
		{"crlf", "output the token CR_TOK\r\n", "CR_TOK"},
		{"no needle", "this line has no marker request\n", ""},
		{"trailing space", "output the token SPACE_TOK and more\n", "SPACE_TOK"},
		{"empty", "", ""},
		{"needle without terminator", "output the token NOEND", ""},
		{"embedded mid-line", "blah blah output the token MID end\n", "MID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractMarker(tc.in); got != tc.want {
				t.Errorf("extractMarker(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestReadEnvelopeMarker_Found(t *testing.T) {
	input := "ignore this line\nanother line\noutput the token GOTIT here\nshould not reach\n"
	r := bufio.NewReader(strings.NewReader(input))
	tok, ok := readEnvelopeMarker(r)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if tok != "GOTIT" {
		t.Errorf("expected GOTIT, got %q", tok)
	}
}

func TestReadEnvelopeMarker_EOF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("nothing matches here\n"))
	tok, ok := readEnvelopeMarker(r)
	if ok || tok != "" {
		t.Errorf("expected (\"\", false), got (%q, %v)", tok, ok)
	}
}

func TestReadEnvelopeMarker_NoFinalNewline(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("output the token LAST end"))
	tok, ok := readEnvelopeMarker(r)
	if !ok || tok != "LAST" {
		t.Errorf("expected ('LAST', true), got (%q, %v)", tok, ok)
	}
}

func TestEnv_FallbackAndOverride(t *testing.T) {
	t.Setenv("MOCK_TEST_KEY", "set-value")
	if got := env("MOCK_TEST_KEY", "fallback"); got != "set-value" {
		t.Errorf("expected set-value, got %q", got)
	}
	if got := env("MOCK_TEST_MISSING_XYZ", "fallback"); got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}
}
