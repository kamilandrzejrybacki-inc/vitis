package codex

import "regexp"

var (
	// Codex-specific blocked prompt patterns
	blockedPromptPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(approve|deny|allow|decline)\b.*\?`),
		regexp.MustCompile(`(?i)\b(y/n|yes/no)\b`),
	}

	// Auth errors from Codex
	authPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(log in|login|sign in|authenticate)\b`),
		regexp.MustCompile(`(?i)\b(api key|refresh token|expired|invalid.*token)\b`),
		regexp.MustCompile(`(?i)codex login`),
	}

	// Rate limit patterns
	rateLimitPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(rate limit|too many requests)\b`),
		regexp.MustCompile(`(?i)\bERROR:.*\b(limit|quota|exceeded)\b`),
	}

	// Codex agent message prefix
	codexResponsePrefix = regexp.MustCompile(`(?m)^codex\s*$`)

	// Token usage line (signals end of output)
	tokenUsageLine = regexp.MustCompile(`(?m)^tokens used\s*$`)

	// Error prefix
	errorPrefix = regexp.MustCompile(`(?m)^ERROR:\s*(.+)$`)
)
