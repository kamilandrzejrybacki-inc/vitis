package model

// RunStatus describes the externally visible state of a Vitis run.
type RunStatus string

const (
	RunRunning          RunStatus = "running"
	RunCompleted        RunStatus = "completed"
	RunBlockedOnInput   RunStatus = "blocked_on_input"
	RunPermissionPrompt RunStatus = "permission_prompt"
	RunAuthRequired     RunStatus = "auth_required"
	RunRateLimited      RunStatus = "rate_limited"
	RunInterrupted      RunStatus = "interrupted"
	RunTimeout          RunStatus = "timeout"
	RunPartial          RunStatus = "partial"
	RunCrashed          RunStatus = "crashed"
	RunFailed           RunStatus = "error"
)

func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunCompleted,
		RunBlockedOnInput,
		RunPermissionPrompt,
		RunAuthRequired,
		RunRateLimited,
		RunInterrupted,
		RunTimeout,
		RunPartial,
		RunCrashed,
		RunFailed:
		return true
	default:
		return false
	}
}
