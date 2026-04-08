package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func NewSessionID() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return "sess_" + hex.EncodeToString(raw[:]), nil
}

// NewID generates a random hex-suffixed ID with the given prefix.
// It panics on entropy failure, which is appropriate for startup-time
// identifiers where a missing CSPRNG is a fatal configuration error.
func NewID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(fmt.Sprintf("generate id: %v", err))
	}
	return prefix + hex.EncodeToString(raw[:])
}
