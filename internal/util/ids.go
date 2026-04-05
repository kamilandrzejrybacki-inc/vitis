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
