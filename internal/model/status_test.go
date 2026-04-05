package model

import "testing"

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		status   RunStatus
		terminal bool
	}{
		{RunCompleted, true},
		{RunBlockedOnInput, true},
		{RunPermissionPrompt, true},
		{RunAuthRequired, true},
		{RunRateLimited, true},
		{RunInterrupted, true},
		{RunTimeout, true},
		{RunPartial, true},
		{RunCrashed, true},
		{RunFailed, true},
		{RunRunning, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.terminal {
				t.Errorf("RunStatus(%q).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
			}
		})
	}
}
