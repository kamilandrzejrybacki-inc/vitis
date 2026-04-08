package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// peerDecl is the parsed form of a single --peer flag value.
//
// Each --peer flag carries a comma-separated key=value list. Recognised keys:
//
//	id          required, peer id (must satisfy model.PeerID validation)
//	provider    required, provider URI without the "provider:" prefix
//	             (e.g. "claude-code", "codex", "mock")
//	seed        optional, per-peer seed text. If omitted, the global
//	             --seed flag is used.
//	model           optional, passes through to PeerSpec.Options
//	reasoning-effort optional, passes through to PeerSpec.Options
//	cwd             optional, passes through to PeerSpec.Options
//	home            optional, passes through to PeerSpec.Options
//
// Values can be quoted with double quotes to embed commas:
//
//	--peer id=alice,provider=claude-code,seed="hello, world."
//
// Backslash escapes inside double-quoted values: \" -> ", \\ -> \.
type peerDecl struct {
	ID       model.PeerID
	Provider string
	Seed     string
	Options  map[string]string
}

var allowedPeerKeys = map[string]bool{
	"id":               true,
	"provider":         true,
	"seed":             true,
	"model":            true,
	"reasoning-effort": true,
	"cwd":              true,
	"home":             true,
}

// parsePeerSpec parses a single --peer flag value into a peerDecl.
// It returns a clear, user-facing error on any malformed input.
func parsePeerSpec(raw string) (peerDecl, error) {
	pairs, err := parseKeyValueList(raw)
	if err != nil {
		return peerDecl{}, err
	}
	pd := peerDecl{Options: map[string]string{}}
	for _, kv := range pairs {
		if !allowedPeerKeys[kv.key] {
			return peerDecl{}, fmt.Errorf("--peer: unknown key %q (allowed: id, provider, seed, model, reasoning-effort, cwd, home)", kv.key)
		}
		switch kv.key {
		case "id":
			pd.ID = model.PeerID(kv.value)
		case "provider":
			pd.Provider = kv.value
		case "seed":
			pd.Seed = kv.value
		default:
			pd.Options[kv.key] = kv.value
		}
	}
	if pd.ID == "" {
		return peerDecl{}, fmt.Errorf("--peer: missing required key id (got %q)", raw)
	}
	if err := pd.ID.Validate(); err != nil {
		return peerDecl{}, fmt.Errorf("--peer: invalid id %q: %w", pd.ID, err)
	}
	if pd.Provider == "" {
		return peerDecl{}, fmt.Errorf("--peer id=%s: missing required key provider", pd.ID)
	}
	return pd, nil
}

// kvPair is a single parsed key=value pair from a comma-separated flag.
type kvPair struct {
	key   string
	value string
}

// parseKeyValueList parses a comma-separated list of key=value pairs,
// honoring double-quoted values that may contain commas. It is a small
// hand-written parser because the values may contain shell-quoted strings
// like seed="hello, world." that the standard "csv"/"strings.Split"
// helpers cannot tokenize.
//
// Grammar (informal):
//
//	list   := pair (',' pair)*
//	pair   := key '=' value
//	key    := [^=,]+
//	value  := bareword | quoted
//	bareword := [^,]*
//	quoted := '"' (escape | [^"\\])* '"'
//	escape := '\"' | '\\'
func parseKeyValueList(raw string) ([]kvPair, error) {
	var out []kvPair
	i := 0
	for i < len(raw) {
		// Skip leading whitespace.
		for i < len(raw) && (raw[i] == ' ' || raw[i] == '\t') {
			i++
		}
		if i >= len(raw) {
			break
		}
		// Read key up to '='.
		keyStart := i
		for i < len(raw) && raw[i] != '=' && raw[i] != ',' {
			i++
		}
		if i >= len(raw) || raw[i] != '=' {
			return nil, fmt.Errorf("expected '=' after key starting at position %d in %q", keyStart, raw)
		}
		key := strings.TrimSpace(raw[keyStart:i])
		if key == "" {
			return nil, fmt.Errorf("empty key in %q", raw)
		}
		i++ // consume '='
		// Read value: either a quoted string or a bareword.
		var value string
		if i < len(raw) && raw[i] == '"' {
			i++ // consume opening quote
			var sb strings.Builder
			for i < len(raw) {
				if raw[i] == '\\' && i+1 < len(raw) {
					next := raw[i+1]
					if next == '"' || next == '\\' {
						sb.WriteByte(next)
						i += 2
						continue
					}
				}
				if raw[i] == '"' {
					break
				}
				sb.WriteByte(raw[i])
				i++
			}
			if i >= len(raw) || raw[i] != '"' {
				return nil, fmt.Errorf("unterminated quoted value for key %q in %q", key, raw)
			}
			i++ // consume closing quote
			value = sb.String()
		} else {
			valStart := i
			for i < len(raw) && raw[i] != ',' {
				i++
			}
			value = strings.TrimSpace(raw[valStart:i])
		}
		out = append(out, kvPair{key: key, value: value})
		// Optional comma separator.
		if i < len(raw) && raw[i] == ',' {
			i++
		} else if i < len(raw) {
			return nil, fmt.Errorf("expected ',' or end of input after value for key %q in %q", key, raw)
		}
	}
	return out, nil
}

