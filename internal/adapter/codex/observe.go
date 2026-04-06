package codex

import (
	"regexp"
	"strings"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func (a *Adapter) Observe(ctx adapter.CompletionContext) *adapter.TranscriptObservation {
	tail := strings.TrimSpace(ctx.NormalizedTail)

	// Process exit is the PRIMARY completion signal for Codex.
	if ctx.ExitCode != nil {
		if *ctx.ExitCode == 0 {
			return &adapter.TranscriptObservation{
				Status:     model.RunCompleted,
				Terminal:   true,
				Confidence: 1.0,
				Reason:     "process exited successfully",
				Evidence:   []string{"exit_zero"},
			}
		}
		return &adapter.TranscriptObservation{
			Status:     model.RunCrashed,
			Terminal:   true,
			Confidence: 0.95,
			Reason:     "process exited with non-zero status",
			Evidence:   []string{"exit_non_zero"},
		}
	}

	// Heuristic checks when idle long enough.
	if ctx.IdleMs >= 1500 {
		if matchAny(authPatterns, tail) {
			return &adapter.TranscriptObservation{
				Status:     model.RunAuthRequired,
				Terminal:   true,
				Confidence: 0.9,
				Reason:     "authentication prompt detected",
				Evidence:   []string{"auth_prompt"},
			}
		}
		if matchAny(rateLimitPatterns, tail) {
			return &adapter.TranscriptObservation{
				Status:     model.RunRateLimited,
				Terminal:   true,
				Confidence: 0.88,
				Reason:     "rate limit detected",
				Evidence:   []string{"rate_limit"},
			}
		}
		if matchAny(blockedPromptPatterns, tail) {
			return &adapter.TranscriptObservation{
				Status:     model.RunBlockedOnInput,
				Terminal:   true,
				Confidence: 0.85,
				Reason:     "interactive prompt detected",
				Evidence:   []string{"blocked_prompt"},
			}
		}
	}

	// Low-confidence idle fallback: process still running but output has stopped.
	if ctx.BytesSeen > 0 && ctx.IdleMs >= 5000 {
		return &adapter.TranscriptObservation{
			Status:     model.RunCompleted,
			Terminal:   true,
			Confidence: 0.35,
			Reason:     "idle fallback",
			Evidence:   []string{"idle_fallback"},
		}
	}

	return nil
}

func matchAny(patterns []*regexp.Regexp, input string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}
