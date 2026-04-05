package util

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestBytesJSON_MarshalEmpty(t *testing.T) {
	b := Bytes([]byte{})
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	// base64 of empty slice is "" so JSON should be `""`
	want := `""`
	if string(data) != want {
		t.Errorf("want %s, got %s", want, data)
	}
}

func TestBytesJSON_MarshalData(t *testing.T) {
	input := []byte("hello")
	b := Bytes(input)
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	wantEncoded := base64.StdEncoding.EncodeToString(input)
	// json.Marshal of a string wraps in quotes
	want := `"` + wantEncoded + `"`
	if string(data) != want {
		t.Errorf("want %s, got %s", want, data)
	}
}

func TestBytesJSON_RoundTrip(t *testing.T) {
	original := Bytes("round trip test data")

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// The JSON is a base64-encoded string; unmarshal it as a string and decode.
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		t.Fatalf("unmarshal to string error: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}

	if string(decoded) != string(original) {
		t.Errorf("round-trip mismatch: want %q, got %q", string(original), string(decoded))
	}
}