// nPeerConfig is the validated, normalized result of parsing all --peer
// flags plus the global --seed.
type nPeerConfig struct {
	Peers    []peerDecl
	OpenerID model.PeerID
}

const maxNPeers = 16

var _ = regexp.MustCompile // keep regexp imported even when unused below

// parseNPeerSpecs takes the raw --peer flag values, the global --seed,
// and the --opener value, and returns a fully validated nPeerConfig.
//
// Validation rules (fail-fast, all errors joined):
//   - 2..maxNPeers peers
//   - unique peer ids
//   - every peer either has its own seed= key or the global broadcastSeed
//     is non-empty
//   - opener references a declared peer id
func parseNPeerSpecs(rawPeers []string, broadcastSeed, opener string) (nPeerConfig, error) {
	var errs []error
	if len(rawPeers) < 2 {
		errs = append(errs, fmt.Errorf("--peer: need at least 2 peers, got %d", len(rawPeers)))
	}
	if len(rawPeers) > maxNPeers {
		errs = append(errs, fmt.Errorf("--peer: too many peers (%d), max is %d", len(rawPeers), maxNPeers))
	}
	cfg := nPeerConfig{}
	seen := map[model.PeerID]bool{}
	for _, raw := range rawPeers {
		pd, err := parsePeerSpec(raw)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if seen[pd.ID] {
			errs = append(errs, fmt.Errorf("--peer: duplicate id %q", pd.ID))
			continue
		}
		seen[pd.ID] = true
		// Apply broadcast seed if no per-peer seed.
		if pd.Seed == "" {
			pd.Seed = broadcastSeed
		}
		if pd.Seed == "" {
			errs = append(errs, fmt.Errorf("--peer id=%s: missing seed (provide seed= in --peer or set --seed for broadcast)", pd.ID))
		}
		cfg.Peers = append(cfg.Peers, pd)
	}
	// Opener defaults to first declared peer.
	if opener == "" || opener == "a" { // "a" is the legacy default; treat as "no explicit opener"
		if len(cfg.Peers) > 0 {
			cfg.OpenerID = cfg.Peers[0].ID
		}
	} else {
		cfg.OpenerID = model.PeerID(opener)
		if !seen[cfg.OpenerID] {
			errs = append(errs, fmt.Errorf("--opener: id %q is not declared in any --peer flag", opener))
		}
	}
	if len(errs) > 0 {
		return nPeerConfig{}, errors.Join(errs...)
	}
	return cfg, nil
}

// toV2Conversation maps a validated nPeerConfig into the v2 model fields
// of a model.Conversation. The caller is responsible for setting non-peer
// fields (id, max_turns, terminator, etc.).
func (c nPeerConfig) toV2Conversation() (peers []model.PeerParticipant, seeds map[model.PeerID]string) {
	peers = make([]model.PeerParticipant, len(c.Peers))
	seeds = make(map[model.PeerID]string, len(c.Peers))
	for i, p := range c.Peers {
		peers[i] = model.PeerParticipant{
			ID: p.ID,
			Spec: model.PeerSpec{
				URI:     "provider:" + p.Provider,
				Options: p.Options,
			},
		}
		seeds[p.ID] = p.Seed
	}
	return peers, seeds
}
