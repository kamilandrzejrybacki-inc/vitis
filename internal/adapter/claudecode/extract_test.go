package claudecode

import "testing"

func TestExtractResponse(t *testing.T) {
	a := New()
	result := a.ExtractResponse(nil, "system noise\n\nFinal answer line 1\nFinal answer line 2\n")
	if result.Response != "Final answer line 1\nFinal answer line 2" {
		t.Fatalf("unexpected response: %q", result.Response)
	}
}
