package claudecode

import (
	"regexp"
	"strings"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

func (a *Adapter) Observe(context adapter.CompletionContext) *adapter.TranscriptObservation {
	tail := strings.TrimSpace(context.NormalizedTail)

	if context.BytesSeen == 0 && context.ExitCode != nil {
		return &adapter.TranscriptObservation{
			Status:     model.RunPartial,
			Terminal:   true,
			Confidence: 0.7,
			Reason:     "process exited without output",
			Evidence:   []string{"exit_no_output"},
		}
	}

	if context.IdleMs >= 1500 {
		if matchAny(blockedPromptPatterns, tail) {
			return &adapter.TranscriptObservation{
				Status:     model.RunBlockedOnInput,
				Terminal:   true,
				Confidence: 0.92,
				Reason:     "interactive prompt detected",
				Evidence:   []string{"blocked_prompt"},
			}
		}
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
				Reason:     "rate limit or usage limit detected",
				Evidence:   []string{"rate_limit"},
			}
		}
		if matchAny(permissionPatterns, tail) {
			return &adapter.TranscriptObservation{
				Status:     model.RunPermissionPrompt,
				Terminal:   false,
				Confidence: 0.7,
				Reason:     "permission prompt detected",
				Evidence:   []string{"permission_prompt"},
			}
		}
	}

	if context.ExitCode != nil {
		if *context.ExitCode == 0 {
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

	if context.IdleMs >= 1500 && matchAny(promptReappearancePatterns, tail) {
		return &adapter.TranscriptObservation{
			Status:     model.RunCompleted,
			Terminal:   true,
			Confidence: 0.75,
			Reason:     "prompt reappeared",
			Evidence:   []string{"prompt_reappeared"},
		}
	}

	// No idle fallback for Claude Code. In PTY mode, Claude Code can pause
	// indefinitely for thinking or permission prompts. Rely on process exit
	// and heuristic pattern detection instead.
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
