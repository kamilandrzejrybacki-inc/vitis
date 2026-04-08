// Package conversation contains the broker, envelope builder, and result
// types for vitis's A2A multi-turn conversations. It depends on internal/bus
// and internal/model. It does NOT depend on internal/peer to avoid an
// import cycle (peer transports import this package).
package conversation

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// MarkerPrefix is the literal prefix for every per-turn marker token.
const MarkerPrefix = "TURN_END_"

// markerSuffixBytes is the number of random bytes encoded into the suffix.
// 6 bytes -> 12 hex chars -> ~2.8e14 possible values per turn.
const markerSuffixBytes = 6

// NewMarkerToken returns a new randomized marker token of the form
// "TURN_END_<12 hex chars>". Crypto-random; unique per turn with vanishing
// collision probability.
func NewMarkerToken() string {
	buf := make([]byte, markerSuffixBytes)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is essentially unrecoverable; if it ever
		// happens we'd rather crash than emit a non-random token that
		// could collide with conversation content.
		panic("crypto/rand failed: " + err.Error())
	}
	return MarkerPrefix + hex.EncodeToString(buf)
}

// ContainsMarker reports whether body contains the literal marker token.
// Returns false if either argument is empty.
func ContainsMarker(body, token string) bool {
	if body == "" || token == "" {
		return false
	}
	return strings.Contains(body, token)
}

// StripMarkerAndAfter returns body truncated at the first occurrence of
// token. Trailing content (and the marker itself) is dropped. The boolean
// reports whether the marker was found.
func StripMarkerAndAfter(body, token string) (string, bool) {
	if token == "" {
		return body, false
	}
	idx := strings.Index(body, token)
	if idx < 0 {
		return body, false
	}
	return body[:idx], true
}
