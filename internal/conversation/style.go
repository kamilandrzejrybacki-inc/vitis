package conversation

// ReplyStyle controls how peer replies should be formatted. clank injects
// style instructions into the per-peer briefing on turn 1; the model
// follows them for the rest of the conversation. The default style
// (StyleNormal) makes no demands on length or register.
//
// The CavemanLite/Full/Ultra styles embed the rules from JuliusBrussee's
// caveman skill (https://github.com/JuliusBrussee/caveman) directly, so
// users get the ~75% reply-token compression without needing to install
// the upstream skill globally on their machine. Caveman targets natural-
// language replies; code blocks, error quotes, and commit messages stay
// verbatim per the canonical rules.
//
// Caveman style is particularly valuable in A2A conversations because:
//   - each reply IS the next envelope's user message → smaller envelope
//   - smaller envelope → smaller next-turn input context for the receiver
//   - receiver also caveman-styled → smaller reply → repeat
//
// The compression compounds across turns. Stacks cleanly with rtk's
// tool-call output compression: rtk shrinks input, caveman shrinks output.
type ReplyStyle string

const (
	StyleNormal       ReplyStyle = "normal"
	StyleCavemanLite  ReplyStyle = "caveman-lite"
	StyleCavemanFull  ReplyStyle = "caveman-full"
	StyleCavemanUltra ReplyStyle = "caveman-ultra"
)

// IsValidStyle reports whether the string is a recognised ReplyStyle.
func IsValidStyle(s string) bool {
	switch ReplyStyle(s) {
	case StyleNormal, StyleCavemanLite, StyleCavemanFull, StyleCavemanUltra:
		return true
	}
	return false
}

// styleInstructionsCavemanShared is the common ruleset injected for any
// caveman-mode style. Verbatim adapted from
// https://github.com/JuliusBrussee/caveman/blob/main/skills/caveman/SKILL.md
// (MIT licensed). Embedded here so clank ships caveman as a first-class
// option without an external dependency.
const styleInstructionsCavemanShared = `

REPLY STYLE — caveman mode (token-efficient, technical accuracy preserved):

Drop: filler words (just/really/basically/actually/simply), pleasantries
(sure/certainly/of course/happy to), hedging (might/could/perhaps/I think).
Keep: technical terms exact, code blocks unchanged, error messages quoted
verbatim, security warnings full-length, irreversible-action confirmations
full-length, multi-step sequences clear.

Pattern: "[thing] [action] [reason]. [next step]."

Bad:  "Sure! I'd be happy to help. The issue is most likely caused by..."
Good: "Bug in auth middleware. Token expiry uses < not <=. Fix:"

Code/commits/PRs: write normal prose. Resume caveman after.`

const styleInstructionsCavemanLite = styleInstructionsCavemanShared + `

Intensity: LITE. Drop fluff and hedging but keep articles (a/an/the) and
full sentences. Professional but tight. No "I'd recommend" — just say what
to do.
`

const styleInstructionsCavemanFull = styleInstructionsCavemanShared + `

Intensity: FULL. Drop articles, use sentence fragments, prefer short
synonyms (big not extensive, fix not "implement a solution for"). Telegraphic
style, but every technical claim still verifiable.
`

const styleInstructionsCavemanUltra = styleInstructionsCavemanShared + `

Intensity: ULTRA. Maximum compression. Abbreviate freely (DB, auth, config,
req, res, fn, impl). Strip conjunctions. Use arrows for causality (X → Y).
One word when one word does the job. Reserve full sentences only for
warnings and irreversible actions.
`

// RenderStyleInstructions returns the system-prompt block for a ReplyStyle
// or the empty string for StyleNormal / unknown styles.
func RenderStyleInstructions(style ReplyStyle) string {
	switch style {
	case StyleCavemanLite:
		return styleInstructionsCavemanLite
	case StyleCavemanFull:
		return styleInstructionsCavemanFull
	case StyleCavemanUltra:
		return styleInstructionsCavemanUltra
	default:
		return ""
	}
}
